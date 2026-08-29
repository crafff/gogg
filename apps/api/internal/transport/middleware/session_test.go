package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	usersvc "github.com/crafff/gogg/apps/api/internal/service/user"
)

type sessionAuthenticatorFunc func(context.Context, string) (uuid.UUID, error)

func (f sessionAuthenticatorFunc) AuthenticateSession(ctx context.Context, token string) (uuid.UUID, error) {
	return f(ctx, token)
}

func TestSessionAuthAttachesAuthenticatedUser(t *testing.T) {
	want := uuid.New()
	handler := SessionAuth(sessionAuthenticatorFunc(func(_ context.Context, token string) (uuid.UUID, error) {
		if token != "session-secret" {
			t.Fatalf("token = %q", token)
		}
		return want, nil
	}), "gogg_session")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := UserIDFromContext(r.Context())
		if !ok || got != want {
			t.Errorf("user id = %s, want %s", got, want)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	req.AddCookie(&http.Cookie{Name: "gogg_session", Value: "session-secret"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestSessionAuthTreatsInvalidSessionAsAnonymous(t *testing.T) {
	handler := SessionAuth(sessionAuthenticatorFunc(func(context.Context, string) (uuid.UUID, error) {
		return uuid.Nil, usersvc.ErrInvalidSession
	}), "gogg_session")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, ok := UserIDFromContext(r.Context()); ok || got != uuid.Nil {
			t.Errorf("user id = %s, want anonymous", got)
		}
		if err := SessionErrorFromContext(r.Context()); err != nil {
			t.Errorf("session error = %v, want nil", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	req.AddCookie(&http.Cookie{Name: "gogg_session", Value: "expired"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestSessionAuthPreservesInfrastructureFailure(t *testing.T) {
	wantErr := errors.New("database unavailable")
	handler := SessionAuth(sessionAuthenticatorFunc(func(context.Context, string) (uuid.UUID, error) {
		return uuid.Nil, wantErr
	}), "gogg_session")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !errors.Is(SessionErrorFromContext(r.Context()), wantErr) {
			t.Errorf("session error = %v, want %v", SessionErrorFromContext(r.Context()), wantErr)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	req.AddCookie(&http.Cookie{Name: "gogg_session", Value: "session-secret"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestCookieCSRFRequiresHeaderForUnsafeCookieRequest(t *testing.T) {
	protected := CookieCSRF("gogg_session", "X-GOGG-CSRF")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tc := range []struct {
		name       string
		method     string
		withCookie bool
		header     string
		want       int
	}{
		{name: "cross-site form blocked", method: http.MethodPost, withCookie: true, want: http.StatusForbidden},
		{name: "custom header accepted", method: http.MethodPost, withCookie: true, header: "1", want: http.StatusNoContent},
		{name: "anonymous mutation unchanged", method: http.MethodPost, want: http.StatusNoContent},
		{name: "safe request unchanged", method: http.MethodGet, withCookie: true, want: http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/graphql", nil)
			if tc.withCookie {
				req.AddCookie(&http.Cookie{Name: "gogg_session", Value: "secret"})
			}
			if tc.header != "" {
				req.Header.Set("X-GOGG-CSRF", tc.header)
			}
			rec := httptest.NewRecorder()
			protected.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
