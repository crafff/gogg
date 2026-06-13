// gogg-api is the synchronous HTTP surface: GraphQL BFF + REST compat
// + auth + OAuth callbacks + health checks. The crawler runs as a
// separate `gogg-worker` binary (see apps/worker).
//
// This binary is the Phase B replacement for the legacy `./gogg serve`
// subcommand. The legacy binary keeps running side-by-side until the
// new API reaches feature parity (rankings + versions + regions),
// per ADR-0001.
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

	"github.com/crafff/gogg/apps/api/internal/config"
	"github.com/crafff/gogg/apps/api/internal/service/catalog"
	"github.com/crafff/gogg/apps/api/internal/service/rankings"
	"github.com/crafff/gogg/apps/api/internal/transport/middleware"
	"github.com/crafff/gogg/apps/api/internal/transport/rest"
	v1 "github.com/crafff/gogg/apps/api/internal/transport/rest/v1"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
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

	handler := buildRouter(cfg, logger, pool)
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

func buildRouter(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	// Order matters: Recover sits outermost so it catches panics from
	// every other middleware AND every handler. Then RequestID, so
	// the recover log carries a request_id. Then Logger to attach the
	// request-scoped slogger. Then CORS.
	r.Use(middleware.Recover)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.CORS(cfg.API.AllowedOrigins))

	r.Get("/healthz", rest.LivenessHandler())
	r.Get("/readyz", rest.ReadinessHandler(
		rest.NamedPinger{Name: "db", Pinger: rest.PoolPinger{Pool: pool}},
	))

	// /api/v1 is the legacy-shape REST compatibility layer; deleted
	// when Phase D's new web app cuts over per ADR-0003.
	queries := sqlcgen.New(pool)
	catalogSvc := catalog.New(queries)
	rankingsSvc := rankings.New(queries, versionResolverAdapter{queries: queries})
	r.Mount("/api/v1", v1.Routes(catalogSvc, rankingsSvc))

	// /graphql, /oauth/callback/* land in later Phase B steps.
	return r
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
// (which returns a row struct) to the simpler string contract the
// rankings service wants. Inlined here in main.go because it's pure
// glue — no business logic, nothing to test in isolation.
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
