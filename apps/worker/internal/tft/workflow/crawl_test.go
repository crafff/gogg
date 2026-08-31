package workflow

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	temporalactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	temporalworkflow "go.temporal.io/sdk/workflow"

	"github.com/crafff/gogg/apps/worker/internal/tft/activity"
	"github.com/crafff/gogg/packages/tftcontract"
)

func TestCrawlRejectsEmptyPlatforms(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.ExecuteWorkflow(Crawl, tftcontract.CrawlInput{})
	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
}

func TestSetTerminalDesiredState(t *testing.T) {
	_ = activity.RunStateInput{}
}

func TestRouteDispatchWaitsForFutureRetryAndDrainsQueue(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(
		func(context.Context, activity.DispatchInput) (activity.BatchResult, error) {
			return activity.BatchResult{}, nil
		},
		temporalactivity.RegisterOptions{Name: "Activities.DispatchMatches"},
	)
	env.OnActivity("Activities.DispatchMatches", mock.Anything, mock.Anything).
		Return(activity.BatchResult{Remaining: 1, HasMore: true, NextEligibleAt: time.Now().Add(time.Minute)}, nil).
		Once()
	env.OnActivity("Activities.DispatchMatches", mock.Anything, mock.Anything).
		Return(activity.BatchResult{}, nil).
		Once()

	env.ExecuteWorkflow(RouteDispatch, tftcontract.RouteInput{RunID: 42, RoutingRegion: "ASIA"})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}

func TestCrawlUsesExecutionRunIDForDatabaseRunIdentity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{ID: "gogg-tft-crawl"})
	env.RegisterActivityWithOptions(activity.New(nil), temporalactivity.RegisterOptions{Name: "Activities."})

	var start activity.StartRunInput
	env.OnGetVersion(balancedMatchDiscoveryChangeID, temporalworkflow.DefaultVersion, 1).Return(temporalworkflow.Version(1)).Once()
	env.OnActivity("Activities.ResolveTargetPatch", mock.Anything, "").Return("16.17", nil).Once()
	env.OnActivity("Activities.StartBalancedRun", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { start = args.Get(1).(activity.StartRunInput) }).
		Return(int64(0), temporal.NewNonRetryableApplicationError("stop after identity capture", "TEST_STOP", nil)).
		Once()

	env.ExecuteWorkflow(Crawl, tftcontract.CrawlInput{Platforms: []string{"KR"}})

	require.Error(t, env.GetWorkflowError())
	require.Equal(t, "gogg-tft-crawl", start.WorkflowID)
	require.NotEmpty(t, start.WorkflowRunID)
	require.NotEqual(t, start.WorkflowID, start.WorkflowRunID)
	require.Equal(t, []string{"KR"}, start.Platforms)
	env.AssertExpectations(t)
}

func TestCrawlBalancesCandidatesBeforeRouteDispatch(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(activity.New(nil), temporalactivity.RegisterOptions{Name: "Activities."})
	var mu sync.Mutex
	sequence := make([]string, 0, 6)
	record := func(value string) {
		mu.Lock()
		defer mu.Unlock()
		sequence = append(sequence, value)
	}

	env.OnGetVersion(balancedMatchDiscoveryChangeID, temporalworkflow.DefaultVersion, 1).Return(temporalworkflow.Version(1)).Once()
	env.OnActivity("Activities.ResolveTargetPatch", mock.Anything, "").Return("16.17", nil).Once()
	env.OnActivity("Activities.StartBalancedRun", mock.Anything, mock.Anything).Return(int64(42), nil).Once()
	env.OnActivity("Activities.RequeueTargetPatch", mock.Anything, "16.17").Return(int64(0), nil).Once()
	env.OnWorkflow(PlatformCandidates, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { record("candidates") }).Return(nil).Once()
	env.OnActivity("Activities.FinalizeBalancedMatches", mock.Anything, activity.FinalizeBalancedMatchesInput{RunID: 42}).
		Run(func(mock.Arguments) { record("finalize") }).Return(activity.FinalizeBalancedMatchesResult{TargetPerRegion: 1000}, nil).Once()
	env.OnWorkflow(RouteDispatch, mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { record("route") }).Return(nil).Times(4)
	env.OnActivity("Activities.ListAnalysisTargets", mock.Anything, mock.Anything).Return([]activity.AnalysisTarget{}, nil).Once()
	env.OnActivity("Activities.SetRunState", mock.Anything, mock.Anything).Return(nil).Once()

	env.ExecuteWorkflow(Crawl, tftcontract.CrawlInput{Platforms: []string{"KR"}})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	mu.Lock()
	defer mu.Unlock()
	candidateIndex := slices.Index(sequence, "candidates")
	finalizeIndex := slices.Index(sequence, "finalize")
	routeIndex := slices.Index(sequence, "route")
	require.GreaterOrEqual(t, candidateIndex, 0, sequence)
	require.Greater(t, finalizeIndex, candidateIndex, sequence)
	require.Greater(t, routeIndex, finalizeIndex, sequence)
	env.AssertExpectations(t)
}

