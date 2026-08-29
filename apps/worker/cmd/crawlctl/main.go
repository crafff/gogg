package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"

	"github.com/crafff/gogg/packages/tftcontract"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "crawlctl: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	host := flags.String("temporal", envOr("GOGG_TEMPORAL_HOST_PORT", "localhost:7233"), "Temporal host:port")
	namespace := flags.String("namespace", envOr("GOGG_TEMPORAL_NAMESPACE", "default"), "Temporal namespace")
	scheduleID := flags.String("schedule", tftcontract.DefaultScheduleID, "schedule ID")
	workflowID := flags.String("workflow-id", "", "active workflow ID")
	drain := flags.Bool("drain", false, "pause schedule and let active runs finish")
	pauseActive := flags.Bool("pause-active", false, "pause schedule and signal active runs")
	product := flags.String("product", "tft", "product weight: lol or tft")
	weight := flags.Int("weight", 1, "positive scheduler weight")
	redisURL := flags.String("redis", envOr("GOGG_REDIS_URL", "redis://localhost:6379/0"), "Redis URL")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if command == "set-weight" {
		return setWeight(*redisURL, *product, *weight)
	}
	c, err := client.Dial(client.Options{HostPort: *host, Namespace: *namespace})
	if err != nil {
		return err
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	handle := c.ScheduleClient().GetHandle(ctx, *scheduleID)
	switch command {
	case "status":
		return printStatus(ctx, c, handle, *scheduleID)
	case "enable":
		if err := ensurePollers(ctx, c, *scheduleID); err != nil {
			return err
		}
		if err := signalRunning(ctx, c, handle, "resume"); err != nil {
			return err
		}
		return handle.Unpause(ctx, client.ScheduleUnpauseOptions{Note: "enabled by crawlctl"})
	case "disable":
		if !*drain && !*pauseActive {
			return fmt.Errorf("disable requires --drain or --pause-active")
		}
		if err := handle.Pause(ctx, client.SchedulePauseOptions{Note: "disabled by crawlctl"}); err != nil {
			return err
		}
		if *pauseActive {
			return signalRunning(ctx, c, handle, "pause")
		}
		return nil
	case "trigger":
		if err := ensurePollers(ctx, c, *scheduleID); err != nil {
			return err
		}
		return handle.Trigger(ctx, client.ScheduleTriggerOptions{})
	case "resume-run":
		if *workflowID == "" {
			return fmt.Errorf("resume-run requires --workflow-id")
		}
		return c.SignalWorkflow(ctx, *workflowID, "", tftcontract.CrawlSignalName, tftcontract.ControlCommand{Action: "resume"})
	case "cancel-run":
		if *workflowID == "" {
			return fmt.Errorf("cancel-run requires --workflow-id")
		}
		return c.SignalWorkflow(ctx, *workflowID, "", tftcontract.CrawlSignalName, tftcontract.ControlCommand{Action: "cancel"})
	default:
		return usageError()
	}
}

func ensurePollers(ctx context.Context, c client.Client, scheduleID string) error {
	required := []struct {
		queue string
		kind  enumspb.TaskQueueType
	}{{tftcontract.SeedTaskQueue, enumspb.TASK_QUEUE_TYPE_WORKFLOW}}
	if scheduleID == tftcontract.DefaultStaticScheduleID {
		required = []struct {
			queue string
			kind  enumspb.TaskQueueType
		}{{tftcontract.StaticTaskQueue, enumspb.TASK_QUEUE_TYPE_WORKFLOW}}
	} else {
		for _, route := range []string{"AMERICAS", "ASIA", "EUROPE", "SEA"} {
			required = append(required, struct {
				queue string
				kind  enumspb.TaskQueueType
			}{tftcontract.MatchTaskQueue(route), enumspb.TASK_QUEUE_TYPE_ACTIVITY})
		}
	}
	for _, requirement := range required {
		description, err := c.DescribeTaskQueue(ctx, requirement.queue, requirement.kind)
		if err != nil {
			return fmt.Errorf("describe task queue %s: %w", requirement.queue, err)
		}
		if len(description.Pollers) == 0 {
			return fmt.Errorf("task queue %s has no %s poller; refusing to enable or trigger", requirement.queue, requirement.kind.String())
		}
	}
	return nil
}

