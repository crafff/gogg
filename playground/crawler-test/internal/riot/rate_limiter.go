package riot

import (
	"context"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	secondLimiter *rate.Limiter // 20次/秒
	minuteLimiter *rate.Limiter // 100次/2分钟
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		secondLimiter: rate.NewLimiter(rate.Limit(20), 1), // Limit 是每秒的请求数，Burst 是允许的最大突发请求数
		minuteLimiter: rate.NewLimiter(rate.Limit(98.0/120.0), 100),
	}
}

// Wait 必须同时拿到两个桶的令牌才能放行
func (r *RateLimiter) Wait(ctx context.Context) error {
	if err := r.minuteLimiter.Wait(ctx); err != nil {
		return err
	}
	return r.secondLimiter.Wait(ctx)
}
