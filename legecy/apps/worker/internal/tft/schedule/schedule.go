package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"

	"github.com/crafff/gogg/apps/worker/internal/tft/config"
	"github.com/crafff/gogg/packages/tftcontract"
)

type Plan struct {
	ID, Cron, TaskQueue, WorkflowID string
	Workflow                        any
	Args                            []any
}

func BuildPlans(cfg config.Config) ([]Plan, error) {
	if cfg.TFT.CrawlCron == "" {
		return nil, fmt.Errorf("tft.crawl_cron is empty")
	}
	if cfg.TFT.StaticCron == "" {
		return nil, fmt.Errorf("tft.static_cron is empty")
	}
	return []Plan{{
		ID: tftcontract.DefaultScheduleID, Cron: cfg.TFT.CrawlCron, TaskQueue: tftcontract.SeedTaskQueue,
		WorkflowID: "gogg-tft-crawl", Workflow: tftcontract.CrawlWorkflowName,
		Args: []any{tftcontract.CrawlInput{ProfileName: cfg.TFT.ProfileName, Platforms: cfg.TFT.Platforms, Window: cfg.TFT.Window, WindowLag: cfg.TFT.WindowLag}},
	}, {
		ID: tftcontract.DefaultStaticScheduleID, Cron: cfg.TFT.StaticCron, TaskQueue: tftcontract.StaticTaskQueue,
		WorkflowID: "gogg-tft-static", Workflow: tftcontract.StaticWorkflowName,
	}}, nil
}

func Upsert(ctx context.Context, c client.Client, plans []Plan) error {
	for _, plan := range plans {
		opts := client.ScheduleOptions{
			ID:      plan.ID,
			Spec:    client.ScheduleSpec{CronExpressions: []string{plan.Cron}},
			Action:  &client.ScheduleWorkflowAction{ID: plan.WorkflowID, Workflow: plan.Workflow, Args: plan.Args, TaskQueue: plan.TaskQueue},
			Overlap: enumspb.SCHEDULE_OVERLAP_POLICY_SKIP, CatchupWindow: time.Hour, PauseOnFailure: true,
			Paused: true, Note: "created paused; enable explicitly with crawlctl",
		}
		if err := upsertOne(ctx, c.ScheduleClient(), plan, opts); err != nil {
			return err
		}
	}
	return nil
}

func upsertOne(ctx context.Context, schedules client.ScheduleClient, plan Plan, opts client.ScheduleOptions) error {
	if _, err := schedules.Create(ctx, opts); err == nil {
		slog.InfoContext(ctx, "tft_schedule_created_paused", "id", plan.ID)
		return nil
	} else if !errors.Is(err, temporal.ErrScheduleAlreadyRunning) {
		return fmt.Errorf("create TFT schedule %s: %w", plan.ID, err)
	}
	handle := schedules.GetHandle(ctx, plan.ID)
	if err := handle.Update(ctx, client.ScheduleUpdateOptions{DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
		// Preserve State so a worker restart can never unpause an operator-
		// disabled schedule.
		input.Description.Schedule.Spec = &opts.Spec
		input.Description.Schedule.Action = opts.Action
		input.Description.Schedule.Policy = &client.SchedulePolicies{Overlap: opts.Overlap, CatchupWindow: opts.CatchupWindow, PauseOnFailure: opts.PauseOnFailure}
		return &client.ScheduleUpdate{Schedule: &input.Description.Schedule}, nil
	}}); err != nil {
		return fmt.Errorf("update TFT schedule %s: %w", plan.ID, err)
	}
	return nil
}
