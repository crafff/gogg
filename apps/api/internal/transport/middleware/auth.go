package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/crafff/gogg/apps/api/internal/auth"
)

// ctxKeyClaims stores parsed JWT claims on the request context.
// Reuses the same private ctxKey enum as RequestID/Logger so the
// runtime doesn't pay for an extra interface{} key allocation.
const ctxKeyClaims ctxKey = 100

// Auth is the optional bearer-token middleware. It looks at the
// Authorization header; when a valid Bearer token is present, the
// parsed claims are stored on the context. Missing or invalid headers
// pass through silently so the same chain serves both authenticated
// and public endpoints.
//
// Handlers that *require* auth check ClaimsFromContext(ctx) and return
// 401 themselves — this layer never short-circuits the request.
// Decoupling extraction from enforcement lets the GraphQL resolver
// surface, which is mostly public-read in V1, share the chain with
// future authenticated routes.
func Auth(issuer *auth.Issuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearer(r.Header.Get("Authorization"))
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			claims, err := issuer.Parse(token)
			if err != nil {
				// Malformed / expired tokens are logged at debug —
				// noisy on a public endpoint, and the handler decides
				// whether the absence of claims is fatal.
				LoggerFromContext(r.Context()).Debug("auth_token_invalid", "err", err)
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyClaims, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext returns the parsed access-token claims set by the
// Auth middleware, or (nil, false) if the request was anonymous /
// the token was rejected.
func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	v, ok := ctx.Value(ctxKeyClaims).(*auth.Claims)
	return v, ok
}

// UserIDFromContext is the common shortcut used by handlers that need
// "the current user id or 401". Returns the zero uuid + false when
// the request is anonymous.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	c, ok := ClaimsFromContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	return c.UserID, true
}

// bearer pulls the token out of "Bearer xxx" headers. Case-insensitive
// on the scheme to match the wide range of clients; rejects anything
// else (no Basic, no token without scheme prefix).
func bearer(header string) (string, bool) {
	const prefix = "bearer "
	if len(header) < len(prefix) {
		return "", false
	}
	if !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}