func TestRunPlatformStageUsesCoroutineContext(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.OnWorkflow(PlatformSeed, mock.Anything, mock.Anything).After(time.Second).Return(nil).Twice()

	env.ExecuteWorkflow(platformStageContextTestWorkflow)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var status tftcontract.CrawlStatus
	require.NoError(t, env.GetWorkflowResult(&status))
	require.Equal(t, 2, status.StageCompleted)
	require.Equal(t, 2, status.StageTotal)
	env.AssertExpectations(t)
}

func TestRunCandidatePlatformStageUsesDedicatedWorkflow(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.OnWorkflow(PlatformCandidates, mock.Anything, mock.Anything).After(time.Second).Return(nil).Twice()

	env.ExecuteWorkflow(candidatePlatformStageContextTestWorkflow)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var status tftcontract.CrawlStatus
	require.NoError(t, env.GetWorkflowResult(&status))
	require.Equal(t, 2, status.StageCompleted)
	require.Equal(t, 2, status.StageTotal)
	env.AssertExpectations(t)
}

func TestRunRouteStageUsesCoroutineContext(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.OnWorkflow(RouteDispatch, mock.Anything, mock.Anything).After(time.Second).Return(nil).Times(4)

	env.ExecuteWorkflow(routeStageContextTestWorkflow)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var status tftcontract.CrawlStatus
	require.NoError(t, env.GetWorkflowResult(&status))
	require.Equal(t, 4, status.StageCompleted)
	require.Equal(t, 4, status.StageTotal)
	env.AssertExpectations(t)
}

func TestRunAnalysisStageUsesCoroutineContext(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(activity.New(nil), temporalactivity.RegisterOptions{Name: "Activities."})
	targets := []activity.AnalysisTarget{
		{Platform: "NA1", Patch: "16.17", SetNumber: 15, QueueID: 1100},
		{Platform: "KR", Patch: "16.17", SetNumber: 15, QueueID: 1100},
	}
	env.OnActivity("Activities.ListAnalysisTargets", mock.Anything, mock.Anything).Return(targets, nil).Once()
	env.OnActivity("Activities.PublishAnalysis", mock.Anything, mock.Anything).
		After(time.Second).
		Return(activity.PublishAnalysisResult{}, nil).
		Times(8)

	env.ExecuteWorkflow(analysisStageContextTestWorkflow)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var status tftcontract.CrawlStatus
	require.NoError(t, env.GetWorkflowResult(&status))
	require.Equal(t, 2, status.StageCompleted)
	require.Equal(t, 2, status.StageTotal)
	env.AssertExpectations(t)
}

func TestWaitStageExposesCompletedUnits(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var queried tftcontract.CrawlStatus
	env.RegisterDelayedCallback(func() {
		value, err := env.QueryWorkflow(tftcontract.CrawlStatusQueryName)
		require.NoError(t, err)
		require.NoError(t, value.Get(&queried))
	}, 45*time.Second)

	env.ExecuteWorkflow(stageProgressTestWorkflow)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 1, queried.StageCompleted)
	require.Equal(t, 2, queried.StageTotal)
}

