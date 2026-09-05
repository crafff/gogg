// Package crawl wraps crawler phase logic into Temporal Activities.
// The algorithm is preserved from the previous in-process runner:
// same Riot API calls, same SQL inserts, same iteration order. The
// orchestration shell is Temporal Workflows, with retries, heartbeats,
// and cancellation handled by the SDK.
//
// Activities are registered as methods on Activities so the Runtime
// dependencies (DB pool, Riot clients) are shared across invocations
// without globals.
package crawl

import "github.com/crafff/gogg/apps/worker/internal/runtime"

// Activities groups the crawl Activity methods so worker.RegisterActivity
// can pick them up with a single call. Temporal registers each method
// using its Go method name, so renames are a wire-incompatible change.
type Activities struct {
	rt *runtime.Runtime
}

// New constructs an Activities bound to the shared runtime. The
// Runtime is shared but read-only after Build; the DB pool + Riot
// limiters are concurrency-safe.
func New(rt *runtime.Runtime) *Activities {
	return &Activities{rt: rt}
}
