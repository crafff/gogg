package cache

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// GetOrLoadShim is the non-generic-erasure variant of GetOrLoad. It
// behaves identically but exists because Go's type inference can't
// always see through closures, especially when the loader closes over
// a struct method. CachedService in apps/api/internal/service/rankings
// uses this; new callers should prefer GetOrLoad directly when the
// inference works.
//
// Functionally GetOrLoad with hit/miss boolean is the right surface
// for HTTP-layer metrics, but service-layer callers don't care — they
// just want the value or the error. The shim returns only (T, error).
func GetOrLoadShim[T any](
	ctx context.Context,
	c Cache,
	key string,
	ttl time.Duration,
	loader func(context.Context) (T, error),
) (T, error) {
	var zero T

	// Try cache.
	var hit T
	if err := c.GetJSON(ctx, key, &hit); err == nil {
		return hit, nil
	} else if !errors.Is(err, ErrMiss) {
		// Non-miss error (network, decode): fall through to loader.
		// Degraded cache must never take down the API.
		_ = err
	}

	val, err := loader(ctx)
	if err != nil {
		return zero, err
	}
	// Best-effort write; if Redis hiccups, log at WARN but return
	// the freshly-loaded value anyway. The next request will retry
	// the SET on a fresh GET miss.
	if err := c.SetJSON(ctx, key, val, ttl); err != nil {
		slog.Default().Warn("cache_set_failed", "key", key, "err", err)
	}
	return val, nil
}
