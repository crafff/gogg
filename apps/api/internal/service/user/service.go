// Package user is the auth-adjacent business layer: turn an OAuth
// userinfo into a gogg user, mint/rotate refresh tokens, and revoke
// on logout. The REST handlers in apps/api/internal/transport/rest
// are thin shells over this service.
//
// Boundary rule (CLAUDE.md §Architectural rules):
//   - This service owns the orchestration: provider abstraction,
//     auth.Issuer, sqlc Querier.
//   - The REST layer just unpacks the HTTP cookie + JSON body,
//     calls one method here, and sets the cookie back. No SQL,
//     no JWT signing, no provider state.
package user

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/crafff/gogg/apps/api/internal/auth"
	"github.com/crafff/gogg/apps/api/internal/auth/provider"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

// Querier is the narrow sqlc surface this service needs. Defined here
// (consumer-side) for the same testability reason catalog / rankings
// do it — tests use a hand-rolled fake without spinning up Postgres.
type Querier interface {
	CreateUser(ctx context.Context, arg sqlcgen.CreateUserParams) (sqlcgen.User, error)
	GetUserByID(ctx context.Context, id pgtype.UUID) (sqlcgen.User, error)
	GetUserByOAuthIdentity(ctx context.Context, provider string, providerUserID string) (sqlcgen.User, error)
	UpsertOAuthIdentity(ctx context.Context, arg sqlcgen.UpsertOAuthIdentityParams) (sqlcgen.UserOauthIdentity, error)
	TouchUserLastLogin(ctx context.Context, id pgtype.UUID) error
	CreateRefreshToken(ctx context.Context, arg sqlcgen.CreateRefreshTokenParams) (sqlcgen.UserRefreshToken, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash []byte) (sqlcgen.UserRefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id pgtype.UUID) error
	RevokeAllRefreshTokensForUser(ctx context.Context, userID pgtype.UUID) error
}

// Service is the user use case. Construct with New().
type Service struct {
	q         Querier
	issuer    *auth.Issuer
	providers map[string]provider.Provider
}

// New returns a Service. Providers are indexed by their Name() so the
// REST handler routes /oauth/callback/{provider} → providers[provider].
// An unconfigured provider (empty client id / secret) should not be
// registered; the caller (main.go) decides which to wire in.
func New(q Querier, issuer *auth.Issuer, providers ...provider.Provider) *Service {
	m := make(map[string]provider.Provider, len(providers))
	for _, p := range providers {
		m[p.Name()] = p
	}
	return &Service{q: q, issuer: issuer, providers: m}
}

// Sentinel errors so the REST layer can map to HTTP codes without
// stringly-typed matching. Domain errors are intentionally limited —
// anything else is a 500 and gets logged.
var (
	ErrUnknownProvider = errors.New("user: unknown oauth provider")
	ErrInvalidRefresh  = errors.New("user: refresh token invalid or expired")
)

// Session is the bundle the REST layer hands back to the client: the
// short-lived JWT in the response body and the long-lived opaque
// refresh in the HttpOnly cookie.
type Session struct {
	AccessToken     string
	AccessExpiresAt time.Time
	RefreshToken    string
	RefreshExpires  time.Time
	UserID          uuid.UUID
}

// AuthCodeURL hands the caller the URL to redirect the browser to.
// state is the CSRF token the caller stores in a cookie before the
// redirect and verifies on callback.
func (s *Service) AuthCodeURL(providerName, state string) (string, error) {
	p, ok := s.providers[providerName]
	if !ok {
		return "", ErrUnknownProvider
	}
	return p.AuthCodeURL(state), nil
}

// LoginFromOAuth completes the OAuth callback: exchange code, find or
// create the user, link/refresh the identity row, and mint a new
// Session.
func (s *Service) LoginFromOAuth(ctx context.Context, providerName, code, userAgent string, ip netip.Addr) (Session, error) {
	p, ok := s.providers[providerName]
	if !ok {
		return Session{}, ErrUnknownProvider
	}
	info, err := p.Exchange(ctx, code)
	if err != nil {
		return Session{}, fmt.Errorf("oauth exchange: %w", err)
	}

	user, err := s.findOrCreateUser(ctx, providerName, info)
	if err != nil {
		return Session{}, err
	}

	if err := s.q.TouchUserLastLogin(ctx, user.ID); err != nil {
		return Session{}, fmt.Errorf("touch last_login: %w", err)
	}
	return s.issueSession(ctx, user.ID, userAgent, ip)
}

