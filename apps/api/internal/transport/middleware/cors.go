package middleware

import (
	"net/http"
	"strings"
)

// CORS rejects unknown origins instead of echoing back the legacy
// permissive "*". `allowed` is a slice of exact origins (scheme +
// host + port); an empty slice means CORS is disabled (same-origin
// only). Wildcards via "*" are not supported — that pattern was the
// security finding ADR-0003 calls out replacing.
func CORS(allowed []string) func(http.Handler) http.Handler {
	allow := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		allow[strings.TrimSpace(o)] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if _, ok := allow[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, "+HeaderRequestID)
					w.Header().Set("Access-Control-Max-Age", "600")
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
