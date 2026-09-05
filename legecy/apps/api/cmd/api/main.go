// gogg-api is the synchronous HTTP surface: GraphQL BFF + REST compat
// + auth + OAuth callbacks + health checks. The crawler runs as a
// separate `gogg-worker` binary (see apps/worker).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"

	"github.com/crafff/gogg/apps/api/internal/auth"
	"github.com/crafff/gogg/apps/api/internal/auth/provider"
	"github.com/crafff/gogg/apps/api/internal/cache"
	"github.com/crafff/gogg/apps/api/internal/config"
	"github.com/crafff/gogg/apps/api/internal/service/catalog"
	"github.com/crafff/gogg/apps/api/internal/service/champion"
	"github.com/crafff/gogg/apps/api/internal/service/championinsights"
	"github.com/crafff/gogg/apps/api/internal/service/rankings"
	summonersvc "github.com/crafff/gogg/apps/api/internal/service/summoner"
	tftsvc "github.com/crafff/gogg/apps/api/internal/service/tft"
	usersvc "github.com/crafff/gogg/apps/api/internal/service/user"
	gqlserver "github.com/crafff/gogg/apps/api/internal/transport/graphql"
	"github.com/crafff/gogg/apps/api/internal/transport/graphql/resolver"
	"github.com/crafff/gogg/apps/api/internal/transport/middleware"
	"github.com/crafff/gogg/apps/api/internal/transport/rest"
	restauth "github.com/crafff/gogg/apps/api/internal/transport/rest/auth"
	v1 "github.com/crafff/gogg/apps/api/internal/transport/rest/v1"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
	"github.com/crafff/gogg/packages/tftcontract"
)

// Build metadata injected via -ldflags at compile time; defaults make
// `go run` show "dev" rather than empty strings.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if err := run(); err != nil {
		// Logger may not be configured yet if the failure was during
		// config load; emit to stderr unconditionally so process
		// supervisors see the cause.
		fmt.Fprintf(os.Stderr, "gogg-api: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := newLogger(cfg.Logging)
	slog.SetDefault(logger)
	logger.Info("starting",
		"version", version, "commit", commit, "build_date", buildDate,
		"port", cfg.API.Port, "log_level", cfg.Logging.Level,
	)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dbCtx, cancel := context.WithTimeout(rootCtx, 10*time.Second)
	defer cancel()
	pool, err := connectDB(dbCtx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()
	logger.Info("db_connected", "max_open_conns", cfg.Database.MaxOpenConns)

	// Redis is optional: if the URL is unset, run without a cache.
	// Useful for local debugging when you want every request to go
	// straight to PG. Production always has it configured.
	var redisClient *cache.Redis
	if cfg.Redis.URL != "" {
		redisClient, err = cache.NewRedis(cfg.Redis.URL)
		if err != nil {
			return fmt.Errorf("init redis: %w", err)
		}
		defer func() { _ = redisClient.Close() }()
		pingCtx, pingCancel := context.WithTimeout(rootCtx, 3*time.Second)
		if err := redisClient.Ping(pingCtx); err != nil {
			pingCancel()
			return fmt.Errorf("ping redis: %w", err)
		}
		pingCancel()
		logger.Info("redis_connected")
	}
	temporalClient, err := client.Dial(client.Options{HostPort: cfg.Temporal.HostPort, Namespace: cfg.Temporal.Namespace})
	if err != nil {
		return fmt.Errorf("connect temporal: %w", err)
	}
	defer temporalClient.Close()
	logger.Info("temporal_connected", "host", cfg.Temporal.HostPort, "namespace", cfg.Temporal.Namespace)

	handler := buildRouter(cfg, logger, pool, redisClient, temporalWorkflowStarter{client: temporalClient})
	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.API.Port),
		Handler:           handler,
		ReadTimeout:       cfg.API.ReadTimeout,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      cfg.API.WriteTimeout,
		IdleTimeout:       cfg.API.IdleTimeout,
		BaseContext:       func(_ net.Listener) context.Context { return rootCtx },
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-rootCtx.Done():
		logger.Info("shutdown_signal_received")
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listen: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.API.ShutdownGrace)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful_shutdown_failed", "err", err)
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("stopped_cleanly")
	return nil
}

type workflowStarter interface {
	summonersvc.WorkflowStarter
	tftsvc.WorkflowStarter
}

