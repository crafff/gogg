package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Logger attaches a request-scoped slog.Logger to the context (carrying
// request_id, method, path) and emits a single structured entry per
// request with status code and duration after the handler returns.
//
// Application code reaches the logger via LoggerFromContext(ctx); never
// log via slog.Default() inside a handler — that loses correlation.
func Logger(base *slog.Logger) func(http.Handler) http.Handler {
	if base == nil {
		base = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := RequestIDFromContext(r.Context())
			logger := base.With(
				slog.String("request_id", rid),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)
			ctx := context.WithValue(r.Context(), ctxKeyLogger, logger)

			ww := newResponseWriter(w)
			start := time.Now()
			next.ServeHTTP(ww, r.WithContext(ctx))

			logger.LogAttrs(r.Context(), slog.LevelInfo, "http_request",
				slog.Int("status", ww.status),
				slog.Int64("bytes", ww.bytes),
				slog.Duration("dur", time.Since(start)),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code +
// bytes written for the logger. http.Hijacker / http.Flusher aren't
// implemented yet — Phase B doesn't need WebSocket support; revisit
// when SSE for summoner search lands in Phase E.
type responseWriter struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += int64(n)
	return n, err
}
