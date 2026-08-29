package workflow

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	temporalactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"

	"github.com/crafff/gogg/apps/worker/internal/tft/activity"
	"github.com/crafff/gogg/packages/tftcontract"
)

func TestPlayerLookupCompletesAfterHistoryFetch(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(activity.New(nil), temporalactivity.RegisterOptions{Name: "Activities."})
	env.OnActivity("Activities.ResolvePlayer", mock.Anything, activity.ResolvePlayerInput{JobID: "job-1", Platform: "KR", GameName: "Player", TagLine: "KR1"}).
		Return(activity.ResolvePlayerResult{PUUID: "puuid-1"}, nil).Once()
	env.OnActivity("Activities.FetchPlayerHistory", mock.Anything, activity.FetchPlayerHistoryInput{JobID: "job-1", Platform: "KR", PUUID: "puuid-1", Count: 20}).
		Return(activity.FetchPlayerHistoryResult{Scanned: 2, Fetched: 2}, nil).Once()
	env.OnActivity("Activities.FinishPlayerLookup", mock.Anything, activity.FinishPlayerLookupInput{JobID: "job-1", Status: "COMPLETED"}).
		Return(nil).Once()

	env.ExecuteWorkflow(PlayerLookup, tftcontract.PlayerLookupInput{JobID: "job-1", Platform: "KR", GameName: "Player", TagLine: "KR1"})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}

func TestPlayerLookupPersistsTerminalResolveFailure(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivityWithOptions(activity.New(nil), temporalactivity.RegisterOptions{Name: "Activities."})
	resolveErr := temporal.NewNonRetryableApplicationError("not found", "NOT_FOUND", nil)
	env.OnActivity("Activities.ResolvePlayer", mock.Anything, mock.Anything).
		Return(activity.ResolvePlayerResult{}, resolveErr).Once()
	env.OnActivity("Activities.FinishPlayerLookup", mock.Anything, mock.MatchedBy(func(input activity.FinishPlayerLookupInput) bool {
		return input.JobID == "job-2" && input.Status == "FAILED" && input.ErrorCode == "NOT_FOUND"
	})).Return(nil).Once()

	env.ExecuteWorkflow(PlayerLookup, tftcontract.PlayerLookupInput{JobID: "job-2", Platform: "KR", GameName: "Missing", TagLine: "KR1"})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}
