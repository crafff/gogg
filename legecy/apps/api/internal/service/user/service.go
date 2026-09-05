// Package user owns browser login, application sessions, and the current-user
// view. OAuth provider calls happen outside database transactions; identity
// binding and session creation happen atomically inside the service.
package user

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"

	"github.com/crafff/gogg/apps/api/internal/auth"
	"github.com/crafff/gogg/apps/api/internal/auth/provider"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

const oauthAttemptTTL = 10 * time.Minute

const (
	maxProviderSubjectRunes = 512
	maxDisplayNameRunes     = 128
	maxEmailRunes           = 320
	maxAvatarURLRunes       = 2048
	maxUserAgentRunes       = 512
)

// TxBeginner is satisfied by pgxpool.Pool. Keeping it narrow lets the service
// own transaction policy without exposing pgx to transports or resolvers.
type TxBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Querier is the non-transactional sqlc surface used by session bootstrap and
// the short-lived OAuth attempt. Login account/session writes use Queries on a
// transaction so they cannot leave an orphan user behind.
type Querier interface {
	CreateOAuthLoginAttempt(context.Context, sqlcgen.CreateOAuthLoginAttemptParams) (sqlcgen.OauthLoginAttempt, error)
	ConsumeOAuthLoginAttempt(context.Context, []byte, string, []byte) (sqlcgen.OauthLoginAttempt, error)
	DeleteExpiredOAuthLoginAttempts(context.Context) (int64, error)
	DeleteExpiredUserSessions(context.Context) (int64, error)
	GetActiveUserSessionByHash(context.Context, []byte) (sqlcgen.UserSession, error)
	RevokeUserSessionByHash(context.Context, []byte) (int64, error)
	GetUserByID(context.Context, pgtype.UUID) (sqlcgen.User, error)
	ListUserOAuthIdentities(context.Context, pgtype.UUID) ([]sqlcgen.UserOauthIdentity, error)
}

type Service struct {
	db         TxBeginner
	q          Querier
	sessionTTL time.Duration
	providers  map[string]provider.Provider
}

func New(db TxBeginner, q Querier, sessionTTL time.Duration, providers ...provider.Provider) *Service {
	registered := make(map[string]provider.Provider, len(providers))
	for _, p := range providers {
		registered[p.Name()] = p
	}
	return &Service{db: db, q: q, sessionTTL: sessionTTL, providers: registered}
}

var (
	ErrUnknownProvider     = errors.New("user: unknown oauth provider")
	ErrInvalidOAuthAttempt = errors.New("user: oauth attempt invalid or expired")
	ErrInvalidSession      = errors.New("user: browser session invalid or expired")
)

type OAuthStart struct {
	URL string
}

type BrowserSession struct {
	Token     string
	ExpiresAt time.Time
	UserID    uuid.UUID
}

type OAuthIdentity struct {
	Provider  string
	Username  *string
	AvatarURL *string
}

type CurrentUser struct {
	ID          uuid.UUID
	DisplayName string
	Email       *string
	AvatarURL   *string
	Locale      string
	Identities  []OAuthIdentity
}

