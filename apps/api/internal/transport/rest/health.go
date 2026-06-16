// Package rest hosts the non-GraphQL HTTP surface: health, metrics,
// auth callbacks, and the /api/v1 legacy compatibility layer.
package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/crafff/gogg/apps/api/internal/transport/middleware"
)

// Pinger is implemented by anything we want /readyz to verify is
// reachable. The pgxpool exposes Ping; Redis exposes a similar method
// we'll add in the cache milestone. Keeping the interface here avoids
// pulling a redis dep into the rest package before we need it.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NamedPinger pairs a dependency name with its pinger so /readyz can
// report exactly which component is unhealthy.
type NamedPinger struct {
	Name   string
	Pinger Pinger
}

// LivenessHandler always returns 200. It only confirms the process is
// running and the listener has not deadlocked — k8s livenessProbe.
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// ReadinessHandler pings each dependency in parallel with a tight
// timeout and reports per-component status. Returns 503 if any
// dependency fails. k8s readinessProbe.
func ReadinessHandler(deps ...NamedPinger) http.HandlerFunc {
	const probeTimeout = 2 * time.Second
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
		defer cancel()

		type result struct {
			name string
			err  error
		}
		ch := make(chan result, len(deps))
		for _, d := range deps {
			// Loop variable scoping fixed in Go 1.22; no `d := d` needed.
			go func() { ch <- result{d.Name, d.Pinger.Ping(ctx)} }()
		}

		statuses := make(map[string]string, len(deps))
		ok := true
		for range deps {
			r := <-ch
			if r.err != nil {
				ok = false
				statuses[r.name] = "down: " + r.err.Error()
				continue
			}
			statuses[r.name] = "ok"
		}

		code := http.StatusOK
		overall := "ready"
		if !ok {
			code = http.StatusServiceUnavailable
			overall = "not_ready"
			middleware.LoggerFromContext(r.Context()).Warn("readiness_failed", "components", statuses)
		}
		writeJSON(w, code, map[string]any{"status": overall, "components": statuses})
	}
}

// PoolPinger adapts *pgxpool.Pool to the Pinger interface. Lives here
// rather than in the storage package because it's a transport concern;
// nothing else needs this thin wrapper.
type PoolPinger struct{ Pool *pgxpool.Pool }

func (p PoolPinger) Ping(ctx context.Context) error { return p.Pool.Ping(ctx) }

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// MetricsHandler returns the promhttp scraper bound to the registry
// the middleware writes into. Lives here next to the other ops
// endpoints because it has the same "out of band, k8s-facing" role
// as /healthz and /readyz.
func MetricsHandler(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
		Registry:          reg,
	})
}
