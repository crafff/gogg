package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	temporalactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	temporalworkflow "go.temporal.io/sdk/workflow"

	"github.com/crafff/gogg/apps/worker/internal/tft/staticdata"
)

func TestStaticSyncDownloadsAndPublishesCDragonBeforeDDragon(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(func(context.Context, staticdata.SourceSyncInput) (staticdata.SyncResult, error) {
		return staticdata.SyncResult{}, nil
	}, temporalactivity.RegisterOptions{Name: "Activities.SyncStaticSource"})
	env.RegisterActivityWithOptions(func(context.Context, []int64) (staticdata.DownloadResult, error) {
		return staticdata.DownloadResult{}, nil
	}, temporalactivity.RegisterOptions{Name: "Activities.DownloadStaticAssetsForSnapshots"})
	env.RegisterActivityWithOptions(func(context.Context, []int64) error { return nil }, temporalactivity.RegisterOptions{Name: "Activities.PublishStaticSnapshots"})

	var order []string
	env.OnGetVersion(sourceFirstStaticSyncChangeID, temporalworkflow.DefaultVersion, 1).Return(temporalworkflow.Version(1)).Once()
	env.OnActivity("Activities.SyncStaticSource", mock.Anything, staticdata.SourceSyncInput{Source: "cdragon"}).
		Run(func(mock.Arguments) { order = append(order, "sync-cdragon") }).
		Return(staticdata.SyncResult{Build: "16.17.1", Patch: "16.17", SnapshotIDs: []int64{12, 14}, AssetJobs: 2}, nil).Once()
	env.OnActivity("Activities.DownloadStaticAssetsForSnapshots", mock.Anything, []int64{12, 14}).
		Run(func(mock.Arguments) { order = append(order, "download-cdragon") }).
		Return(staticdata.DownloadResult{Total: 2, Completed: 2}, nil).Once()
	env.OnActivity("Activities.PublishStaticSnapshots", mock.Anything, []int64{12, 14}).
		Run(func(mock.Arguments) { order = append(order, "publish-cdragon") }).Return(nil).Once()
	env.OnActivity("Activities.SyncStaticSource", mock.Anything, staticdata.SourceSyncInput{Source: "ddragon", Build: "16.17.1", Patch: "16.17"}).
		Run(func(mock.Arguments) { order = append(order, "sync-ddragon") }).
		Return(staticdata.SyncResult{SnapshotIDs: []int64{11, 13}, AssetJobs: 4}, nil).Once()
	env.OnActivity("Activities.DownloadStaticAssetsForSnapshots", mock.Anything, []int64{11, 13}).
		Run(func(mock.Arguments) { order = append(order, "download-ddragon") }).
		Return(staticdata.DownloadResult{Total: 4, Completed: 4}, nil).Once()
	env.OnActivity("Activities.PublishStaticSnapshots", mock.Anything, []int64{11, 13}).
		Run(func(mock.Arguments) { order = append(order, "publish-ddragon") }).Return(nil).Once()

	env.ExecuteWorkflow(StaticSync)

	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, []string{"sync-cdragon", "download-cdragon", "publish-cdragon", "sync-ddragon", "download-ddragon", "publish-ddragon"}, order)
	env.AssertExpectations(t)
}