func buildRouter(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, redisClient *cache.Redis, workflowStarter workflowStarter) http.Handler {
	r := chi.NewRouter()
	queries := sqlcgen.New(pool)
	providers := configuredProviders(cfg.OAuth, logger)
	userService := usersvc.New(pool, queries, cfg.Auth.RefreshTTL, providers...)
	sessionCookieName := restauth.SessionCookieName(cfg.Auth.CookieSecure)

	// Bearer JWT remains optional for non-browser clients. Browser login is a
	// separate opaque-session path and never exposes JWTs to the SPA.
	var issuer *auth.Issuer
	if cfg.Auth.JWTSecret != "" {
		var err error
		issuer, err = auth.NewIssuer(cfg.Auth.JWTSecret, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL, cfg.Auth.Issuer)
		if err != nil {
			// Misconfigured secret at startup is fatal — log loudly and
			// return a router that 503s everything so the failure is
			// visible to k8s readiness probes (which Page on consistent
			// 5xx).
			logger.Error("auth_issuer_init_failed", "err", err)
			return errorRouter(err)
		}
		logger.Info("auth_enabled", "issuer", cfg.Auth.Issuer, "access_ttl", cfg.Auth.AccessTTL, "refresh_ttl", cfg.Auth.RefreshTTL)
	} else {
		logger.Warn("auth_disabled_no_jwt_secret")
	}

	// Prometheus registry: process collectors + our HTTP histograms.
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	metrics := middleware.NewMetrics(reg)

	// Order matters: Recover sits outermost so it catches panics from
	// every other middleware AND every handler. Then RequestID, so
	// the recover log carries a request_id. Then Logger to attach the
	// request-scoped slogger. Metrics goes before CORS so OPTIONS
	// preflight counts as traffic (helpful for spotting CORS abuse).
	// Auth is last so it can use the request-scoped logger; it's
	// optional (doesn't 401), so endpoints that need claims check
	// middleware.ClaimsFromContext themselves.
	r.Use(middleware.Recover)
	r.Use(middleware.RequestID)
	r.Use(middleware.ClientIP)
	r.Use(middleware.Logger(logger))
	r.Use(metrics.Middleware)
	r.Use(middleware.CORS(cfg.API.AllowedOrigins))
	if issuer != nil {
		r.Use(middleware.Auth(issuer))
	}
	r.Use(middleware.SessionAuth(userService, sessionCookieName))
	r.Use(middleware.CookieCSRF(sessionCookieName, restauth.CSRFHeader))

	r.Get("/healthz", rest.LivenessHandler())

	pingers := []rest.NamedPinger{
		{Name: "db", Pinger: rest.PoolPinger{Pool: pool}},
	}
	if redisClient != nil {
		pingers = append(pingers, rest.NamedPinger{Name: "redis", Pinger: redisClient})
	}
	r.Get("/readyz", rest.ReadinessHandler(pingers...))

	r.Method(http.MethodGet, "/metrics", rest.MetricsHandler(reg))
	if cfg.Assets.Root != "" {
		r.Handle("/game-assets/*", rest.GameAssetsHandler(cfg.Assets.Root))
		logger.Info("game_assets_enabled", "root", cfg.Assets.Root)
	}

	// /api/v1 is the legacy-shape REST compatibility layer; deleted
	// when Phase D's new web app cuts over per ADR-0003.
	catalogSvc := catalog.New(queries)
	baseRankings := rankings.New(queries, versionResolverAdapter{queries: queries})
	championSvc := champion.New(queries, versionResolverAdapter{queries: queries})
	championInsightsSvc := championinsights.New(queries)
	summonerSvc := summonersvc.New(queries, redisClient, workflowStarter, summonersvc.Config{
		Freshness: cfg.Summoner.Freshness, IPLimit: cfg.Summoner.IPLimit, IPWindow: cfg.Summoner.IPLimitWindow,
		RegionTaskQueues: cfg.Temporal.RegionTaskQueues,
	})
	tftService := tftsvc.New(queries, redisClient, workflowStarter, tftsvc.RuntimeConfig{
		Freshness: cfg.TFT.PlayerFreshness, IPLimit: cfg.TFT.PlayerIPLimit, IPWindow: cfg.TFT.PlayerIPLimitWindow,
	})

	// Wrap rankings in the cache decorator when Redis is configured.
	// Service layer doesn't know whether it's cached; transport
	// layer doesn't know either. Only main.go has the wiring view.
	var rankingsSvc v1.RankingsService = baseRankings
	if redisClient != nil {
		const rankingsTTL = 5 * time.Minute
		rankingsSvc = rankings.NewCached(baseRankings, redisClient, rankingsTTL)
		logger.Info("rankings_cache_enabled", "ttl", rankingsTTL)
	}

	r.Mount("/api/v1", v1.Routes(catalogSvc, rankingsSvc))

	// GraphQL is the long-term query surface (per ADR-0003). /api/v1
	// stays mounted until the Phase D frontend cuts over. Resolvers
	// reuse the same service instances — caching applies to /graphql
	// requests for free.
	gqlRoot := &resolver.Resolver{Catalog: catalogSvc, Rankings: rankingsSvc, Champion: championSvc, ChampionInsights: championInsightsSvc, Summoners: summonerSvc, Users: userService, TFT: tftService}
	r.Handle("/graphql", gqlserver.NewHandler(gqlRoot))
	if cfg.API.GraphQLPlayground {
		r.Handle("/graphql/playground", gqlserver.NewPlaygroundHandler("/graphql"))
		logger.Info("graphql_playground_enabled", "path", "/graphql/playground")
	}

	// Browser OAuth is mounted independently of the optional JWT issuer. An
	// environment with no configured Google client exposes no login provider,
	// while logout remains idempotently available.
	r.Mount("/", restauth.Routes(userService, restauth.Config{
		CookieSecure: cfg.Auth.CookieSecure,
		Limiter:      redisClient,
	}))
	names := make([]string, 0, len(providers))
	for _, p := range providers {
		names = append(names, p.Name())
	}
	logger.Info("oauth_providers_registered", "providers", names)
	return r
}

