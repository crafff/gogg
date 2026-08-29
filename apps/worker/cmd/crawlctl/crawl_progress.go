package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/client"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
	"github.com/crafff/gogg/packages/tftcontract"
)

const progressQueryTimeout = 5 * time.Second

type crawlProgressSnapshot struct {
	Run      sqlcgen.TftCrawlRun
	Progress sqlcgen.GetTFTRunProgressRow
	Routes   []sqlcgen.ListTFTRunRouteProgressRow
}

func printCrawlProgress(ctx context.Context, temporal client.Client, description *client.ScheduleDescription, scheduleID, databaseDSN string, output io.Writer) error {
	pool, poolErr := newProgressPool(ctx, databaseDSN)
	if poolErr == nil {
		defer pool.Close()
	}

	for _, running := range description.Info.RunningWorkflows {
		var status tftcontract.CrawlStatus
		liveStage := "unknown"
		value, queryErr := temporal.QueryWorkflow(ctx, running.WorkflowID, "", tftcontract.CrawlStatusQueryName)
		if queryErr == nil {
			queryErr = value.Get(&status)
		}
		if queryErr != nil {
			fmt.Fprintf(output, "workflow=%s temporal_status=unavailable error=%q\n", running.WorkflowID, queryErr)
		} else {
			liveStage = status.Stage
			fmt.Fprintf(output, "workflow=%s run_id=%d state=%s stage=%s pause_pending=%t",
				running.WorkflowID, status.RunID, status.State, status.Stage, status.PausePending)
			if status.StageTotal > 0 {
				fmt.Fprintf(output, " stage_units=%d/%d", status.StageCompleted, status.StageTotal)
			}
			fmt.Fprintln(output)
		}

		if poolErr != nil {
			fmt.Fprintf(output, "progress=unavailable workflow=%s error=%q\n", running.WorkflowID, poolErr)
			continue
		}
		queryCtx, cancel := context.WithTimeout(ctx, progressQueryTimeout)
		var snapshot crawlProgressSnapshot
		var err error
		if status.RunID > 0 {
			snapshot, err = loadProgressByRunID(queryCtx, pool, status.RunID)
		} else {
			snapshot, err = loadProgressByWorkflowRunID(queryCtx, pool, running.FirstExecutionRunID)
		}
		cancel()
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				fmt.Fprintf(output, "progress=pending workflow=%s reason=%q\n", running.WorkflowID, "database run has not been created yet")
			} else {
				fmt.Fprintf(output, "progress=unavailable workflow=%s error=%q\n", running.WorkflowID, err)
			}
			continue
		}
		renderCrawlProgress(output, snapshot, liveStage, false)
	}

	if len(description.Info.RunningWorkflows) > 0 {
		return nil
	}
	if poolErr != nil {
		fmt.Fprintf(output, "latest_run=unavailable error=%q\n", poolErr)
		return nil
	}
	queryCtx, cancel := context.WithTimeout(ctx, progressQueryTimeout)
	var snapshot crawlProgressSnapshot
	var err error
	if recent := latestScheduledWorkflow(description); recent != nil {
		snapshot, err = loadProgressByWorkflowRunID(queryCtx, pool, recent.FirstExecutionRunID)
		if errors.Is(err, pgx.ErrNoRows) {
			cancel()
			fmt.Fprintf(output, "latest_action=%s workflow_run_id=%s persisted_run=none reason=%q\n",
				recent.WorkflowID, recent.FirstExecutionRunID, "workflow ended before database run creation")
			return nil
		}
	} else {
		snapshot, err = loadLatestProgress(queryCtx, pool, scheduleID)
	}
	cancel()
	if errors.Is(err, pgx.ErrNoRows) {
		fmt.Fprintln(output, "latest_run=none")
		return nil
	}
	if err != nil {
		fmt.Fprintf(output, "latest_run=unavailable error=%q\n", err)
		return nil
	}
	renderCrawlProgress(output, snapshot, snapshot.Run.Stage, true)
	return nil
}

func newProgressPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("progress database DSN is invalid")
	}
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open progress database: %w", err)
	}
	return pool, nil
}

func loadProgressByRunID(ctx context.Context, pool *pgxpool.Pool, runID int64) (crawlProgressSnapshot, error) {
	return loadProgress(ctx, pool, func(q *sqlcgen.Queries) (sqlcgen.TftCrawlRun, error) {
		return q.GetTFTRunByID(ctx, runID)
	})
}

func loadProgressByWorkflowRunID(ctx context.Context, pool *pgxpool.Pool, workflowRunID string) (crawlProgressSnapshot, error) {
	return loadProgress(ctx, pool, func(q *sqlcgen.Queries) (sqlcgen.TftCrawlRun, error) {
		return q.GetTFTRunByWorkflowRunID(ctx, workflowRunID)
	})
}

func loadLatestProgress(ctx context.Context, pool *pgxpool.Pool, scheduleID string) (crawlProgressSnapshot, error) {
	return loadProgress(ctx, pool, func(q *sqlcgen.Queries) (sqlcgen.TftCrawlRun, error) {
		return q.GetLatestTFTRunByScheduleID(ctx, scheduleID)
	})
}

func loadProgress(ctx context.Context, pool *pgxpool.Pool, lookup func(*sqlcgen.Queries) (sqlcgen.TftCrawlRun, error)) (snapshot crawlProgressSnapshot, resultErr error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return snapshot, err
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if rollbackErr := tx.Rollback(cleanupCtx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) && resultErr == nil {
			resultErr = rollbackErr
		}
	}()
	queries := sqlcgen.New(tx)
	snapshot.Run, err = lookup(queries)
	if err != nil {
		return snapshot, err
	}
	snapshot.Progress, err = queries.GetTFTRunProgress(ctx, snapshot.Run.ID)
	if err != nil {
		return snapshot, err
	}
	snapshot.Routes, err = queries.ListTFTRunRouteProgress(ctx, snapshot.Run.ID)
	if err != nil {
		return snapshot, err
	}
	if err := tx.Commit(ctx); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func renderCrawlProgress(output io.Writer, snapshot crawlProgressSnapshot, liveStage string, latest bool) {
	run, progress := snapshot.Run, snapshot.Progress
	matches := aggregateRouteProgress(snapshot.Routes)
	stage := liveStage
	if stage == "" {
		stage = run.Stage
	}
	patch := "unknown"
	if run.TargetPatch != nil && *run.TargetPatch != "" {
		patch = *run.TargetPatch
	}
	prefix := "persisted_run"
	if latest {
		prefix = "latest_run"
	}
	fmt.Fprintf(output, "%s=%d workflow=%s status=%s stage=%s patch=%s started=%s elapsed=%s",
		prefix, run.ID, run.WorkflowID, run.Status, stage, patch, formatTimestamp(run.StartedAt.Time, run.StartedAt.Valid), runElapsed(run))
	if run.LastError != nil && *run.LastError != "" {
		fmt.Fprintf(output, " last_error=%q", *run.LastError)
	}
	fmt.Fprintln(output)

	resolved := matches.CompletedMatches + matches.TerminalMatches
	remaining := matches.PendingMatches + matches.RetryMatches + matches.LeasedMatches + matches.NotEnqueuedMatches
	platformTotal := configuredPlatformCount(run.Config)
	platforms := fmt.Sprintf("%d/?", progress.CompletedPlatforms)
	if platformTotal > 0 {
		platforms = fmt.Sprintf("%d/%d", progress.CompletedPlatforms, platformTotal)
	}
	percent := "growing"
	stableTotal := stableMatchTotal(stage) || (platformTotal > 0 && progress.CompletedPlatforms >= platformTotal)
	if matches.DiscoveredMatches == 0 && stableTotal {
		percent = "100.0"
	} else if matches.DiscoveredMatches > 0 && stableTotal {
		percent = fmt.Sprintf("%.1f", float64(resolved)*100/float64(matches.DiscoveredMatches))
	}
	fmt.Fprintf(output, "progress_scope=run run_id=%d platforms=%s seeds_discovered=%d seeds_selected=%d matches=%d/%d percent=%s completed=%d terminal=%d remaining=%d pending=%d retry=%d leased=%d not_enqueued=%d eta=unavailable\n",
		run.ID, platforms, progress.DiscoveredSeeds, progress.SelectedSeeds, resolved, matches.DiscoveredMatches, percent,
		matches.CompletedMatches, matches.TerminalMatches, remaining, matches.PendingMatches, matches.RetryMatches,
		matches.LeasedMatches, matches.NotEnqueuedMatches)
	for _, route := range snapshot.Routes {
		routeResolved := route.CompletedMatches + route.TerminalMatches
		routeRemaining := route.PendingMatches + route.RetryMatches + route.LeasedMatches + route.NotEnqueuedMatches
		globalRemaining := route.GlobalPendingMatches + route.GlobalRetryMatches + route.GlobalLeasedMatches
		fmt.Fprintf(output, "route=%s run_matches=%d/%d completed=%d terminal=%d run_remaining=%d pending=%d retry=%d leased=%d not_enqueued=%d global_queue_remaining=%d\n",
			route.RoutingRegion, routeResolved, route.DiscoveredMatches, route.CompletedMatches, route.TerminalMatches,
			routeRemaining, route.PendingMatches, route.RetryMatches, route.LeasedMatches, route.NotEnqueuedMatches, globalRemaining)
	}
}

