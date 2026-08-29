//go:build integration

package riotapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisQuotaSharesLoLAndTFTApplicationBudget(t *testing.T) {
	client := integrationRedis(t)
	coordinator, err := NewRedisQuotaCoordinator(client)
	require.NoError(t, err)
	coordinator.defaultShortCap = 2
	coordinator.defaultLongCap = 2
	coordinator.pollInterval = 5 * time.Millisecond

	family := fmt.Sprintf("quota-test-%d", time.Now().UnixNano())
	base := QuotaScope{Credential: family, Route: family + ".example", Service: "match", Family: family}
	lol := base
	lol.Product, lol.Method = "lol", "lol-match-list"
	tft := base
	tft.Product, tft.Method = "tft", "tft-match-list"

	require.NoError(t, coordinator.Acquire(context.Background(), lol))
	require.NoError(t, coordinator.Acquire(context.Background(), tft))
	blocked, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, coordinator.Acquire(blocked, tft), context.DeadlineExceeded)
}

func TestRedisQuotaCooldownOnlyExtends(t *testing.T) {
	client := integrationRedis(t)
	coordinator, err := NewRedisQuotaCoordinator(client)
	require.NoError(t, err)
	family := fmt.Sprintf("cooldown-test-%d", time.Now().UnixNano())
	scope := QuotaScope{Credential: family, Product: "tft", Route: family + ".example", Method: "detail", Service: "match", Family: family}

	headers := http.Header{"Retry-After": []string{"2"}, "X-Rate-Limit-Type": []string{"application"}}
	require.NoError(t, coordinator.Observe(context.Background(), scope, http.StatusTooManyRequests, headers))
	headers.Set("Retry-After", "1")
	require.NoError(t, coordinator.Observe(context.Background(), scope, http.StatusTooManyRequests, headers))
	ttl, err := client.PTTL(context.Background(), quotaKey(scope, "app", scope.Credential, scope.Route)+":blocked").Result()
	require.NoError(t, err)
	require.Greater(t, ttl, 1500*time.Millisecond)
}

func integrationRedis(t *testing.T) *redis.Client {
	t.Helper()
	rawURL := os.Getenv("GOGG_REDIS_URL")
	if rawURL == "" {
		rawURL = "redis://localhost:6379/0"
	}
	options, err := redis.ParseURL(rawURL)
	require.NoError(t, err)
	client := redis.NewClient(options)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, client.Ping(ctx).Err())
	return client
}