// Providers returns a deterministic list for the public login page. Only
// providers registered by main are exposed; Discord remains dormant in V1.
func (s *Service) Providers() []string {
	names := make([]string, 0, len(s.providers))
	for name := range s.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// BeginOAuth stores the one-shot state + PKCE verifier before returning the
// provider redirect. returnTo has already been restricted to a local path by
// the REST boundary and is validated again there after callback.
func (s *Service) BeginOAuth(ctx context.Context, providerName, returnTo, browserBinding string) (OAuthStart, error) {
	p, ok := s.providers[providerName]
	if !ok {
		return OAuthStart{}, ErrUnknownProvider
	}
	if _, err := s.q.DeleteExpiredOAuthLoginAttempts(ctx); err != nil {
		return OAuthStart{}, fmt.Errorf("clean expired oauth attempts: %w", err)
	}
	if _, err := s.q.DeleteExpiredUserSessions(ctx); err != nil {
		return OAuthStart{}, fmt.Errorf("clean expired browser sessions: %w", err)
	}
	state, err := auth.NewOpaqueToken()
	if err != nil {
		return OAuthStart{}, fmt.Errorf("generate oauth state: %w", err)
	}
	verifier := oauth2.GenerateVerifier()
	expiresAt := time.Now().UTC().Add(oauthAttemptTTL)
	if _, err := s.q.CreateOAuthLoginAttempt(ctx, sqlcgen.CreateOAuthLoginAttemptParams{
		StateHash:          auth.HashOpaqueToken(state),
		Provider:           providerName,
		CodeVerifier:       verifier,
		ReturnTo:           returnTo,
		BrowserBindingHash: auth.HashOpaqueToken(browserBinding),
		ExpiresAt:          timestamptz(expiresAt),
	}); err != nil {
		return OAuthStart{}, fmt.Errorf("store oauth attempt: %w", err)
	}
	return OAuthStart{URL: p.AuthCodeURL(state, verifier)}, nil
}

// CompleteOAuth atomically consumes the login attempt, exchanges the code,
// and then creates/updates the local identity and browser session in one short
// database transaction. No Google network call is made while a tx is open.
func (s *Service) CompleteOAuth(ctx context.Context, providerName, code, state, browserBinding, userAgent string, ip netip.Addr) (BrowserSession, string, error) {
	p, ok := s.providers[providerName]
	if !ok {
		return BrowserSession{}, "", ErrUnknownProvider
	}
	attempt, err := s.q.ConsumeOAuthLoginAttempt(
		ctx,
		auth.HashOpaqueToken(state),
		providerName,
		auth.HashOpaqueToken(browserBinding),
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BrowserSession{}, "", ErrInvalidOAuthAttempt
		}
		return BrowserSession{}, "", fmt.Errorf("consume oauth attempt: %w", err)
	}

	exchangeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	info, err := p.Exchange(exchangeCtx, code, attempt.CodeVerifier)
	if err != nil {
		return BrowserSession{}, "", fmt.Errorf("oauth exchange: %w", err)
	}
	if info.Subject == "" || len([]rune(info.Subject)) > maxProviderSubjectRunes {
		return BrowserSession{}, "", provider.ErrUserInfoIncomplete
	}
	info.Email = truncateUTF8(info.Email, maxEmailRunes)
	info.Username = truncateUTF8(info.Username, maxDisplayNameRunes)
	info.Avatar = truncateUTF8(info.Avatar, maxAvatarURLRunes)

	token, err := auth.NewOpaqueToken()
	if err != nil {
		return BrowserSession{}, "", fmt.Errorf("generate browser session: %w", err)
	}
	expiresAt := time.Now().UTC().Add(s.sessionTTL)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return BrowserSession{}, "", fmt.Errorf("begin login transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := sqlcgen.New(tx)

	if err := qtx.AcquireOAuthIdentityLock(ctx, providerName, info.Subject); err != nil {
		return BrowserSession{}, "", fmt.Errorf("lock oauth identity: %w", err)
	}
	account, err := qtx.GetUserByOAuthIdentity(ctx, providerName, info.Subject)
	switch {
	case err == nil:
		if _, err := qtx.UpsertOAuthIdentity(ctx, identityParams(account.ID, providerName, info)); err != nil {
			return BrowserSession{}, "", fmt.Errorf("refresh oauth identity: %w", err)
		}
	case errors.Is(err, pgx.ErrNoRows):
		userID, idErr := newUUIDv7()
		if idErr != nil {
			return BrowserSession{}, "", idErr
		}
		account, err = qtx.CreateUser(ctx, sqlcgen.CreateUserParams{
			ID: userID, DisplayName: displayName(providerName, info),
			Email: optStr(info.Email), Locale: "zh-CN",
		})
		if err != nil {
			return BrowserSession{}, "", fmt.Errorf("create user: %w", err)
		}
		if _, err := qtx.UpsertOAuthIdentity(ctx, identityParams(account.ID, providerName, info)); err != nil {
			return BrowserSession{}, "", fmt.Errorf("link oauth identity: %w", err)
		}
	default:
		return BrowserSession{}, "", fmt.Errorf("find oauth identity: %w", err)
	}

	if err := qtx.TouchUserLastLogin(ctx, account.ID); err != nil {
		return BrowserSession{}, "", fmt.Errorf("touch last login: %w", err)
	}
	sessionID, err := newUUIDv7()
	if err != nil {
		return BrowserSession{}, "", err
	}
	if _, err := qtx.CreateUserSession(ctx, sqlcgen.CreateUserSessionParams{
		ID: sessionID, UserID: account.ID, TokenHash: auth.HashOpaqueToken(token),
		ExpiresAt: timestamptz(expiresAt), UserAgent: optStr(truncateUTF8(userAgent, maxUserAgentRunes)), Ip: optAddr(ip),
	}); err != nil {
		return BrowserSession{}, "", fmt.Errorf("create user session: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return BrowserSession{}, "", fmt.Errorf("commit login transaction: %w", err)
	}
	uid, err := uuidFromPg(account.ID)
	if err != nil {
		return BrowserSession{}, "", err
	}
	return BrowserSession{Token: token, ExpiresAt: expiresAt, UserID: uid}, attempt.ReturnTo, nil
}

func (s *Service) AuthenticateSession(ctx context.Context, cleartext string) (uuid.UUID, error) {
	row, err := s.q.GetActiveUserSessionByHash(ctx, auth.HashOpaqueToken(cleartext))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrInvalidSession
		}
		return uuid.Nil, fmt.Errorf("lookup browser session: %w", err)
	}
	return uuidFromPg(row.UserID)
}