func TestWaitStageDoesNotCountFailedUnitAsCompleted(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.ExecuteWorkflow(stageFailureProgressTestWorkflow)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var status tftcontract.CrawlStatus
	require.NoError(t, env.GetWorkflowResult(&status))
	require.Equal(t, 1, status.StageCompleted)
	require.Equal(t, 2, status.StageTotal)
}

func stageProgressTestWorkflow(ctx temporalworkflow.Context) error {
	state := tftcontract.CrawlStatus{State: "running", Stage: "match_detail", StageTotal: 2}
	if err := temporalworkflow.SetQueryHandler(ctx, tftcontract.CrawlStatusQueryName, func() (tftcontract.CrawlStatus, error) {
		return state, nil
	}); err != nil {
		return err
	}
	results := temporalworkflow.NewBufferedChannel(ctx, 2)
	for index, delay := range []time.Duration{30 * time.Second, 90 * time.Second} {
		index, delay := index, delay
		temporalworkflow.Go(ctx, func(gctx temporalworkflow.Context) {
			if err := temporalworkflow.NewTimer(gctx, delay).Get(gctx, nil); err == nil {
				results.Send(gctx, stageResult{Key: fmt.Sprintf("stage-%d", index)})
			}
		})
	}
	control := temporalworkflow.GetSignalChannel(ctx, "stage-progress-test-control")
	_, cancel := temporalworkflow.WithCancel(ctx)
	_, _, err := waitStage(ctx, control, results, 2, cancel, &state)
	return err
}

func stageFailureProgressTestWorkflow(ctx temporalworkflow.Context) (tftcontract.CrawlStatus, error) {
	state := tftcontract.CrawlStatus{State: "running", Stage: "match_detail", StageTotal: 2}
	results := temporalworkflow.NewBufferedChannel(ctx, 2)
	results.Send(ctx, stageResult{Key: "success"})
	results.Send(ctx, stageResult{Key: "failure", Err: errors.New("failed child")})
	control := temporalworkflow.GetSignalChannel(ctx, "stage-failure-test-control")
	_, cancel := temporalworkflow.WithCancel(ctx)
	_, _, _ = waitStage(ctx, control, results, 2, cancel, &state)
	return state, nil
}

func platformStageContextTestWorkflow(ctx temporalworkflow.Context) (tftcontract.CrawlStatus, error) {
	state := tftcontract.CrawlStatus{State: "running", Stage: "seed_and_discover"}
	control := temporalworkflow.GetSignalChannel(ctx, "platform-stage-context-test-control")
	_, _, err := runPlatformStage(ctx, control, tftcontract.CrawlInput{Platforms: []string{"NA1", "KR"}}, 42, &state)
	return state, err
}

func candidatePlatformStageContextTestWorkflow(ctx temporalworkflow.Context) (tftcontract.CrawlStatus, error) {
	state := tftcontract.CrawlStatus{State: "running", Stage: "seed_and_candidate_discover"}
	control := temporalworkflow.GetSignalChannel(ctx, "candidate-platform-stage-context-test-control")
	_, _, err := runCandidatePlatformStage(ctx, control, tftcontract.CrawlInput{Platforms: []string{"NA1", "KR"}}, 42, &state)
	return state, err
}

func routeStageContextTestWorkflow(ctx temporalworkflow.Context) (tftcontract.CrawlStatus, error) {
	state := tftcontract.CrawlStatus{State: "running", Stage: "match_detail"}
	control := temporalworkflow.GetSignalChannel(ctx, "route-stage-context-test-control")
	_, _, err := runRouteStage(ctx, control, 42, "16.17", false, &state)
	return state, err
}

func analysisStageContextTestWorkflow(ctx temporalworkflow.Context) (tftcontract.CrawlStatus, error) {
	windowEnd := time.Date(2026, 8, 29, 18, 0, 0, 0, time.UTC)
	state := tftcontract.CrawlStatus{State: "running", Stage: "analysis"}
	control := temporalworkflow.GetSignalChannel(ctx, "analysis-stage-context-test-control")
	_, _, err := runAnalysisStage(withCrawlActivityOptions(ctx), control, tftcontract.CrawlInput{
		WindowStart: windowEnd.Add(-7 * 24 * time.Hour),
		WindowEnd:   windowEnd,
	}, 42, &state)
	return state, err
}
