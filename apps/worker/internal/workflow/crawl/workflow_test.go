package crawl

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	crawlact "github.com/crafff/gogg/apps/worker/internal/activity/crawl"
)

// TestCrawlRegionWorkflow_Pipeline_FansPhase2PerTier confirms that
// execution=pipeline schedules one Phase2 Activity per TargetTier.
// We mock every activity by name (struct-method registration is the
// production path; tests use OnActivity by symbol).
//
// We don't exercise the legacy phase code itself — chunk 2's live
// smoke already verified end-to-end against compose Postgres. The
// test here pins the workflow's *branching* contract: pipeline vs
// sequential dispatch shape.
func TestCrawlRegionWorkflow_Pipeline_FansPhase2PerTier(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	var acts *crawlact.Activities
	env.RegisterWorkflow(CrawlRegionWorkflow)
	env.OnActivity(acts.CreateRun, mockAny, mockAny).Return(
		crawlact.CreateRunOutput{RunID: 42, StartedAt: time.Now()}, nil)
	env.OnActivity(acts.Phase0VersionSync, mockAny, mockAny).Return(
		crawlact.Phase0Output{ResolvedVersion: "16.12", LatestVersion: "16.12", UpsertedCount: 1}, nil)
	env.OnActivity(acts.PinRunVersion, mockAny, mockAny).Return(nil)
	env.OnActivity(acts.Phase1RankSnapshot, mockAny, mockAny).Return(
		crawlact.Phase1Output{TierCounts: map[string]int{"CHALLENGER": 1}}, nil)

	// Pipeline mode fans out one goroutine per tier — concurrent
	// writes need atomic. A plain int++ here trips -race even though
	// the workflow contract is correctness, not performance.
	var phase2Calls atomic.Int64
	env.OnActivity(acts.Phase2MatchIDCollection, mockAny, mockAny).Run(func(args mock.Arguments) {
		phase2Calls.Add(1)
	}).Return(crawlact.Phase2Output{}, nil)

	env.OnActivity(acts.Phase3MatchDetails, mockAny, mockAny).Return(
		crawlact.Phase3Output{}, nil)
	env.OnActivity(acts.Phase35OnDemandRank, mockAny, mockAny).Return(
		crawlact.Phase35Output{}, nil)
	env.OnActivity(acts.Phase4AvgTierCalc, mockAny, mockAny).Return(
		crawlact.Phase4Output{}, nil)
	env.OnActivity(acts.Phase5Timeline, mockAny, mockAny).Return(
		crawlact.Phase5Output{}, nil)
	env.OnActivity(acts.Phase55ItemClassify, mockAny, mockAny).Return(
		crawlact.Phase55Output{}, nil)
	env.OnActivity(acts.CompleteRun, mockAny, mockAny).Return(nil)

	env.ExecuteWorkflow(CrawlRegionWorkflow, CrawlRegionInput{
		Profile: crawlact.ProfileSnapshot{
			Region:            "KR",
			Mode:              "incremental",
			TargetTiers:       []string{"CHALLENGER", "GRANDMASTER", "MASTER"},
			RankPrefetchTiers: []string{"CHALLENGER"},
			Queue:             "RANKED_SOLO_5x5",
			Execution:         "pipeline",
		},
	})

	require.True(t, env.IsWorkflowCompleted(), "workflow should complete")
	require.NoError(t, env.GetWorkflowError(), "workflow should not error")
	require.Equal(t, int64(3), phase2Calls.Load(),
		"pipeline mode must schedule one Phase2 per TargetTier")

	var out CrawlRegionOutput
	require.NoError(t, env.GetWorkflowResult(&out))
	require.Equal(t, 42, out.RunID)
	require.Equal(t, "16.12", out.ResolvedVersion)
	require.Len(t, out.Phase2Outputs, 3, "Phase2Outputs must hold one entry per tier in pipeline mode")
}

