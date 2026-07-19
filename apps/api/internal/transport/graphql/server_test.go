package graphql

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/crafff/gogg/apps/api/internal/transport/graphql/resolver"
)

func TestHandlerRejectsOversizedBody(t *testing.T) {
	h := NewHandler(&resolver.Resolver{})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(make([]byte, maxRequestBodyBytes+1)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}