type temporalWorkflowStarter struct{ client client.Client }

func (s temporalWorkflowStarter) StartSummonerWorkflow(ctx context.Context, workflowID, taskQueue string, input summonersvc.WorkflowInput) error {
	_, err := s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID: workflowID, TaskQueue: taskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
	}, "EnrichSummonerWorkflow", input)
	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return nil
	}
	return err
}

func (s temporalWorkflowStarter) StartTFTPlayerWorkflow(ctx context.Context, workflowID string, input tftsvc.WorkflowInput) error {
	_, err := s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID: workflowID, TaskQueue: tftcontract.SeedTaskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
	}, tftcontract.PlayerLookupWorkflowName, tftcontract.PlayerLookupInput{
		JobID: input.JobID, Platform: input.Platform, GameName: input.GameName, TagLine: input.TagLine,
	})
	var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(err, &alreadyStarted) {
		return nil
	}
	return err
}

// configuredProviders returns only the providers that have a non-empty
// client_id + client_secret + redirect_url. Skipping unconfigured
// providers means /oauth/start/{provider} cleanly 404s in environments
// where only one provider is wired up.
func configuredProviders(cfg config.OAuthConfig, logger *slog.Logger) []provider.Provider {
	var out []provider.Provider
	if isProviderConfigured(cfg.Google) {
		out = append(out, provider.NewGoogle(cfg.Google.ClientID, cfg.Google.ClientSecret, cfg.Google.RedirectURL))
	} else {
		logger.Debug("oauth_provider_skipped", "name", "google", "reason", "incomplete config")
	}
	logger.Debug("oauth_provider_skipped", "name", "discord", "reason", "not enabled in google-only login v1")
	return out
}

func isProviderConfigured(p config.OAuthProviderConfig) bool {
	return p.ClientID != "" && p.ClientSecret != "" && p.RedirectURL != ""
}

// errorRouter is the "init failed" placeholder that keeps the binary
// alive but unhealthy. /readyz returns 503 so k8s won't route traffic
// here; every other path returns 503 with the same message so the
// failure is observable from the outside.
func errorRouter(initErr error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "service unavailable: "+initErr.Error(), http.StatusServiceUnavailable)
	})
}

func connectDB(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	pgcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		pgcfg.MaxConns = int32(cfg.MaxOpenConns)
	}
	if cfg.MinIdleConns > 0 {
		pgcfg.MinConns = int32(cfg.MinIdleConns)
	}
	if cfg.ConnMaxLifetimeSeconds > 0 {
		pgcfg.MaxConnLifetime = cfg.ConnMaxLifetimeSeconds
	}
	pool, err := pgxpool.NewWithConfig(ctx, pgcfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// versionResolverAdapter bridges sqlcgen.Queries.GetLatestGameVersion
// (the newest statistics-ready version, returned as a row struct) to the
// simpler string contract the rankings and champion services want. Inlined
// here because it is pure wiring.
type versionResolverAdapter struct {
	queries *sqlcgen.Queries
}

func (a versionResolverAdapter) GetLatestVersion(ctx context.Context) (string, error) {
	row, err := a.queries.GetLatestGameVersion(ctx)
	if err != nil {
		return "", err
	}
	return row.Version, nil
}

func newLogger(cfg config.LoggingConfig) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if cfg.Format == "text" {
		h = slog.NewTextHandler(os.Stderr, opts)
	} else {
		h = slog.NewJSONHandler(os.Stderr, opts)
	}
	return slog.New(h)
}
