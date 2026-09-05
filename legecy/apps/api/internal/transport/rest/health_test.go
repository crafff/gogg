package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(_ context.Context) error { return f.err }

func TestLivenessHandler_returns200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	LivenessHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q", body["status"])
	}
}

func TestReadinessHandler_allHealthy(t *testing.T) {
	h := ReadinessHandler(
		NamedPinger{Name: "db", Pinger: fakePinger{}},
		NamedPinger{Name: "redis", Pinger: fakePinger{}},
	)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestReadinessHandler_oneDown(t *testing.T) {
	h := ReadinessHandler(
		NamedPinger{Name: "db", Pinger: fakePinger{}},
		NamedPinger{Name: "redis", Pinger: fakePinger{err: errors.New("dial refused")}},
	)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	comps, _ := body["components"].(map[string]any)
	if got, _ := comps["db"].(string); got != "ok" {
		t.Errorf("db status = %q", got)
	}
	if got, _ := comps["redis"].(string); got == "ok" {
		t.Errorf("redis should not be ok, got %q", got)
	}
}
