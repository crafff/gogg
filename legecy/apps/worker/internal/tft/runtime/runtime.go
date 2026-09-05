package runtime

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/crafff/gogg/apps/worker/internal/tft/config"
	"github.com/crafff/gogg/apps/worker/internal/tft/ingest"
	"github.com/crafff/gogg/apps/worker/internal/tft/rawarchive"
	"github.com/crafff/gogg/packages/riotapi"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

type Runtime struct {
	Cfg     config.Config
	Pool    *pgxpool.Pool
	Queries *sqlcgen.Queries
	Redis   *redis.Client
	Riot    map[string]*riotapi.Client
	Ingest  *ingest.Store
	Gate    *AdaptiveGate
}

func Build(ctx context.Context, cfg config.Config) (*Runtime, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse TFT database DSN: %w", err)
	}
	if cfg.Database.MaxOpenConns > 0 {
		poolCfg.MaxConns = cfg.Database.MaxOpenConns
	}
	if cfg.Database.MaxIdleConns > 0 {
		poolCfg.MinConns = cfg.Database.MaxIdleConns
	}
	if cfg.Database.ConnMaxLifetime > 0 {
		poolCfg.MaxConnLifetime = time.Duration(cfg.Database.ConnMaxLifetime) * time.Second
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("open TFT database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping TFT database: %w", err)
	}

	redisOpts, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("parse TFT Redis URL: %w", err)
	}
	redisClient := redis.NewClient(redisOpts)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = redisClient.Close()
		pool.Close()
		return nil, fmt.Errorf("ping TFT Redis: %w", err)
	}
	quota, err := riotapi.NewRedisQuotaCoordinator(redisClient)
	if err != nil {
		_ = redisClient.Close()
		pool.Close()
		return nil, err
	}

	queries := sqlcgen.New(pool)
	archive := rawarchive.New(cfg.Raw.Root, cfg.Raw.CompressionLevel, queries)
	clients := make(map[string]*riotapi.Client, len(cfg.TFT.Platforms))
	for _, configuredPlatform := range cfg.TFT.Platforms {
		platform := strings.ToUpper(configuredPlatform)
		client := riotapi.NewClient(cfg.Riot.APIKey, config.PlatformURL(platform), config.RegionalURL(platform))
		client.SetQuotaCoordinator(quota)
		client.SetResponseRecorder(platform, archive)
		clients[platform] = client
	}
	return &Runtime{Cfg: cfg, Pool: pool, Queries: queries, Redis: redisClient, Riot: clients, Ingest: ingest.New(pool), Gate: NewAdaptiveGate()}, nil
}

func (r *Runtime) Close() {
	if r.Redis != nil {
		_ = r.Redis.Close()
	}
	if r.Pool != nil {
		r.Pool.Close()
	}
}

func (r *Runtime) Client(platform string) (*riotapi.Client, error) {
	client, ok := r.Riot[strings.ToUpper(platform)]
	if !ok {
		return nil, fmt.Errorf("TFT platform %q is not configured", platform)
	}
	return client, nil
}

// AdaptiveGate enforces platform=1, route=1 and global=8 initially. A route
// gains one slot after two clean 120-second windows; any 429 halves it. This
// is process-local concurrency control layered under the Redis request quota.
type AdaptiveGate struct {
	mu           sync.Mutex
	notify       chan struct{}
	globalActive int
	globalLimit  int
	keyActive    map[string]int
	keyLimit     map[string]int
	last429      map[string]time.Time
	lastIncrease map[string]time.Time
	startedAt    time.Time
}

func NewAdaptiveGate() *AdaptiveGate {
	return &AdaptiveGate{notify: make(chan struct{}), globalLimit: 8, keyActive: map[string]int{}, keyLimit: map[string]int{}, last429: map[string]time.Time{}, lastIncrease: map[string]time.Time{}, startedAt: time.Now()}
}

func (g *AdaptiveGate) Acquire(ctx context.Context, platform, route string) (func(), error) {
	platformKey := "platform:" + strings.ToUpper(platform)
	keys := []string{platformKey}
	if route != "" {
		keys = append(keys, "route:"+strings.ToUpper(route))
	}
	for {
		g.mu.Lock()
		for _, key := range keys {
			if _, ok := g.keyLimit[key]; !ok {
				g.keyLimit[key] = 1
			}
		}
		available := g.globalActive < g.globalLimit
		for _, key := range keys {
			available = available && g.keyActive[key] < g.keyLimit[key]
		}
		if available {
			g.globalActive++
			for _, key := range keys {
				g.keyActive[key]++
			}
			g.mu.Unlock()
			var once sync.Once
			return func() { once.Do(func() { g.release(keys...) }) }, nil
		}
		wait := g.notify
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-wait:
		}
	}
}

func (g *AdaptiveGate) release(keys ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.globalActive--
	for _, key := range keys {
		g.keyActive[key]--
	}
	close(g.notify)
	g.notify = make(chan struct{})
}

func (g *AdaptiveGate) ObserveSuccess(route string, now time.Time) {
	key := "route:" + strings.ToUpper(route)
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.keyLimit[key]; !ok {
		g.keyLimit[key] = 1
	}
	if _, ok := g.lastIncrease[key]; !ok {
		g.lastIncrease[key] = g.startedAt
	}
	if now.Sub(g.last429[key]) < 240*time.Second || now.Sub(g.lastIncrease[key]) < 240*time.Second {
		return
	}
	if g.keyLimit[key] < 4 {
		g.keyLimit[key]++
	}
	if g.globalLimit < 16 {
		g.globalLimit++
	}
	g.lastIncrease[key] = now
	close(g.notify)
	g.notify = make(chan struct{})
}

func (g *AdaptiveGate) ObserveRateLimit(route string, now time.Time) {
	key := "route:" + strings.ToUpper(route)
	g.mu.Lock()
	defer g.mu.Unlock()
	limit := g.keyLimit[key]
	if limit < 1 {
		limit = 1
	}
	g.keyLimit[key] = max(1, limit/2)
	g.globalLimit = max(8, g.globalLimit/2)
	g.last429[key] = now
}
