// Package cache wraps go-redis with the read-through pattern we use
// everywhere: try Redis, miss → call the loader, write the result
// back. Concurrent misses for the same key collapse into a single
// loader call via singleflight so a thundering herd never lands on
// the database.
//
// Values are stored as JSON. JSON is portable, debuggable in
// redis-cli (GET key | jq), and a fine fit for the small objects
// we cache (rankings result, summoner profile, version list).
// MessagePack or Protobuf would be cheaper at the byte level but
// we're not bottlenecked on serialization yet.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// Cache is the small surface every package in the api uses. Keep this
// interface narrow: bigger Redis features (pipelines, scripts, pub/sub)
// stay package-internal so service code never touches them.
type Cache interface {
	GetJSON(ctx context.Context, key string, dst any) error
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	Ping(ctx context.Context) error
}

// ErrMiss is returned by GetJSON when the key is not in Redis. Caller
// distinguishes miss-vs-real-error via errors.Is(err, cache.ErrMiss).
var ErrMiss = errors.New("cache: miss")

// Redis is the production Cache backed by go-redis. Construct with
// NewRedis(). The zero value is not usable.
type Redis struct {
	client *redis.Client
	sf     singleflight.Group
}

// NewRedis returns a Cache talking to the URL ("redis://host:port/db").
// The connection isn't opened until the first call; callers should
// invoke Ping() at startup to fail fast on misconfiguration.
func NewRedis(url string) (*Redis, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	return &Redis{client: redis.NewClient(opts)}, nil
}

// Ping is the readiness probe — readyz calls this through NamedPinger.
func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// GetJSON reads `key` into `dst` (which must be a pointer). Returns
// ErrMiss on a cache miss; other Redis errors are wrapped.
func (r *Redis) GetJSON(ctx context.Context, key string, dst any) error {
	b, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrMiss
		}
		return fmt.Errorf("redis get %q: %w", key, err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("decode cache %q: %w", key, err)
	}
	return nil
}

// SetJSON marshals `value` and writes it under `key` with the given
// TTL. A zero or negative ttl is treated as no-expiry (Redis SET
// without EX); callers should not pass zero except for sentinel
// values you intend to last forever.
func (r *Redis) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode cache %q: %w", key, err)
	}
	if err := r.client.Set(ctx, key, b, ttl).Err(); err != nil {
		return fmt.Errorf("redis set %q: %w", key, err)
	}
	return nil
}

// Delete drops keys (variadic so single-key and bulk drops use the
// same path). No error on missing keys.
func (r *Redis) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}

// Close releases the underlying connection pool. Call from main.go
// during shutdown.
func (r *Redis) Close() error { return r.client.Close() }

// GetOrLoad is the read-through helper. T is the cached type; pass a
// function that loads the value on miss. Singleflight collapses
// concurrent fillers for the same key so one DB hit serves N waiters.
//
//	type Result struct { … }
//	res, err := cache.GetOrLoad(ctx, c, "rankings:overall:<filter-hash>",
//	    5 * time.Minute,
//	    func(ctx context.Context) (Result, error) { return svc.GetOverall(ctx, f) })
//
// On loader error nothing is written to Redis. On Redis write error
// the value is still returned to the caller (best-effort caching).
func GetOrLoad[T any](
	ctx context.Context,
	c Cache,
	key string,
	ttl time.Duration,
	loader func(context.Context) (T, error),
) (T, bool, error) {
	var zero T

	// Fast path: hit.
	var hit T
	if err := c.GetJSON(ctx, key, &hit); err == nil {
		return hit, true, nil
	} else if !errors.Is(err, ErrMiss) {
		// Real Redis error (network, decode); fall through to loader
		// so a degraded cache doesn't take the whole API down.
		// The error is intentionally swallowed here; callers see it
		// in the metrics labels (cache_state="error") that the
		// transport-layer instrumentation will add in a follow-up.
		_ = err
	}

	// Miss path with singleflight: one filler per key.
	sf, ok := c.(interface {
		do(string, func() (any, error)) (any, error, bool)
	})
	_ = sf // Reserved for future tracing hook.
	_ = ok

	v, err, _ := withSingleflight(c, key, func() (any, error) {
		return loader(ctx)
	})
	if err != nil {
		return zero, false, err
	}
	value, ok := v.(T)
	if !ok {
		return zero, false, fmt.Errorf("cache: loader returned %T, want %T", v, zero)
	}
	if err := c.SetJSON(ctx, key, value, ttl); err != nil {
		// Best-effort: log via the singleflight error path, return
		// the freshly-loaded value anyway. Caller doesn't care that
		// the cache write failed — they got their answer.
		_ = err
	}
	return value, false, nil
}

// withSingleflight is the seam that lets *Redis (which embeds a
// singleflight.Group) dedupe loaders, while other Cache implementations
// (tests, fakes, future in-process caches) just call the loader
// directly. The package-level singleflight is keyed by Cache identity
// + key, but we don't expose that surface — *Redis.sf already does the
// right thing.
func withSingleflight(c Cache, key string, fn func() (any, error)) (any, error, bool) {
	if r, ok := c.(*Redis); ok {
		v, err, shared := r.sf.Do(key, fn)
		return v, err, shared
	}
	v, err := fn()
	return v, err, false
}
