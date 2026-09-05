package workflow

import (
	"errors"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/crafff/gogg/apps/worker/internal/tft/activity"
	"github.com/crafff/gogg/apps/worker/internal/tft/config"
	"github.com/crafff/gogg/packages/tftcontract"
)

// PlayerLookup is an isolated, on-demand workflow. It never admits matches to
// an analysis cohort; it only writes raw archive records and canonical facts.
func PlayerLookup(ctx workflow.Context, in tftcontract.PlayerLookupInput) error {
	resolveCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: time.Second, BackoffCoefficient: 2, MaximumInterval: 30 * time.Second, MaximumAttempts: 5},
	})
	var player activity.ResolvePlayerResult
	if err := workflow.ExecuteActivity(resolveCtx, "Activities.ResolvePlayer", activity.ResolvePlayerInput{JobID: in.JobID, Platform: in.Platform, GameName: in.GameName, TagLine: in.TagLine}).Get(resolveCtx, &player); err != nil {
		if finishErr := finishPlayerLookup(ctx, in.JobID, "FAILED", applicationErrorType(err), err.Error()); finishErr != nil {
			return errors.Join(err, finishErr)
		}
		return err
	}
	fetchCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 5 * time.Second, BackoffCoefficient: 2, MaximumInterval: time.Minute, MaximumAttempts: 3},
	})
	fetchCtx = workflow.WithTaskQueue(fetchCtx, tftcontract.MatchTaskQueue(config.RoutingRegion(in.Platform)))
	var result activity.FetchPlayerHistoryResult
	if err := workflow.ExecuteActivity(fetchCtx, "Activities.FetchPlayerHistory", activity.FetchPlayerHistoryInput{JobID: in.JobID, Platform: in.Platform, PUUID: player.PUUID, Count: 20}).Get(fetchCtx, &result); err != nil {
		if finishErr := finishPlayerLookup(ctx, in.JobID, "FAILED", applicationErrorType(err), err.Error()); finishErr != nil {
			return errors.Join(err, finishErr)
		}
		return err
	}
	status := "COMPLETED"
	if result.Failed > 0 {
		status = "PARTIAL"
	}
	return finishPlayerLookup(ctx, in.JobID, status, "", "")
}

func finishPlayerLookup(ctx workflow.Context, jobID, status, code, message string) error {
	disconnected, _ := workflow.NewDisconnectedContext(ctx)
	disconnected = workflow.WithActivityOptions(disconnected, workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: time.Second, MaximumInterval: 10 * time.Second, MaximumAttempts: 5},
	})
	return workflow.ExecuteActivity(disconnected, "Activities.FinishPlayerLookup", activity.FinishPlayerLookupInput{JobID: jobID, Status: status, ErrorCode: code, ErrorMessage: message}).Get(disconnected, nil)
}

func applicationErrorType(err error) string {
	var app *temporal.ApplicationError
	if errors.As(err, &app) && app.Type() != "" {
		return app.Type()
	}
	return "REFRESH_FAILED"
}