func TestStaticSyncKeepsCDragonPublishedWhenDDragonSyncFails(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(func(context.Context, staticdata.SourceSyncInput) (staticdata.SyncResult, error) {
		return staticdata.SyncResult{}, nil
	}, temporalactivity.RegisterOptions{Name: "Activities.SyncStaticSource"})
	env.RegisterActivityWithOptions(func(context.Context, []int64) (staticdata.DownloadResult, error) {
		return staticdata.DownloadResult{}, nil
	}, temporalactivity.RegisterOptions{Name: "Activities.DownloadStaticAssetsForSnapshots"})
	env.RegisterActivityWithOptions(func(context.Context, []int64) error { return nil }, temporalactivity.RegisterOptions{Name: "Activities.PublishStaticSnapshots"})

	var order []string
	env.OnGetVersion(sourceFirstStaticSyncChangeID, temporalworkflow.DefaultVersion, 1).Return(temporalworkflow.Version(1)).Once()
	env.OnActivity("Activities.SyncStaticSource", mock.Anything, staticdata.SourceSyncInput{Source: "cdragon"}).
		Run(func(mock.Arguments) { order = append(order, "sync-cdragon") }).
		Return(staticdata.SyncResult{Build: "16.17.1", Patch: "16.17", SnapshotIDs: []int64{12, 14}, AssetJobs: 2}, nil).Once()
	env.OnActivity("Activities.DownloadStaticAssetsForSnapshots", mock.Anything, []int64{12, 14}).
		Run(func(mock.Arguments) { order = append(order, "download-cdragon") }).
		Return(staticdata.DownloadResult{Total: 2, Completed: 2}, nil).Once()
	env.OnActivity("Activities.PublishStaticSnapshots", mock.Anything, []int64{12, 14}).
		Run(func(mock.Arguments) { order = append(order, "publish-cdragon") }).Return(nil).Once()
	env.OnActivity("Activities.SyncStaticSource", mock.Anything, staticdata.SourceSyncInput{Source: "ddragon", Build: "16.17.1", Patch: "16.17"}).
		Run(func(mock.Arguments) { order = append(order, "sync-ddragon") }).
		Return(staticdata.SyncResult{}, errors.New("ddragon unavailable"))

	env.ExecuteWorkflow(StaticSync)

	require.ErrorContains(t, env.GetWorkflowError(), "ddragon unavailable")
	require.Equal(t, []string{
		"sync-cdragon", "download-cdragon", "publish-cdragon",
		"sync-ddragon", "sync-ddragon", "sync-ddragon", "sync-ddragon", "sync-ddragon",
	}, order)
	env.AssertExpectations(t)
}

func TestStaticSyncReplaysLegacyGlobalDownloader(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(func(context.Context) (staticdata.SyncResult, error) { return staticdata.SyncResult{}, nil }, temporalactivity.RegisterOptions{Name: "Activities.SyncStatic"})
	env.RegisterActivityWithOptions(func(context.Context, int) (staticdata.DownloadResult, error) { return staticdata.DownloadResult{}, nil }, temporalactivity.RegisterOptions{Name: "Activities.DownloadStaticAssets"})
	env.RegisterActivityWithOptions(func(context.Context, []int64) error { return nil }, temporalactivity.RegisterOptions{Name: "Activities.PublishStaticSnapshots"})

	env.OnGetVersion(sourceFirstStaticSyncChangeID, temporalworkflow.DefaultVersion, 1).
		Return(temporalworkflow.DefaultVersion).Once()
	env.OnActivity("Activities.SyncStatic", mock.Anything).
		Return(staticdata.SyncResult{SnapshotIDs: []int64{11}, AssetJobs: 1}, nil).Once()
	env.OnGetVersion(snapshotScopedStaticDownloadChangeID, temporalworkflow.DefaultVersion, 1).
		Return(temporalworkflow.DefaultVersion).Once()
	env.OnActivity("Activities.DownloadStaticAssets", mock.Anything, 20).
		Return(staticdata.DownloadResult{Processed: 1}, nil).Once()
	env.OnActivity("Activities.PublishStaticSnapshots", mock.Anything, []int64{11}).Return(nil).Once()

	env.ExecuteWorkflow(StaticSync)

	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}

func TestStaticSyncNoOpDoesNotRepublishCurrentSnapshots(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(func(context.Context) (staticdata.SyncResult, error) { return staticdata.SyncResult{}, nil }, temporalactivity.RegisterOptions{Name: "Activities.SyncStatic"})
	env.OnGetVersion(sourceFirstStaticSyncChangeID, temporalworkflow.DefaultVersion, 1).
		Return(temporalworkflow.DefaultVersion).Once()
	env.OnActivity("Activities.SyncStatic", mock.Anything).
		Return(staticdata.SyncResult{Build: "16.17.1", Patch: "16.17"}, nil).Once()

	env.ExecuteWorkflow(StaticSync)

	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}
