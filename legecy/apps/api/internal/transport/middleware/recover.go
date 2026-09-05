package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover catches panics from downstream handlers, logs the stack with
// the request-scoped logger, and returns a generic 500 to the client
// (never leak panic message to the wire). Must sit outermost (or just
// inside RequestID/Logger) so it can call LoggerFromContext.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			LoggerFromContext(r.Context()).LogAttrs(r.Context(), slog.LevelError,
				"panic_recovered",
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
			)
			// Don't write a body if the handler already started one.
			// http.ResponseWriter has no introspection for that, but
			// WriteHeader will be a no-op if it already fired.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal server error"}`))
		}()
		next.ServeHTTP(w, r)
	})
}
