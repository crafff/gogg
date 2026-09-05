package user

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/crafff/gogg/apps/api/internal/auth"
	"github.com/crafff/gogg/apps/api/internal/auth/provider"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

type stubProvider struct {
	name     string
	state    string
	verifier string
}

func (p *stubProvider) Name() string { return p.name }

func (p *stubProvider) AuthCodeURL(state, verifier string) string {
	p.state = state
	p.verifier = verifier
	return "https://accounts.example/authorize"
}

func (*stubProvider) Exchange(context.Context, string, string) (provider.UserInfo, error) {
	return provider.UserInfo{}, nil
}

type stubQuerier struct {
	createdAttempt sqlcgen.CreateOAuthLoginAttemptParams
	session        sqlcgen.UserSession
	sessionErr     error
	user           sqlcgen.User
	identities     []sqlcgen.UserOauthIdentity
}

func (*stubQuerier) DeleteExpiredOAuthLoginAttempts(context.Context) (int64, error) {
	return 0, nil
}

func (*stubQuerier) DeleteExpiredUserSessions(context.Context) (int64, error) {
	return 0, nil
}

func (q *stubQuerier) CreateOAuthLoginAttempt(_ context.Context, arg sqlcgen.CreateOAuthLoginAttemptParams) (sqlcgen.OauthLoginAttempt, error) {
	q.createdAttempt = arg
	return sqlcgen.OauthLoginAttempt{}, nil
}

func (*stubQuerier) ConsumeOAuthLoginAttempt(context.Context, []byte, string, []byte) (sqlcgen.OauthLoginAttempt, error) {
	panic("not used")
}

func (q *stubQuerier) GetActiveUserSessionByHash(context.Context, []byte) (sqlcgen.UserSession, error) {
	return q.session, q.sessionErr
}

func (*stubQuerier) RevokeUserSessionByHash(context.Context, []byte) (int64, error) {
	return 1, nil
}

func (q *stubQuerier) GetUserByID(context.Context, pgtype.UUID) (sqlcgen.User, error) {
	return q.user, nil
}

func (q *stubQuerier) ListUserOAuthIdentities(context.Context, pgtype.UUID) ([]sqlcgen.UserOauthIdentity, error) {
	return q.identities, nil
}

func TestBeginOAuthStoresOnlyHashedSecretsAndPKCEVerifier(t *testing.T) {
	queries := &stubQuerier{}
	google := &stubProvider{name: "google"}
	svc := New(nil, queries, 30*24*time.Hour, google)

	start, err := svc.BeginOAuth(t.Context(), "google", "/me", "browser-binding-secret")
	if err != nil {
		t.Fatalf("BeginOAuth() error = %v", err)
	}
	if start.URL != "https://accounts.example/authorize" {
		t.Errorf("URL = %q", start.URL)
	}
	if google.state == "" || google.verifier == "" {
		t.Fatal("provider did not receive state and PKCE verifier")
	}
	if !bytes.Equal(queries.createdAttempt.StateHash, auth.HashOpaqueToken(google.state)) {
		t.Error("stored state hash does not match the generated state")
	}
	if bytes.Equal(queries.createdAttempt.StateHash, []byte(google.state)) {
		t.Error("OAuth state was stored in cleartext")
	}
	if !bytes.Equal(queries.createdAttempt.BrowserBindingHash, auth.HashOpaqueToken("browser-binding-secret")) {
		t.Error("stored browser binding hash does not match")
	}
	if queries.createdAttempt.CodeVerifier != google.verifier {
		t.Error("stored verifier does not match the provider challenge input")
	}
	if queries.createdAttempt.ReturnTo != "/me" {
		t.Errorf("returnTo = %q", queries.createdAttempt.ReturnTo)
	}
	if !queries.createdAttempt.ExpiresAt.Valid || time.Until(queries.createdAttempt.ExpiresAt.Time) <= 0 {
		t.Error("OAuth attempt does not have a future expiry")
	}
}

func TestAuthenticateSessionAndCurrentUser(t *testing.T) {
	userID := uuid.New()
	email, username, avatar := "player@example.com", "Player", "https://example/avatar.png"
	queries := &stubQuerier{
		session: sqlcgen.UserSession{UserID: toPgUUID(userID)},
		user: sqlcgen.User{
			ID: toPgUUID(userID), DisplayName: "Player", Email: &email, Locale: "zh-CN",
		},
		identities: []sqlcgen.UserOauthIdentity{{
			UserID: toPgUUID(userID), Provider: "google", ProviderUsername: &username, AvatarUrl: &avatar,
		}},
	}
	svc := New(nil, queries, time.Hour)

	gotID, err := svc.AuthenticateSession(t.Context(), "opaque-session")
	if err != nil || gotID != userID {
		t.Fatalf("AuthenticateSession() = (%s, %v), want (%s, nil)", gotID, err, userID)
	}
	account, err := svc.CurrentUser(t.Context(), gotID)
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}
	if account.DisplayName != "Player" || account.Email == nil || *account.Email != email || account.AvatarURL == nil || *account.AvatarURL != avatar {
		t.Fatalf("CurrentUser() = %#v", account)
	}
	if len(account.Identities) != 1 || account.Identities[0].Provider != "google" {
		t.Fatalf("identities = %#v", account.Identities)
	}
}

func TestAuthenticateSessionMapsMissingRow(t *testing.T) {
	svc := New(nil, &stubQuerier{sessionErr: pgx.ErrNoRows}, time.Hour)
	if _, err := svc.AuthenticateSession(t.Context(), "expired"); err != ErrInvalidSession {
		t.Fatalf("error = %v, want %v", err, ErrInvalidSession)
	}
}
