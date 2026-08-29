package workflow

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	temporalactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
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
	env.OnActivity("Activities.ResolveTargetPatch", mock.Anything, "").Return("16.17", nil).Once()
	env.OnActivity("Activities.StartRun", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { start = args.Get(1).(activity.StartRunInput) }).
		Return(int64(0), errors.New("stop after identity capture")).
		Once()

	env.ExecuteWorkflow(Crawl, tftcontract.CrawlInput{Platforms: []string{"KR"}})

	require.Error(t, env.GetWorkflowError())
	require.Equal(t, "gogg-tft-crawl", start.WorkflowID)
	require.NotEmpty(t, start.WorkflowRunID)
	require.NotEqual(t, start.WorkflowID, start.WorkflowRunID)
	require.Equal(t, []string{"KR"}, start.Platforms)
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
