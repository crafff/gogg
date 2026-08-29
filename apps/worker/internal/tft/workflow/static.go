package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/crafff/gogg/apps/worker/internal/tft/staticdata"
)

func StaticSync(ctx workflow.Context) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 5 * time.Second, MaximumInterval: 5 * time.Minute, MaximumAttempts: 5},
	})
	var synced staticdata.SyncResult
	if err := workflow.ExecuteActivity(ctx, "Activities.SyncStatic").Get(ctx, &synced); err != nil {
		return err
	}
	for {
		var result staticdata.DownloadResult
		if err := workflow.ExecuteActivity(ctx, "Activities.DownloadStaticAssets", 20).Get(ctx, &result); err != nil {
			return err
		}
		if !result.HasMore {
			break
		}
		if !result.NextEligibleAt.IsZero() {
			wait := result.NextEligibleAt.Sub(workflow.Now(ctx))
			if wait > 0 {
				if err := workflow.NewTimer(ctx, wait).Get(ctx, nil); err != nil {
					return err
				}
			}
		}
	}
	return workflow.ExecuteActivity(ctx, "Activities.PublishStaticSnapshots", synced.SnapshotIDs).Get(ctx, nil)
}
