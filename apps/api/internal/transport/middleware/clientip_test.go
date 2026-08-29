package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientIPPrefersIngressRealIP(t *testing.T) {
	var got string
	handler := ClientIP(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = ClientIPFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	req.RemoteAddr = "10.0.0.2:54321"
	req.Header.Set("X-Real-IP", "203.0.113.10")
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 203.0.113.10")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.Equal(t, "203.0.113.10", got)
}

func TestClientIPFallsBackToRemoteAddress(t *testing.T) {
	var got string
	handler := ClientIP(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = ClientIPFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	req.RemoteAddr = "192.0.2.4:4321"

	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.Equal(t, "192.0.2.4", got)
}