type routeProgressTotal struct {
	DiscoveredMatches, CompletedMatches, TerminalMatches int64
	PendingMatches, RetryMatches, LeasedMatches          int64
	NotEnqueuedMatches                                   int64
}

func aggregateRouteProgress(routes []sqlcgen.ListTFTRunRouteProgressRow) (total routeProgressTotal) {
	for _, route := range routes {
		total.DiscoveredMatches += route.DiscoveredMatches
		total.CompletedMatches += route.CompletedMatches
		total.TerminalMatches += route.TerminalMatches
		total.PendingMatches += route.PendingMatches
		total.RetryMatches += route.RetryMatches
		total.LeasedMatches += route.LeasedMatches
		total.NotEnqueuedMatches += route.NotEnqueuedMatches
	}
	return total
}

func latestScheduledWorkflow(description *client.ScheduleDescription) *client.ScheduleWorkflowExecution {
	for index := len(description.Info.RecentActions) - 1; index >= 0; index-- {
		if workflow := description.Info.RecentActions[index].StartWorkflowResult; workflow != nil {
			return workflow
		}
	}
	return nil
}

func configuredPlatformCount(raw []byte) int64 {
	var config struct {
		Platforms []string `json:"platforms"`
	}
	if json.Unmarshal(raw, &config) != nil {
		return 0
	}
	return int64(len(config.Platforms))
}

func stableMatchTotal(stage string) bool {
	switch strings.ToLower(stage) {
	case "match_detail", "analysis", "completed", "completed_with_errors", "cancelled", "failed":
		return true
	default:
		return false
	}
}

func formatTimestamp(value time.Time, valid bool) string {
	if !valid {
		return "unknown"
	}
	return value.UTC().Format(time.RFC3339)
}

func runElapsed(run sqlcgen.TftCrawlRun) string {
	if !run.StartedAt.Valid {
		return "unknown"
	}
	end := time.Now()
	if run.EndedAt.Valid {
		end = run.EndedAt.Time
	}
	duration := end.Sub(run.StartedAt.Time)
	if duration < 0 {
		duration = 0
	}
	return duration.Round(time.Second).String()
}
