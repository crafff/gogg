package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type clientIPContextKey struct{}

// ClientIP records the request origin for public-action throttling. Production
// ingress overwrites X-Real-IP with its direct peer before proxying to the API;
// X-Forwarded-For is only the compatibility fallback.
func ClientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := strings.TrimSpace(r.Header.Get("X-Real-IP"))
		if ip == "" {
			ip = strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
		}
		if ip == "" {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err == nil {
				ip = host
			} else {
				ip = r.RemoteAddr
			}
		}
		ctx := context.WithValue(r.Context(), clientIPContextKey{}, ip)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ClientIPFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPContextKey{}).(string)
	if ip == "" {
		return "unknown"
	}
	return ip
}
