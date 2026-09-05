package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/crafff/gogg/apps/worker/internal/tft/staticdata"
	"github.com/crafff/gogg/packages/tftcontract"
)

const (
	snapshotScopedStaticDownloadChangeID = "tft-static-snapshot-scoped-download-v1"
	sourceFirstStaticSyncChangeID        = "tft-static-source-first-sync-v1"
)

func StaticSync(ctx workflow.Context) error {
	state := tftcontract.StaticStatus{State: "running", Stage: "syncing"}
	if err := workflow.SetQueryHandler(ctx, tftcontract.StaticStatusQueryName, func() (tftcontract.StaticStatus, error) { return state, nil }); err != nil {
		return err
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 5 * time.Second, MaximumInterval: 5 * time.Minute, MaximumAttempts: 5},
	})
	if workflow.GetVersion(ctx, sourceFirstStaticSyncChangeID, workflow.DefaultVersion, 1) == 1 {
		return syncStaticBySource(ctx, &state)
	}
	return syncStaticCombined(ctx, &state)
}

func syncStaticCombined(ctx workflow.Context, state *tftcontract.StaticStatus) error {
	var synced staticdata.SyncResult
	if err := workflow.ExecuteActivity(ctx, "Activities.SyncStatic").Get(ctx, &synced); err != nil {
		return err
	}
	state.SnapshotIDs = append([]int64(nil), synced.SnapshotIDs...)
	state.Total = int64(synced.AssetJobs)
	state.Remaining = state.Total
	if len(synced.SnapshotIDs) == 0 {
		state.State, state.Stage = "completed", "completed"
		return nil
	}
	version := workflow.GetVersion(ctx, snapshotScopedStaticDownloadChangeID, workflow.DefaultVersion, 1)
	if version == workflow.DefaultVersion {
		return downloadStaticLegacy(ctx, synced, state)
	}
	return downloadStaticScoped(ctx, synced, state)
}

func syncStaticBySource(ctx workflow.Context, state *tftcontract.StaticStatus) error {
	var target staticdata.SourceSyncInput
	for _, source := range []string{"cdragon", "ddragon"} {
		state.Stage, state.Source = "syncing", source
		var synced staticdata.SyncResult
		input := target
		input.Source = source
		if err := workflow.ExecuteActivity(ctx, "Activities.SyncStaticSource", input).Get(ctx, &synced); err != nil {
			return err
		}
		if target.Build == "" {
			target.Build, target.Patch = synced.Build, synced.Patch
		}
		state.SnapshotIDs = append(state.SnapshotIDs, synced.SnapshotIDs...)
		state.Total += int64(synced.AssetJobs)
		state.Remaining = max(0, state.Total-state.Completed-state.Skipped)
		if len(synced.SnapshotIDs) == 0 {
			continue
		}
		if err := downloadStaticSnapshotGroup(ctx, staticSnapshotGroup{Source: source, IDs: synced.SnapshotIDs}, state); err != nil {
			return err
		}
	}
	state.State, state.Stage, state.Source, state.Remaining = "completed", "completed", "", 0
	return nil
}

func downloadStaticLegacy(ctx workflow.Context, synced staticdata.SyncResult, state *tftcontract.StaticStatus) error {
	state.Stage, state.Source = "downloading", "legacy-global"
	state.Total = 0 // The legacy queue is global, so its denominator is not scoped to synced snapshots.
	for {
		var result staticdata.DownloadResult
		if err := workflow.ExecuteActivity(ctx, "Activities.DownloadStaticAssets", 20).Get(ctx, &result); err != nil {
			return err
		}
		state.Completed += int64(result.Processed)
		state.Remaining = result.Remaining
		state.Fetched += int64(result.Processed)
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
	state.Stage = "publishing"
	if err := workflow.ExecuteActivity(ctx, "Activities.PublishStaticSnapshots", synced.SnapshotIDs).Get(ctx, nil); err != nil {
		return err
	}
	state.State, state.Stage, state.Remaining = "completed", "completed", 0
	return nil
}

func downloadStaticScoped(ctx workflow.Context, synced staticdata.SyncResult, state *tftcontract.StaticStatus) error {
	groups := prioritizedSnapshotGroups(synced)
	for _, group := range groups {
		if err := downloadStaticSnapshotGroup(ctx, group, state); err != nil {
			return err
		}
	}
	state.State, state.Stage, state.Source, state.Remaining = "completed", "completed", "", 0
	return nil
}

func downloadStaticSnapshotGroup(ctx workflow.Context, group staticSnapshotGroup, state *tftcontract.StaticStatus) error {
	state.Stage, state.Source = "downloading", group.Source
	baseCompleted, baseSkipped := state.Completed, state.Skipped
	for {
		var result staticdata.DownloadResult
		if err := workflow.ExecuteActivity(ctx, "Activities.DownloadStaticAssetsForSnapshots", group.IDs).Get(ctx, &result); err != nil {
			return err
		}
		state.Completed = baseCompleted + result.Completed
		state.Skipped = baseSkipped + result.Skipped
		state.Failed = result.Failed
		state.Fetched += int64(result.Fetched)
		state.Remaining = max(0, state.Total-state.Completed-state.Skipped)
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
	state.Stage = "publishing"
	return workflow.ExecuteActivity(ctx, "Activities.PublishStaticSnapshots", group.IDs).Get(ctx, nil)
}

type staticSnapshotGroup struct {
	Source string
	IDs    []int64
}

func prioritizedSnapshotGroups(synced staticdata.SyncResult) []staticSnapshotGroup {
	bySource := map[string][]int64{}
	for _, snapshot := range synced.Snapshots {
		bySource[snapshot.Source] = append(bySource[snapshot.Source], snapshot.ID)
	}
	groups := make([]staticSnapshotGroup, 0, 2)
	for _, source := range []string{"cdragon", "ddragon"} {
		if ids := bySource[source]; len(ids) > 0 {
			groups = append(groups, staticSnapshotGroup{Source: source, IDs: ids})
		}
	}
	if len(groups) == 0 && len(synced.SnapshotIDs) > 0 {
		groups = append(groups, staticSnapshotGroup{Source: "all", IDs: append([]int64(nil), synced.SnapshotIDs...)})
	}
	return groups
}