// RefreshSession rotates an existing refresh token: validates the
// cleartext, revokes the row, and issues a new pair. Strict rotation
// (i.e. one-shot refresh tokens) is the industry default — even if a
// refresh leaks, the next legitimate use revokes the leaked copy.
func (s *Service) RefreshSession(ctx context.Context, cleartext, userAgent string, ip netip.Addr) (Session, error) {
	row, err := s.q.GetRefreshTokenByHash(ctx, auth.HashRefreshToken(cleartext))
	if err != nil {
		return Session{}, ErrInvalidRefresh
	}
	if row.RevokedAt.Valid {
		return Session{}, ErrInvalidRefresh
	}
	if !row.ExpiresAt.Valid || row.ExpiresAt.Time.Before(time.Now()) {
		return Session{}, ErrInvalidRefresh
	}
	if err := s.q.RevokeRefreshToken(ctx, row.ID); err != nil {
		return Session{}, fmt.Errorf("revoke old refresh: %w", err)
	}
	uid, err := uuid.FromBytes(row.UserID.Bytes[:])
	if err != nil {
		return Session{}, fmt.Errorf("user_id parse: %w", err)
	}
	return s.issueSession(ctx, toPgUUID(uid), userAgent, ip)
}

// Logout revokes the supplied refresh token. An unknown token is not
// an error (idempotent endpoint, client is asked to forget).
func (s *Service) Logout(ctx context.Context, cleartext string) error {
	if cleartext == "" {
		return nil
	}
	row, err := s.q.GetRefreshTokenByHash(ctx, auth.HashRefreshToken(cleartext))
	if err != nil {
		return nil
	}
	if err := s.q.RevokeRefreshToken(ctx, row.ID); err != nil {
		return fmt.Errorf("revoke refresh: %w", err)
	}
	return nil
}

// findOrCreateUser: known identity → existing user; new identity →
// create user + link identity.
func (s *Service) findOrCreateUser(ctx context.Context, providerName string, info provider.UserInfo) (sqlcgen.User, error) {
	user, err := s.q.GetUserByOAuthIdentity(ctx, providerName, info.Subject)
	if err == nil {
		// Refresh cached identity labels (email, username, avatar
		// can change provider-side).
		_, _ = s.q.UpsertOAuthIdentity(ctx, sqlcgen.UpsertOAuthIdentityParams{
			ID:               toPgUUID(uuid.New()),
			UserID:           user.ID,
			Provider:         providerName,
			ProviderUserID:   info.Subject,
			ProviderEmail:    optStr(info.Email),
			ProviderUsername: optStr(info.Username),
			AvatarUrl:        optStr(info.Avatar),
		})
		return user, nil
	}

	// First time we see this identity → mint a user.
	displayName := info.Username
	if displayName == "" {
		displayName = info.Email
	}
	if displayName == "" {
		displayName = providerName + " user"
	}
	uid := uuid.New()
	created, err := s.q.CreateUser(ctx, sqlcgen.CreateUserParams{
		ID:          toPgUUID(uid),
		DisplayName: displayName,
		Email:       optStr(info.Email),
		Locale:      "zh-CN",
	})
	if err != nil {
		return sqlcgen.User{}, fmt.Errorf("create user: %w", err)
	}
	if _, err := s.q.UpsertOAuthIdentity(ctx, sqlcgen.UpsertOAuthIdentityParams{
		ID:               toPgUUID(uuid.New()),
		UserID:           created.ID,
		Provider:         providerName,
		ProviderUserID:   info.Subject,
		ProviderEmail:    optStr(info.Email),
		ProviderUsername: optStr(info.Username),
		AvatarUrl:        optStr(info.Avatar),
	}); err != nil {
		return sqlcgen.User{}, fmt.Errorf("link identity: %w", err)
	}
	return created, nil
}

// issueSession is the common tail of LoginFromOAuth and RefreshSession.
// Signs the access token, generates the opaque refresh, inserts the
// row, and returns the bundle.
func (s *Service) issueSession(ctx context.Context, userID pgtype.UUID, userAgent string, ip netip.Addr) (Session, error) {
	uid, err := uuid.FromBytes(userID.Bytes[:])
	if err != nil {
		return Session{}, fmt.Errorf("user_id parse: %w", err)
	}
	access, accExp, err := s.issuer.IssueAccess(uid)
	if err != nil {
		return Session{}, err
	}
	refresh, err := auth.NewOpaqueToken()
	if err != nil {
		return Session{}, err
	}
	refExp := time.Now().Add(s.issuer.RefreshTTL())
	if _, err := s.q.CreateRefreshToken(ctx, sqlcgen.CreateRefreshTokenParams{
		ID:        toPgUUID(uuid.New()),
		UserID:    userID,
		TokenHash: auth.HashRefreshToken(refresh),
		ExpiresAt: pgtype.Timestamptz{Time: refExp, Valid: true},
		UserAgent: optStr(userAgent),
		Ip:        optAddr(ip),
	}); err != nil {
		return Session{}, fmt.Errorf("store refresh: %w", err)
	}
	return Session{
		AccessToken:     access,
		AccessExpiresAt: accExp,
		RefreshToken:    refresh,
		RefreshExpires:  refExp,
		UserID:          uid,
	}, nil
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
	var p pgtype.UUID
	copy(p.Bytes[:], u[:])
	p.Valid = true
	return p
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optAddr(a netip.Addr) *netip.Addr {
	if !a.IsValid() {
		return nil
	}
	return &a
}