func printStatus(ctx context.Context, c client.Client, handle client.ScheduleHandle, id string) error {
	description, err := handle.Describe(ctx)
	if err != nil {
		return err
	}
	paused, note := false, ""
	if description.Schedule.State != nil {
		paused, note = description.Schedule.State.Paused, description.Schedule.State.Note
	}
	fmt.Printf("schedule=%s paused=%t note=%q actions=%d running=%d\n", id, paused, note, description.Info.NumActions, len(description.Info.RunningWorkflows))
	for _, running := range description.Info.RunningWorkflows {
		if id == tftcontract.DefaultStaticScheduleID {
			var status tftcontract.StaticStatus
			value, queryErr := c.QueryWorkflow(ctx, running.WorkflowID, "", tftcontract.StaticStatusQueryName)
			if queryErr == nil {
				queryErr = value.Get(&status)
			}
			if queryErr != nil {
				fmt.Printf("workflow=%s status=unavailable error=%q\n", running.WorkflowID, queryErr)
				continue
			}
			if status.Source == "legacy-global" {
				fmt.Printf("workflow=%s state=%s stage=%s mode=legacy-global processed_global=%d remaining_global=%d fetched=%d\n",
					running.WorkflowID, status.State, status.Stage, status.Completed, status.Remaining, status.Fetched)
				continue
			}
			fmt.Printf("workflow=%s state=%s stage=%s source=%s assets=%d/%d completed=%d skipped=%d failed=%d remaining=%d fetched=%d\n",
				running.WorkflowID, status.State, status.Stage, status.Source,
				status.Completed+status.Skipped, status.Total, status.Completed, status.Skipped,
				status.Failed, status.Remaining, status.Fetched)
			continue
		}
		var status tftcontract.CrawlStatus
		value, err := c.QueryWorkflow(ctx, running.WorkflowID, "", tftcontract.CrawlStatusQueryName)
		if err == nil {
			err = value.Get(&status)
		}
		if err != nil {
			fmt.Printf("workflow=%s status=unavailable error=%q\n", running.WorkflowID, err)
			continue
		}
		fmt.Printf("workflow=%s run_id=%d state=%s stage=%s pause_pending=%t\n", running.WorkflowID, status.RunID, status.State, status.Stage, status.PausePending)
	}
	return nil
}

func signalRunning(ctx context.Context, c client.Client, handle client.ScheduleHandle, action string) error {
	description, err := handle.Describe(ctx)
	if err != nil {
		return err
	}
	for _, running := range description.Info.RunningWorkflows {
		if err := c.SignalWorkflow(ctx, running.WorkflowID, "", tftcontract.CrawlSignalName, tftcontract.ControlCommand{Action: action}); err != nil {
			return fmt.Errorf("signal %s: %w", running.WorkflowID, err)
		}
		if action == "pause" {
			if err := waitWorkflowState(ctx, c, running.WorkflowID, "paused"); err != nil {
				return err
			}
		}
	}
	return nil
}

func waitWorkflowState(ctx context.Context, c client.Client, workflowID, wanted string) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		value, err := c.QueryWorkflow(ctx, workflowID, "", tftcontract.CrawlStatusQueryName)
		if err == nil {
			var status tftcontract.CrawlStatus
			if value.Get(&status) == nil && status.State == wanted {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait workflow %s state %s: %w", workflowID, wanted, ctx.Err())
		case <-ticker.C:
		}
	}
}

func setWeight(rawURL, product string, weight int) error {
	product = strings.ToLower(product)
	if product != "lol" && product != "tft" {
		return fmt.Errorf("product must be lol or tft")
	}
	if weight < 1 {
		return fmt.Errorf("weight must be positive")
	}
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(opts)
	defer rdb.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return rdb.HSet(ctx, "gogg:riot-quota:v1:weights", product, strconv.Itoa(weight)).Err()
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func usageError() error {
	return fmt.Errorf("usage: crawlctl status|enable|disable|trigger|set-weight|resume-run|cancel-run [flags]")
}
