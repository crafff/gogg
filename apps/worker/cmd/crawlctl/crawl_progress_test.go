package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

func TestRenderCrawlProgressSeparatesRunAndGlobalQueue(t *testing.T) {
	started := time.Now().Add(-2 * time.Minute)
	patch := "16.17"
	snapshot := crawlProgressSnapshot{
		Run: sqlcgen.TftCrawlRun{
			ID: 42, WorkflowID: "tft-workflow", Status: "running", Stage: "seed",
			TargetPatch: &patch, Config: []byte(`{"platforms":["KR","NA1"]}`),
			StartedAt: pgtype.Timestamptz{Time: started, Valid: true},
		},
		Progress: sqlcgen.GetTFTRunProgressRow{
			DiscoveredSeeds: 40, SelectedSeeds: 20, CompletedPlatforms: 2,
		},
		Routes: []sqlcgen.ListTFTRunRouteProgressRow{{
			RoutingRegion: "AMERICAS", DiscoveredMatches: 8, CompletedMatches: 4,
			TerminalMatches: 1, PendingMatches: 1, RetryMatches: 1, LeasedMatches: 1,
			GlobalPendingMatches: 7, GlobalRetryMatches: 2, GlobalLeasedMatches: 1,
		}},
	}

	var output bytes.Buffer
	renderCrawlProgress(&output, snapshot, "match_detail", false)
	text := output.String()
	require.Contains(t, text, "persisted_run=42")
	require.Contains(t, text, "platforms=2/2")
	require.Contains(t, text, "matches=5/8 percent=62.5")
	require.Contains(t, text, "remaining=3 pending=1 retry=1 leased=1 not_enqueued=0")
	require.Contains(t, text, "route=AMERICAS run_matches=5/8")
	require.Contains(t, text, "global_queue_remaining=10")
	require.Contains(t, text, "eta=unavailable")
}

func TestRenderCrawlProgressDoesNotClaimStablePercentageDuringDiscovery(t *testing.T) {
	snapshot := crawlProgressSnapshot{
		Run:    sqlcgen.TftCrawlRun{ID: 7, WorkflowID: "discovering", Status: "running", Config: []byte(`{}`)},
		Routes: []sqlcgen.ListTFTRunRouteProgressRow{{DiscoveredMatches: 2, CompletedMatches: 2}},
	}

	var output bytes.Buffer
	renderCrawlProgress(&output, snapshot, "seed_and_discover", false)
	require.Contains(t, output.String(), "percent=growing")
}

func TestRenderCrawlProgressShowsBalancedCandidatesAndShortfall(t *testing.T) {
	snapshot := crawlProgressSnapshot{
		Run: sqlcgen.TftCrawlRun{
			ID: 9, WorkflowID: "balanced", Status: "running",
			Config: []byte(`{"platforms":["NA1","KR"],"match_target_per_region":1000,"match_selection_revision":"route-balance-v1"}`),
		},
		Routes: []sqlcgen.ListTFTRunRouteProgressRow{{
			RoutingRegion: "AMERICAS", DiscoveredMatches: 946, PendingMatches: 946,
		}},
		Candidates: []sqlcgen.ListTFTRunCandidateRouteProgressRow{{
			RoutingRegion: "AMERICAS", CandidateMatches: 946, SelectedMatches: 946,
		}},
		Sampling: &sqlcgen.TftRunMatchSampling{Phase: "finalized", TargetPerRegion: 1000},
	}

	var output bytes.Buffer
	renderCrawlProgress(&output, snapshot, "unknown", false)
	require.Contains(t, output.String(), "stage=unknown")
	require.Contains(t, output.String(), "route=AMERICAS candidates=946 target=1000 admitted=946 shortfall=54 run_matches=0/946")

	output.Reset()
	snapshot.Sampling.Phase = "open"
	renderCrawlProgress(&output, snapshot, "seed_and_candidate_discover", false)
	require.Contains(t, output.String(), "admitted=pending shortfall=pending")
}

func TestRenderCrawlProgressCanInferStableTotalWhenTemporalStageIsUnavailable(t *testing.T) {
	snapshot := crawlProgressSnapshot{
		Run:      sqlcgen.TftCrawlRun{ID: 8, WorkflowID: "fallback", Status: "running", Config: []byte(`{"platforms":["KR"]}`)},
		Progress: sqlcgen.GetTFTRunProgressRow{CompletedPlatforms: 1},
		Routes:   []sqlcgen.ListTFTRunRouteProgressRow{{DiscoveredMatches: 2, CompletedMatches: 2}},
	}

	var output bytes.Buffer
	renderCrawlProgress(&output, snapshot, "unknown", false)
	require.Contains(t, output.String(), "stage=unknown")
	require.Contains(t, output.String(), "matches=2/2 percent=100.0")
}

func TestLatestScheduledWorkflowUsesNewestWorkflowAction(t *testing.T) {
	description := &client.ScheduleDescription{Info: client.ScheduleInfo{RecentActions: []client.ScheduleActionResult{
		{StartWorkflowResult: &client.ScheduleWorkflowExecution{WorkflowID: "old", FirstExecutionRunID: "old-run"}},
		{},
		{StartWorkflowResult: &client.ScheduleWorkflowExecution{WorkflowID: "new", FirstExecutionRunID: "new-run"}},
	}}}

	result := latestScheduledWorkflow(description)
	require.NotNil(t, result)
	require.Equal(t, "new-run", result.FirstExecutionRunID)
}

func TestConfiguredPlatformCountRejectsInvalidConfig(t *testing.T) {
	require.Equal(t, int64(0), configuredPlatformCount([]byte(`not-json`)))
	require.Equal(t, int64(3), configuredPlatformCount([]byte(`{"platforms":["KR","NA1","EUW1"]}`)))
}

func TestConfiguredMatchBalanceRequiresTargetAndRevision(t *testing.T) {
	target, ok := configuredMatchBalance([]byte(`{"match_target_per_region":1000,"match_selection_revision":"route-balance-v1"}`))
	require.True(t, ok)
	require.Equal(t, 1000, target)
	_, ok = configuredMatchBalance([]byte(`{"match_target_per_region":1000}`))
	require.False(t, ok)
}

func TestStableMatchTotal(t *testing.T) {
	for _, stage := range []string{"match_detail", "analysis", "completed", "failed"} {
		require.True(t, stableMatchTotal(stage), stage)
	}
	for _, stage := range []string{"starting", "seed", "seed_and_discover", ""} {
		require.False(t, stableMatchTotal(stage), stage)
	}
}

func TestProgressOutputUsesSingleLineFields(t *testing.T) {
	var output bytes.Buffer
	renderCrawlProgress(&output, crawlProgressSnapshot{Run: sqlcgen.TftCrawlRun{ID: 1}}, "starting", true)
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		require.NotContains(t, line, "\n")
	}
}
