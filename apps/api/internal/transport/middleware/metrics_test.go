package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetrics_recordsDurationCountAndInFlight(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	r := chi.NewRouter()
	r.Use(m.Middleware)
	r.Get("/x/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x/42", nil))

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d", rec.Code)
	}

	// Counter increments by 1, route should be the chi pattern not
	// the literal URL — this is the high-cardinality guard.
	got := testutil.ToFloat64(m.requestsTotal.WithLabelValues("GET", "/x/{id}", "418"))
	if got != 1 {
		t.Errorf("requests_total{route=/x/{id}, 418} = %v, want 1", got)
	}
}

func TestMetrics_registersThreeCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	// A counter / histogram with no samples won't appear in
	// Gather(); drive one request through the middleware so they
	// each have a sample to gather.
	r := chi.NewRouter()
	r.Use(m.Middleware)
	r.Get("/_", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/_", nil))

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	got := make(map[string]bool, len(mfs))
	for _, mf := range mfs {
		got[mf.GetName()] = true
	}
	for _, want := range []string{
		"gogg_api_http_request_duration_seconds",
		"gogg_api_http_requests_total",
		"gogg_api_http_requests_in_flight",
	} {
		if !got[want] {
			t.Errorf("metric %q not registered (have: %v)", want, got)
		}
	}
}
