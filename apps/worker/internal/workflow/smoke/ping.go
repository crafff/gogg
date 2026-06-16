// Package smoke holds the chunk-1 hello-world workflow used to verify
// the gogg-worker ↔ Temporal connection end-to-end before any business
// Activities land. It has no project dependencies (no DB, no Riot
// client), so a failure here is a pure plumbing problem: client dial,
// task queue mismatch, registration, or worker option misuse.
//
// Removed once chunks 2+ have proven the wiring with a real crawl
// Workflow.
package smoke

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// PingTaskQueue is the queue the smoke worker subscribes to; the
// starter must target the same value. Exported so the future starter
// (or a temporal CLI invocation) can reference it.
const PingTaskQueue = "smoke"

// PingInput is the Workflow's only argument. Kept as a struct rather
// than a bare string so future fields don't require a versioned
// signature change.
type PingInput struct {
	Message string `json:"message"`
}

// PingResult is the Workflow's output.
type PingResult struct {
	Reply      string    `json:"reply"`
	ExecutedAt time.Time `json:"executed_at"`
}

// PingActivity is the leaf executed on the worker. Temporal serialises
// the result, so anything in PingResult must be JSON-encodable; bare
// time.Time is fine because the SDK handles it.
func PingActivity(ctx context.Context, in PingInput) (PingResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("ping_activity_executing", "message", in.Message)
	return PingResult{
		Reply:      fmt.Sprintf("pong: %s", in.Message),
		ExecutedAt: time.Now().UTC(),
	}, nil
}

// PingWorkflow runs PingActivity once with a small retry budget. The
// short StartToCloseTimeout + low MaximumAttempts keeps a flapping
// Activity from masking real bugs during smoke verification.
func PingWorkflow(ctx workflow.Context, in PingInput) (PingResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("ping_workflow_starting", "message", in.Message)

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Second,
			MaximumAttempts:    3,
		},
	})

	var result PingResult
	if err := workflow.ExecuteActivity(ctx, PingActivity, in).Get(ctx, &result); err != nil {
		return PingResult{}, fmt.Errorf("execute ping activity: %w", err)
	}
	logger.Info("ping_workflow_completed", "reply", result.Reply)
	return result, nil
}
