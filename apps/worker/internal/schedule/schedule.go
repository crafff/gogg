// Package schedule turns the legacy YAML `schedule:` entries into
// Temporal Schedules. Each entry is upserted idempotently at worker
// start, replacing the robfig/cron loop the legacy `gogg crawl daemon`
// command ran.
//
// The schedule's workflow ID prefix is `gogg-crawl-{profile}`. Temporal
// appends the trigger timestamp to disambiguate runs, so back-to-back
// fires get distinct WorkflowIDs and event histories.
//
// Phase C chunk 4 ships this; chunks 5+ may layer overrides (pause
// per-schedule, region-specific TaskQueue overrides) but the contract
// here is the V1 baseline.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"

	crawlact "github.com/crafff/gogg/apps/worker/internal/activity/crawl"
	crawlwf "github.com/crafff/gogg/apps/worker/internal/workflow/crawl"
	legacycfg "github.com/crafff/gogg/internal/config"
)

// Plan is the resolved upsert plan derived from cfg.Schedule. Tested
// independently so the live Temporal client only sees pre-validated
// inputs. Each Plan entry is one Temporal Schedule.
type Plan struct {
	ID         string                   // e.g. "gogg-crawl-daily_kr"
	Cron       string                   // raw cron expression
	TaskQueue  string                   // e.g. "crawl-kr"
	WorkflowID string                   // prefix for workflow executions
	Input      crawlwf.CrawlRegionInput // workflow argument
}

// BuildPlan converts each cfg.Schedule entry into a Plan, resolving
// profile names to their region for task queue routing. Pure; takes
// no I/O, so the rest of the package can unit-test branching without
// reaching for a fake Temporal server.
func BuildPlan(cfg *legacycfg.Config) ([]Plan, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	plans := make([]Plan, 0, len(cfg.Schedule))
	for i, s := range cfg.Schedule {
		if strings.TrimSpace(s.Cron) == "" {
			return nil, fmt.Errorf("schedule[%d]: cron is empty", i)
		}
		if strings.TrimSpace(s.Profile) == "" {
			return nil, fmt.Errorf("schedule[%d]: profile is empty", i)
		}
		profile, err := cfg.Profile(s.Profile)
		if err != nil {
			return nil, fmt.Errorf("schedule[%d]: %w", i, err)
		}
		if err := profile.Validate(); err != nil {
			return nil, fmt.Errorf("schedule[%d]: profile %q invalid: %w", i, s.Profile, err)
		}
		queue := taskQueueForRegion(profile.Region)
		plans = append(plans, Plan{
			ID:         "gogg-crawl-" + s.Profile,
			Cron:       s.Cron,
			TaskQueue:  queue,
			WorkflowID: "gogg-crawl-" + s.Profile,
			Input: crawlwf.CrawlRegionInput{
				ProfileName: s.Profile,
				// Inline profile snapshot too — survives a config drift
				// between schedule upsert time and execution time. The
				// Workflow prefers ProfileName when set; the Profile is
				// a fallback only used if ProfileName lookup fails.
				Profile: crawlact.ProfileSnapshot{
					Region:            profile.Region,
					Mode:              string(profile.Mode),
					Version:           profile.Version,
					TargetTiers:       profile.TargetTiers,
					RankPrefetchTiers: profile.RankPrefetchTiers,
					Queue:             profile.Queue,
					Execution:         string(profile.Execution),
				},
			},
		})
	}
	return plans, nil
}

// taskQueueForRegion mirrors the worker config's default queue names.
// Lowercased to match docker-compose / k8s naming conventions.
func taskQueueForRegion(region string) string {
	return "crawl-" + strings.ToLower(region)
}

// Upsert applies the plan list against a live Temporal Schedule client.
// Idempotent: existing schedules are Update'd with the latest spec;
// new ones are Create'd. Logs are info-level so startup logs surface
// the schedule wiring clearly.
func Upsert(ctx context.Context, c client.Client, plans []Plan) error {
	sc := c.ScheduleClient()
	for _, p := range plans {
		action := &client.ScheduleWorkflowAction{
			ID:                       p.WorkflowID,
			Workflow:                 crawlwf.CrawlRegionWorkflow,
			Args:                     []any{p.Input},
			TaskQueue:                p.TaskQueue,
			WorkflowExecutionTimeout: 24 * time.Hour,
		}
		spec := client.ScheduleSpec{CronExpressions: []string{p.Cron}}
		opts := client.ScheduleOptions{
			ID:     p.ID,
			Spec:   spec,
			Action: action,
		}
		if err := upsertOne(ctx, sc, p, opts); err != nil {
			return fmt.Errorf("upsert schedule %q: %w", p.ID, err)
		}
	}
	return nil
}

func upsertOne(ctx context.Context, sc client.ScheduleClient, p Plan, opts client.ScheduleOptions) error {
	handle, err := sc.Create(ctx, opts)
	if err == nil {
		slog.InfoContext(ctx, "schedule_created",
			"id", p.ID, "task_queue", p.TaskQueue, "cron", p.Cron)
		_ = handle
		return nil
	}
	// SDK returns the typed sentinel temporal.ErrScheduleAlreadyRunning
	// for duplicate IDs. Fall through to Update so config changes
	// propagate without a manual delete step.
	if !errors.Is(err, temporal.ErrScheduleAlreadyRunning) {
		return err
	}
	h := sc.GetHandle(ctx, p.ID)
	if err := h.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(in client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			in.Description.Schedule.Spec = &opts.Spec
			in.Description.Schedule.Action = opts.Action
			return &client.ScheduleUpdate{Schedule: &in.Description.Schedule}, nil
		},
	}); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	slog.InfoContext(ctx, "schedule_updated",
		"id", p.ID, "task_queue", p.TaskQueue, "cron", p.Cron)
	return nil
}
