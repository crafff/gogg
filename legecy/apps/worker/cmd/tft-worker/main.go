package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	temporalactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	tftactivity "github.com/crafff/gogg/apps/worker/internal/tft/activity"
	"github.com/crafff/gogg/apps/worker/internal/tft/config"
	"github.com/crafff/gogg/apps/worker/internal/tft/runtime"
	"github.com/crafff/gogg/apps/worker/internal/tft/schedule"
	tftworkflow "github.com/crafff/gogg/apps/worker/internal/tft/workflow"
	"github.com/crafff/gogg/packages/tftcontract"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gogg-tft-worker: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	logger.Info("tft_storage_ready", "raw_root", cfg.Raw.Root, "static_root", cfg.TFT.StaticRoot)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	rt, err := runtime.Build(ctx, cfg)
	if err != nil {
		return err
	}
	defer rt.Close()
	temporalClient, err := client.Dial(client.Options{HostPort: cfg.Temporal.HostPort, Namespace: cfg.Temporal.Namespace})
	if err != nil {
		return err
	}
	defer temporalClient.Close()
	activities := tftactivity.New(rt)
	type queueConfig struct {
		name        string
		concurrency int
	}
	queues := []queueConfig{{tftcontract.SeedTaskQueue, 8}, {tftcontract.MatchTaskQueue("AMERICAS"), 1}, {tftcontract.MatchTaskQueue("ASIA"), 1}, {tftcontract.MatchTaskQueue("EUROPE"), 1}, {tftcontract.MatchTaskQueue("SEA"), 1}, {tftcontract.StaticTaskQueue, 1}}
	workers := make([]worker.Worker, 0, len(queues))
	for _, queue := range queues {
		w := worker.New(temporalClient, queue.name, worker.Options{MaxConcurrentActivityExecutionSize: queue.concurrency})
		if queue.name == tftcontract.SeedTaskQueue {
			registerWorkflowCompat(w, tftworkflow.Crawl, tftcontract.CrawlWorkflowName)
			registerWorkflowCompat(w, tftworkflow.PlatformSeed, tftcontract.PlatformWorkflowName)
			registerWorkflowCompat(w, tftworkflow.PlatformCandidates, tftcontract.CandidateWorkflowName)
			registerWorkflowCompat(w, tftworkflow.RouteDispatch, tftcontract.RouteWorkflowName)
			registerWorkflowCompat(w, tftworkflow.PlayerLookup, tftcontract.PlayerLookupWorkflowName)
		} else if queue.name == tftcontract.StaticTaskQueue {
			registerWorkflowCompat(w, tftworkflow.StaticSync, tftcontract.StaticWorkflowName)
		}
		w.RegisterActivityWithOptions(activities, temporalactivity.RegisterOptions{Name: "Activities."})
		if err := w.Start(); err != nil {
			stopWorkers(workers)
			return fmt.Errorf("start TFT queue %s: %w", queue.name, err)
		}
		workers = append(workers, w)
		logger.Info("tft_worker_listening", "task_queue", queue.name)
	}
	plans, err := schedule.BuildPlans(cfg)
	if err != nil {
		stopWorkers(workers)
		return err
	}
	if err := schedule.Upsert(ctx, temporalClient, plans); err != nil {
		stopWorkers(workers)
		return err
	}
	<-ctx.Done()
	stopWorkers(workers)
	return nil
}

func registerWorkflowCompat(w worker.Worker, workflowFunc any, stableName string) {
	// Schedules created by the first TFT release persisted Go function names
	// such as "StaticSync". Keep those names registered while all new starts use
	// the stable contract aliases.
	w.RegisterWorkflow(workflowFunc)
	w.RegisterWorkflowWithOptions(workflowFunc, workflow.RegisterOptions{Name: stableName})
}
func stopWorkers(workers []worker.Worker) {
	for i := len(workers) - 1; i >= 0; i-- {
		workers[i].Stop()
	}
}
