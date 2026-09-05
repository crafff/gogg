// Package middleware holds the chi middleware stack for gogg-api.
//
// Each middleware lives in its own file and is independent so they can
// be reordered without churn. The canonical order, applied in main.go,
// is: Recover → RequestID → Logger → CORS → application routes.
package middleware

import (
	"context"
	"log/slog"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota + 1
	ctxKeyLogger
)

// RequestIDFromContext returns the request ID stored on the context by
// the RequestID middleware, or "" if there is none.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// LoggerFromContext returns the request-scoped slog.Logger placed on
// the context by the Logger middleware. Falls back to slog.Default()
// when called outside a request, so production code can always log
// without nil-guards.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if v, ok := ctx.Value(ctxKeyLogger).(*slog.Logger); ok {
		return v
	}
	return slog.Default()
}
