package rankings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/crafff/gogg/apps/api/internal/cache"
)

// CachedService wraps a Service with a Redis read-through cache.
// Implements the same surface the transport layer expects so wiring
// in main.go is a single-line swap.
//
// Cache key is a stable hash of the Filter so order/whitespace in
// query params doesn't fragment the cache. V1 uses a flat 5-minute
// TTL; the worker-driven "invalidate on crawl complete" pub/sub is
// a Phase E concern. Until then a stale page for at most TTL is fine
// — ranking data only moves on patch boundaries anyway.
type CachedService struct {
	inner *Service
	cache cache.Cache
	ttl   time.Duration
}

// NewCached wraps `inner` with `c`. ttl applies to every entry; pass
// 0 to disable expiry (debugging only).
func NewCached(inner *Service, c cache.Cache, ttl time.Duration) *CachedService {
	return &CachedService{inner: inner, cache: c, ttl: ttl}
}

// GetOverall serves from cache when possible; misses fall through to
// the inner service and populate the cache before returning.
func (c *CachedService) GetOverall(ctx context.Context, f Filter) (Result, error) {
	key := cacheKey("overall", f)
	return cache.GetOrLoadShim(ctx, c.cache, key, c.ttl, func(ctx context.Context) (Result, error) {
		return c.inner.GetOverall(ctx, f)
	})
}

// GetByPosition mirrors GetOverall.
func (c *CachedService) GetByPosition(ctx context.Context, f Filter) (Result, error) {
	key := cacheKey("by_position", f)
	return cache.GetOrLoadShim(ctx, c.cache, key, c.ttl, func(ctx context.Context) (Result, error) {
		return c.inner.GetByPosition(ctx, f)
	})
}

// cacheKey is a stable string derived from a Filter. JSON marshal of
// the Filter gives canonical field order (Go encoder is deterministic
// for structs), then SHA-256 of the JSON keeps key length bounded and
// safe for Redis (no spaces, no control chars).
func cacheKey(prefix string, f Filter) string {
	b, _ := json.Marshal(f) // Filter has only basic types; never errors.
	sum := sha256.Sum256(b)
	return fmt.Sprintf("gogg:api:rankings:%s:%s", prefix, hex.EncodeToString(sum[:]))
}