func (s *Service) LogoutBrowserSession(ctx context.Context, cleartext string) error {
	if cleartext == "" {
		return nil
	}
	if _, err := s.q.RevokeUserSessionByHash(ctx, auth.HashOpaqueToken(cleartext)); err != nil {
		return fmt.Errorf("revoke browser session: %w", err)
	}
	return nil
}

func (s *Service) CurrentUser(ctx context.Context, userID uuid.UUID) (*CurrentUser, error) {
	pgID := toPgUUID(userID)
	account, err := s.q.GetUserByID(ctx, pgID)
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}
	rows, err := s.q.ListUserOAuthIdentities(ctx, pgID)
	if err != nil {
		return nil, fmt.Errorf("list oauth identities: %w", err)
	}
	out := &CurrentUser{
		ID: userID, DisplayName: account.DisplayName, Email: account.Email,
		Locale: account.Locale, Identities: make([]OAuthIdentity, 0, len(rows)),
	}
	for _, row := range rows {
		identity := OAuthIdentity{Provider: row.Provider, Username: row.ProviderUsername, AvatarURL: row.AvatarUrl}
		out.Identities = append(out.Identities, identity)
		if out.AvatarURL == nil && row.AvatarUrl != nil {
			out.AvatarURL = row.AvatarUrl
		}
	}
	return out, nil
}

func identityParams(userID pgtype.UUID, providerName string, info provider.UserInfo) sqlcgen.UpsertOAuthIdentityParams {
	return sqlcgen.UpsertOAuthIdentityParams{
		ID: toPgUUID(uuid.New()), UserID: userID, Provider: providerName,
		ProviderUserID: info.Subject, ProviderEmail: optStr(info.Email),
		ProviderUsername: optStr(info.Username), AvatarUrl: optStr(info.Avatar),
	}
}

func displayName(providerName string, info provider.UserInfo) string {
	if info.Username != "" {
		return info.Username
	}
	if info.Email != "" {
		return info.Email
	}
	return providerName + " user"
}

func newUUIDv7() (pgtype.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("generate uuid v7: %w", err)
	}
	return toPgUUID(id), nil
}

func uuidFromPg(in pgtype.UUID) (uuid.UUID, error) {
	if !in.Valid {
		return uuid.Nil, errors.New("user: invalid uuid")
	}
	id, err := uuid.FromBytes(in.Bytes[:])
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse uuid: %w", err)
	}
	return id, nil
}

func toPgUUID(in uuid.UUID) pgtype.UUID {
	var out pgtype.UUID
	copy(out.Bytes[:], in[:])
	out.Valid = true
	return out
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func optStr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func optAddr(value netip.Addr) *netip.Addr {
	if !value.IsValid() {
		return nil
	}
	return &value
}

func truncateUTF8(value string, maxRunes int) string {
	value = strings.ToValidUTF8(value, "")
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}
