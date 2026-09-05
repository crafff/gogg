package summoner

import (
	"errors"
	"testing"

	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
	"go.temporal.io/sdk/temporal"
)

func TestNotFoundInRegionIsExplicitAndNonRetryable(t *testing.T) {
	got := notFoundInRegionError(errors.New("riot 404"))

	var applicationErr *temporal.ApplicationError
	if !errors.As(got, &applicationErr) {
		t.Fatalf("error type = %T, want *temporal.ApplicationError", got)
	}
	if applicationErr.Type() != "NOT_FOUND_IN_REGION" {
		t.Fatalf("type = %q", applicationErr.Type())
	}
	if !applicationErr.NonRetryable() {
		t.Fatal("wrong-region error must not be retried")
	}
}

func TestRankedSoloEntryIgnoresOtherQueues(t *testing.T) {
	entries := []riotapi.LeagueEntryDTO{
		{QueueType: "RANKED_FLEX_SR", Tier: "DIAMOND", Rank: "I", LeaguePoints: 80},
		{QueueType: rankedSoloQueue, Tier: "emerald", Rank: "II", LeaguePoints: 42},
	}

	entry, ok := rankedSoloEntry(entries)
	if !ok {
		t.Fatal("ranked solo entry was not selected")
	}
	if entry.Tier != "emerald" || entry.Rank != "II" || entry.LeaguePoints != 42 {
		t.Fatalf("entry = %#v", entry)
	}
	if _, ok := rankedSoloEntry(entries[:1]); ok {
		t.Fatal("flex-only player must be treated as unranked for the solo-rank snapshot")
	}
}

func TestLookupApexThresholdsUsesObservedMinimumLP(t *testing.T) {
	got := lookupApexThresholds(sqlcgen.GetSummonerLookupApexThresholdsRow{
		ChallengerMinLp: 1200, GrandmasterMinLp: 800,
	})
	want := storage.ApexThresholds{ChallengerMinScore: 4000, GrandmasterMinScore: 3600}
	if got != want {
		t.Fatalf("thresholds = %#v, want %#v", got, want)
	}

	missing := lookupApexThresholds(sqlcgen.GetSummonerLookupApexThresholdsRow{
		ChallengerMinLp: -1, GrandmasterMinLp: -1,
	})
	if missing.ChallengerMinScore <= masterBaseScore || missing.GrandmasterMinScore <= masterBaseScore {
		t.Fatalf("missing apex thresholds must be unreachable: %#v", missing)
	}
}
