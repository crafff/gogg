package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

// MetricsRegistry holds the gogg-api Prometheus collectors. Construct
// once at startup, register against a registry, and inject the
// Middleware into the chi stack.
type MetricsRegistry struct {
	requestDuration *prometheus.HistogramVec
	requestsTotal   *prometheus.CounterVec
	inFlight        prometheus.Gauge
}

// NewMetrics returns a registry pre-loaded with our HTTP collectors
// and registered against `reg`. Pass the same registry to promhttp.Handler.
func NewMetrics(reg prometheus.Registerer) *MetricsRegistry {
	m := &MetricsRegistry{
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "gogg",
			Subsystem: "api",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency by method, route, status.",
			// Buckets tuned for an aggregation API: most queries are
			// <50ms cached, <500ms uncached. The long tail catches
			// pathological queries before they're invisible.
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}, []string{"method", "route", "status"}),

		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "gogg",
			Subsystem: "api",
			Name:      "http_requests_total",
			Help:      "Total HTTP requests by method, route, status.",
		}, []string{"method", "route", "status"}),

		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "gogg",
			Subsystem: "api",
			Name:      "http_requests_in_flight",
			Help:      "In-flight HTTP requests right now.",
		}),
	}
	reg.MustRegister(m.requestDuration, m.requestsTotal, m.inFlight)
	return m
}

// Middleware records duration + count + in-flight per request. Use
// route templates from chi (e.g. "/api/v1/rankings/champions") instead
// of raw URL paths so summoners/12345 doesn't explode cardinality.
func (m *MetricsRegistry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.inFlight.Inc()
		defer m.inFlight.Dec()

		start := time.Now()
		mw := newResponseWriter(w)
		next.ServeHTTP(mw, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			// Unrouted requests (404 before any handler matched)
			// collapse to a single label so an attacker scanning
			// random URLs can't blow up our metric cardinality.
			route = "unmatched"
		}
		status := strconv.Itoa(mw.status)
		labels := prometheus.Labels{"method": r.Method, "route": route, "status": status}
		m.requestDuration.With(labels).Observe(time.Since(start).Seconds())
		m.requestsTotal.With(labels).Inc()
	})
}
