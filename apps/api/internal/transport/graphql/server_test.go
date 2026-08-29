package graphql

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/crafff/gogg/apps/api/internal/transport/graphql/domainerr"
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

func TestErrorPresenterPreservesSafeDomainExtensions(t *testing.T) {
	err := domainerr.WrapWithExtensions(
		"RATE_LIMITED",
		"retry later",
		errors.New("internal limiter detail"),
		map[string]any{"retryAfterSeconds": 42, "code": "must-not-override"},
	)

	got := sanitizingErrorPresenter(t.Context(), err)

	if got.Message != "retry later" {
		t.Fatalf("message = %q", got.Message)
	}
	if got.Extensions["code"] != "RATE_LIMITED" {
		t.Fatalf("code = %#v", got.Extensions["code"])
	}
	if got.Extensions["retryAfterSeconds"] != 42 {
		t.Fatalf("retryAfterSeconds = %#v", got.Extensions["retryAfterSeconds"])
	}
}
