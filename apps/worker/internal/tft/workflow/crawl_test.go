package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	temporalactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"

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
	env.AssertExpectations(t)
}
