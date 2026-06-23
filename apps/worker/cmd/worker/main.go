// gogg-worker hosts the Temporal Workflows + Activities that drive the
// asynchronous side of the platform. Crawl workflows, email, cache
// prewarm, and summoner enrichment belong here.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"go.temporal.io/sdk/client"
	sdklog "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/worker"

	crawlact "github.com/crafff/gogg/apps/worker/internal/activity/crawl"
	"github.com/crafff/gogg/apps/worker/internal/config"
	"github.com/crafff/gogg/apps/worker/internal/runtime"
	"github.com/crafff/gogg/apps/worker/internal/schedule"
	crawlwf "github.com/crafff/gogg/apps/worker/internal/workflow/crawl"
	"github.com/crafff/gogg/apps/worker/internal/workflow/smoke"
)

// Build metadata injected via -ldflags at compile time.
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
		fmt.Fprintf(os.Stderr, "gogg-worker: %v\n", err)
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
		"temporal_host", cfg.Temporal.HostPort,
		"temporal_namespace", cfg.Temporal.Namespace,
		"task_queues", cfg.Temporal.TaskQueues,
	)

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	rt, err := runtime.Build(rootCtx, cfg)
	if err != nil {
		return fmt.Errorf("build runtime: %w", err)
	}
	defer rt.Close()
	logger.Info("runtime_built", "regions", regionKeys(rt))

	crawlActs := crawlact.New(rt)

	c, err := client.Dial(client.Options{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
		Logger:    &temporalSlogLogger{l: logger.With("component", "temporal_sdk")},
	})
	if err != nil {
		return fmt.Errorf("temporal dial %s: %w", cfg.Temporal.HostPort, err)
	}
	defer c.Close()
	logger.Info("temporal_connected")

	// One worker per task queue. They share the gRPC client but their
	// poller goroutines + concurrency limits are independent. We
	// register the smoke workflow + crawl workflow on every queue —
	// idle registrations are cheap and let `--task-queue smoke` still
	// work as the chunk-1 plumbing test.
	workers := make([]worker.Worker, 0, len(cfg.Temporal.TaskQueues))
	for _, tq := range cfg.Temporal.TaskQueues {
		w := worker.New(c, tq, worker.Options{})
		w.RegisterWorkflow(smoke.PingWorkflow)
		w.RegisterActivity(smoke.PingActivity)
		w.RegisterWorkflow(crawlwf.CrawlRegionWorkflow)
		w.RegisterActivity(crawlActs)
		if err := w.Start(); err != nil {
			stopAll(workers, logger)
			return fmt.Errorf("worker start task_queue=%s: %w", tq, err)
		}
		workers = append(workers, w)
		logger.Info("worker_listening", "task_queue", tq)
	}

	// Upsert schedules from cfg.Schedule. Idempotent so repeated
	// worker restarts don't churn — and if an entry was removed from
	// YAML, the corresponding Temporal Schedule is intentionally left
	// in place; deletion is a manual `temporal schedule delete` step
	// to avoid accidentally orphaning history during a config typo.
	plans, err := schedule.BuildPlan(rt.Cfg)
	if err != nil {
		stopAll(workers, logger)
		return fmt.Errorf("build schedule plan: %w", err)
	}
	if len(plans) == 0 {
		logger.Info("no_schedules_in_config")
	} else if err := schedule.Upsert(rootCtx, c, plans); err != nil {
		stopAll(workers, logger)
		return fmt.Errorf("upsert schedules: %w", err)
	}

	<-rootCtx.Done()
	logger.Info("worker_shutdown_signal")
	stopAll(workers, logger)
	logger.Info("worker_stopped")
	return nil
}

// stopAll halts every worker in reverse start order so the most
// recently registered queue is drained first; not load-bearing but
// keeps logs readable.
func stopAll(ws []worker.Worker, logger *slog.Logger) {
	for i := len(ws) - 1; i >= 0; i-- {
		ws[i].Stop()
		logger.Info("worker_stopped_queue", "index", i)
	}
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

// temporalSlogLogger adapts slog to the Temporal SDK log.Logger
// interface. Temporal's keyvals contract matches slog's own (alternating
// key/value), so the adapter is straight-through.
type temporalSlogLogger struct {
	l *slog.Logger
}

// Compile-time assertion that the adapter still satisfies the SDK
// interface — protects against silent breakage on SDK upgrades.
var _ sdklog.Logger = (*temporalSlogLogger)(nil)

func (t *temporalSlogLogger) Debug(msg string, kv ...any) { t.l.Debug(msg, kv...) }
func (t *temporalSlogLogger) Info(msg string, kv ...any)  { t.l.Info(msg, kv...) }
func (t *temporalSlogLogger) Warn(msg string, kv ...any)  { t.l.Warn(msg, kv...) }
func (t *temporalSlogLogger) Error(msg string, kv ...any) { t.l.Error(msg, kv...) }

// regionKeys flattens runtime.Riot into a sorted (by Go map iteration
// — non-deterministic but only used for a single startup log line)
// slice of region keys, so the startup record shows what's wired.
func regionKeys(rt *runtime.Runtime) []string {
	out := make([]string, 0, len(rt.Riot))
	for k := range rt.Riot {
		out = append(out, k)
	}
	return out
}
