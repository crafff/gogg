package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// HeaderRequestID is the canonical request-id propagation header.
// Same value the X-Request-Id convention has used since the Heroku
// log standards; clients and load balancers both recognise it.
const HeaderRequestID = "X-Request-Id"

// RequestID populates the context with a request ID, copying from
// the incoming X-Request-Id header when present so distributed
// callers can correlate logs, and generating a fresh hex token
// otherwise. Always echoed back in the response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(HeaderRequestID, id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// newRequestID returns a 16-byte hex string (32 chars). Not a UUID,
// but cheap, sortable enough for log correlation, and never zero-rate.
func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// rand.Read on Linux only fails if the kernel CSPRNG is
		// broken; falling back to a fixed marker so requests still
		// flow rather than panicking the whole server.
		return "rid-degraded"
	}
	return hex.EncodeToString(b[:])
}