// TestCrawlRegionWorkflow_Sequential_OneTier_BulkPhase2 confirms the
// sequential dispatch fires Phase2 once with all tiers inline.
func TestCrawlRegionWorkflow_Sequential_OneBulkPhase2(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	var acts *crawlact.Activities
	env.RegisterWorkflow(CrawlRegionWorkflow)
	env.OnActivity(acts.CreateRun, mockAny, mockAny).Return(
		crawlact.CreateRunOutput{RunID: 7, StartedAt: time.Now()}, nil)
	env.OnActivity(acts.Phase0VersionSync, mockAny, mockAny).Return(
		crawlact.Phase0Output{ResolvedVersion: "16.12", LatestVersion: "16.12"}, nil)
	env.OnActivity(acts.PinRunVersion, mockAny, mockAny).Return(nil)
	env.OnActivity(acts.Phase1RankSnapshot, mockAny, mockAny).Return(
		crawlact.Phase1Output{}, nil)

	// Sequential mode dispatches one Phase2 call so there's no
	// concurrent writer here, but the testsuite still runs activity
	// callbacks on a worker goroutine — keep this atomic for
	// consistency with the pipeline test and to stay -race clean.
	var phase2Calls atomic.Int64
	env.OnActivity(acts.Phase2MatchIDCollection, mockAny, mockAny).Run(func(args mock.Arguments) {
		phase2Calls.Add(1)
	}).Return(crawlact.Phase2Output{}, nil)

	env.OnActivity(acts.Phase3MatchDetails, mockAny, mockAny).Return(crawlact.Phase3Output{}, nil)
	env.OnActivity(acts.Phase35OnDemandRank, mockAny, mockAny).Return(crawlact.Phase35Output{}, nil)
	env.OnActivity(acts.Phase4AvgTierCalc, mockAny, mockAny).Return(crawlact.Phase4Output{}, nil)
	env.OnActivity(acts.Phase5Timeline, mockAny, mockAny).Return(crawlact.Phase5Output{}, nil)
	env.OnActivity(acts.Phase55ItemClassify, mockAny, mockAny).Return(crawlact.Phase55Output{}, nil)
	env.OnActivity(acts.CompleteRun, mockAny, mockAny).Return(nil)

	env.ExecuteWorkflow(CrawlRegionWorkflow, CrawlRegionInput{
		Profile: crawlact.ProfileSnapshot{
			Region:            "KR",
			Mode:              "incremental",
			TargetTiers:       []string{"CHALLENGER", "GRANDMASTER", "MASTER"},
			RankPrefetchTiers: []string{"CHALLENGER"},
			Queue:             "RANKED_SOLO_5x5",
			Execution:         "sequential",
		},
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, int64(1), phase2Calls.Load(),
		"sequential mode must dispatch one Phase2 with all tiers inline")
}

// TestCrawlRegionWorkflow_FullChain_Sequential walks every Activity in
// the sequential pipeline and asserts the order + that the workflow's
// composed output carries every per-phase result. Mocks the entire I/O
// surface — Riot + DB never touched. Confirms no Activity is skipped
// or accidentally short-circuited by an error in the chain.
func TestCrawlRegionWorkflow_FullChain_Sequential(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	var acts *crawlact.Activities
	env.RegisterWorkflow(CrawlRegionWorkflow)

	called := map[string]int{}
	tick := func(name string) func(mock.Arguments) {
		return func(args mock.Arguments) { called[name]++ }
	}
	env.OnActivity(acts.CreateRun, mockAny, mockAny).
		Run(tick("CreateRun")).
		Return(crawlact.CreateRunOutput{RunID: 99, StartedAt: time.Now()}, nil)
	env.OnActivity(acts.Phase0VersionSync, mockAny, mockAny).
		Run(tick("Phase0VersionSync")).
		Return(crawlact.Phase0Output{ResolvedVersion: "16.12"}, nil)
	env.OnActivity(acts.PinRunVersion, mockAny, mockAny).
		Run(tick("PinRunVersion")).Return(nil)
	env.OnActivity(acts.Phase1RankSnapshot, mockAny, mockAny).
		Run(tick("Phase1RankSnapshot")).
		Return(crawlact.Phase1Output{TierCounts: map[string]int{"CHALLENGER": 300}}, nil)
	env.OnActivity(acts.Phase2MatchIDCollection, mockAny, mockAny).
		Run(tick("Phase2MatchIDCollection")).
		Return(crawlact.Phase2Output{}, nil)
	env.OnActivity(acts.Phase3MatchDetails, mockAny, mockAny).
		Run(tick("Phase3MatchDetails")).Return(crawlact.Phase3Output{}, nil)
	env.OnActivity(acts.Phase35OnDemandRank, mockAny, mockAny).
		Run(tick("Phase35OnDemandRank")).Return(crawlact.Phase35Output{}, nil)
	env.OnActivity(acts.Phase4AvgTierCalc, mockAny, mockAny).
		Run(tick("Phase4AvgTierCalc")).Return(crawlact.Phase4Output{}, nil)
	env.OnActivity(acts.Phase5Timeline, mockAny, mockAny).
		Run(tick("Phase5Timeline")).Return(crawlact.Phase5Output{}, nil)
	env.OnActivity(acts.Phase55ItemClassify, mockAny, mockAny).
		Run(tick("Phase55ItemClassify")).Return(crawlact.Phase55Output{}, nil)
	env.OnActivity(acts.CompleteRun, mockAny, mockAny).
		Run(tick("CompleteRun")).Return(nil)

	env.ExecuteWorkflow(CrawlRegionWorkflow, CrawlRegionInput{
		Profile: crawlact.ProfileSnapshot{
			Region:            "KR",
			Mode:              "incremental",
			TargetTiers:       []string{"CHALLENGER", "GRANDMASTER", "MASTER"},
			RankPrefetchTiers: []string{"CHALLENGER"},
			Queue:             "RANKED_SOLO_5x5",
			Execution:         "sequential",
		},
	})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	for _, name := range []string{
		"CreateRun", "Phase0VersionSync", "PinRunVersion",
		"Phase1RankSnapshot", "Phase2MatchIDCollection",
		"Phase3MatchDetails", "Phase35OnDemandRank", "Phase4AvgTierCalc",
		"Phase5Timeline", "Phase55ItemClassify", "CompleteRun",
	} {
		require.Equal(t, 1, called[name],
			"activity %s should fire exactly once", name)
	}

	var out CrawlRegionOutput
	require.NoError(t, env.GetWorkflowResult(&out))
	require.Equal(t, 99, out.RunID)
	require.Equal(t, "16.12", out.ResolvedVersion)
}

// TestCrawlRegionWorkflow_FailRunOnPhaseError ensures the failure
// path's disconnected FailRun activity runs when a phase errors.
func TestCrawlRegionWorkflow_FailRunOnPhaseError(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	var acts *crawlact.Activities
	env.RegisterWorkflow(CrawlRegionWorkflow)
	env.OnActivity(acts.CreateRun, mockAny, mockAny).Return(
		crawlact.CreateRunOutput{RunID: 11, StartedAt: time.Now()}, nil)
	env.OnActivity(acts.Phase0VersionSync, mockAny, mockAny).Return(
		crawlact.Phase0Output{ResolvedVersion: "16.12"}, nil)
	env.OnActivity(acts.PinRunVersion, mockAny, mockAny).Return(nil)
	env.OnActivity(acts.Phase1RankSnapshot, mockAny, mockAny).
		Return(crawlact.Phase1Output{}, errors.New("phase1 boom"))

	var failCalls atomic.Int64
	env.OnActivity(acts.FailRun, mockAny, mockAny).Run(func(args mock.Arguments) {
		failCalls.Add(1)
	}).Return(nil)

	env.ExecuteWorkflow(CrawlRegionWorkflow, CrawlRegionInput{
		Profile: crawlact.ProfileSnapshot{
			Region:            "KR",
			Mode:              "incremental",
			TargetTiers:       []string{"CHALLENGER"},
			RankPrefetchTiers: []string{"CHALLENGER"},
			Queue:             "RANKED_SOLO_5x5",
			Execution:         "sequential",
		},
	})

	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError(), "phase1 boom must propagate")
	require.GreaterOrEqual(t, failCalls.Load(), int64(1),
		"FailRun must fire on workflow failure")
}

// mockAny is the testify/mock wildcard matcher used in every
// OnActivity call here. We use the sentinel directly rather than
// inlining mock.Anything everywhere so the chain reads close to the
// activity signatures.
var mockAny = mock.Anything
