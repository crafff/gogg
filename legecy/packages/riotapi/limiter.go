package riotapi

import (
	"context"

	"golang.org/x/time/rate"
)

// RateLimiter enforces both the per-second and per-2-minute Riot API limits.
type RateLimiter struct {
	perSecond *rate.Limiter // 20 req/s
	per2Min   *rate.Limiter // 100 req/120s
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		perSecond: rate.NewLimiter(rate.Limit(20), 1),
		per2Min:   rate.NewLimiter(rate.Limit(98.0/120.0), 1),
	}
}

// Wait blocks until both buckets grant a token, or ctx is cancelled.
func (r *RateLimiter) Wait(ctx context.Context) error {
	if err := r.per2Min.Wait(ctx); err != nil {
		return err
	}
	return r.perSecond.Wait(ctx)
}
