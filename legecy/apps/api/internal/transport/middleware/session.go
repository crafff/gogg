package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	usersvc "github.com/crafff/gogg/apps/api/internal/service/user"
)

const (
	ctxKeySessionUser ctxKey = 101
	ctxKeySessionErr  ctxKey = 102
)

type SessionAuthenticator interface {
	AuthenticateSession(context.Context, string) (uuid.UUID, error)
}

// SessionAuth resolves an optional opaque browser session. Missing, expired,
// and revoked sessions remain anonymous so public queries keep working; an
// infrastructure failure is recorded for `me` to surface as unavailable.
func SessionAuth(svc SessionAuthenticator, cookieName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			userID, err := svc.AuthenticateSession(r.Context(), cookie.Value)
			if err != nil {
				if errors.Is(err, usersvc.ErrInvalidSession) {
					next.ServeHTTP(w, r)
					return
				}
				LoggerFromContext(r.Context()).Error("session_lookup_failed")
				ctx := context.WithValue(r.Context(), ctxKeySessionErr, err)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeySessionUser, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func SessionErrorFromContext(ctx context.Context) error {
	err, _ := ctx.Value(ctxKeySessionErr).(error)
	return err
}

// CookieCSRF requires a non-simple custom header whenever an unsafe request
// carries the browser session cookie. Cross-site forms cannot set this header;
// the exact-origin CORS allowlist blocks hostile JavaScript from adding it.
func CookieCSRF(cookieName, headerName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie(cookieName)
			if err == nil && cookie.Value != "" && r.Header.Get(headerName) != "1" {
				http.Error(w, "csrf check failed", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
