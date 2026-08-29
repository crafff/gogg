package enrich

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	sumact "github.com/crafff/gogg/apps/worker/internal/activity/summoner"
)

func TestEnrichSummonerWorkflowCompletes(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	var activities *sumact.Activities

	env.RegisterWorkflow(EnrichSummonerWorkflow)
	env.OnActivity(activities.ResolveAndRefreshProfile, mock.Anything, mock.Anything).
		Return(sumact.ResolveOutput{PUUID: "puuid-1", GameName: "Faker", TagLine: "KR1"}, nil)
	env.OnActivity(activities.FetchRecentMatches, mock.Anything, mock.Anything).
		Return(sumact.FetchOutput{Scanned: 100, Supported: 44, Fetched: 12, Failed: 1}, nil)
	env.OnActivity(activities.EnrichMatchRanks, mock.Anything, mock.MatchedBy(func(input sumact.EnrichRanksInput) bool {
		return input.JobID == "job-1" && input.Region == "KR"
	})).Return(sumact.EnrichRanksOutput{
		Targets: 203, Ranked: 180, Unranked: 21, Failed: 2, MatchesComputed: 44,
	}, nil).Once()
	env.OnActivity(activities.FinalizeLookupJob, mock.Anything, mock.MatchedBy(func(input sumact.FinalizeInput) bool {
		return input.JobID == "job-1" && input.PUUID == "puuid-1" &&
			input.MatchFailures == 1 && input.RankFailures == 2
	})).Return(nil).Once()

	env.ExecuteWorkflow(EnrichSummonerWorkflow, SummonerInput{
		JobID: "job-1", Region: "KR", GameName: "Faker", TagLine: "KR1",
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var output SummonerOutput
	require.NoError(t, env.GetWorkflowResult(&output))
	require.Equal(t, SummonerOutput{
		PUUID: "puuid-1", Scanned: 100, Supported: 44, Fetched: 12, Failed: 1,
		RankTargets: 203, Ranked: 180, Unranked: 21, RankFailed: 2, MatchesRanked: 44,
	}, output)
	env.AssertExpectations(t)
}

func TestEnrichSummonerWorkflowRecordsFailure(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	var activities *sumact.Activities

	env.RegisterWorkflow(EnrichSummonerWorkflow)
	env.OnActivity(activities.ResolveAndRefreshProfile, mock.Anything, mock.Anything).
		Return(sumact.ResolveOutput{}, temporal.NewNonRetryableApplicationError("not found", "NOT_FOUND", errors.New("404")))
	env.OnActivity(activities.FailLookupJob, mock.Anything, mock.MatchedBy(func(input sumact.FailInput) bool {
		return input.JobID == "job-2" && input.Code == "NOT_FOUND"
	})).Return(nil).Once()

	env.ExecuteWorkflow(EnrichSummonerWorkflow, SummonerInput{
		JobID: "job-2", Region: "KR", GameName: "Missing", TagLine: "KR1",
	})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}

func TestEnrichSummonerWorkflowReplaysPreRankEnrichmentHistory(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	var activities *sumact.Activities

	env.RegisterWorkflow(EnrichSummonerWorkflow)
	env.OnActivity(activities.ResolveAndRefreshProfile, mock.Anything, mock.Anything).
		Return(sumact.ResolveOutput{PUUID: "legacy-puuid"}, nil).Once()
	env.OnActivity(activities.FetchRecentMatches, mock.Anything, mock.Anything).
		Return(sumact.FetchOutput{Scanned: 10, Supported: 8, Fetched: 3}, nil).Once()
	env.OnGetVersion("summoner-rank-enrichment", workflow.DefaultVersion, 1).
		Return(workflow.DefaultVersion).Once()

	env.ExecuteWorkflow(EnrichSummonerWorkflow, SummonerInput{
		JobID: "legacy-job", Region: "NA1", GameName: "Legacy", TagLine: "NA1",
	})

	require.NoError(t, env.GetWorkflowError())
	var output SummonerOutput
	require.NoError(t, env.GetWorkflowResult(&output))
	require.Equal(t, SummonerOutput{
		PUUID: "legacy-puuid", Scanned: 10, Supported: 8, Fetched: 3,
	}, output)
	env.AssertExpectations(t)
}
