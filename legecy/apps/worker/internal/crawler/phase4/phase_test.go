package phase4

import (
	"testing"

	"github.com/crafff/gogg/apps/worker/internal/storage"
)

func TestScoreToTierUsesLookupApexThresholds(t *testing.T) {
	thresholds := storage.ApexThresholds{
		ChallengerMinScore:  4000,
		GrandmasterMinScore: 3600,
	}
	tests := []struct {
		score    int
		tier     string
		division string
	}{
		{score: 2250, tier: "EMERALD", division: "II"},
		{score: 2800, tier: "MASTER", division: "I"},
		{score: 3600, tier: "GRANDMASTER", division: "I"},
		{score: 4000, tier: "CHALLENGER", division: "I"},
	}
	for _, test := range tests {
		t.Run(test.tier, func(t *testing.T) {
			tier, division := scoreToTier(test.score, thresholds)
			if tier != test.tier || division != test.division {
				t.Fatalf("scoreToTier(%d) = %s %s, want %s %s", test.score, tier, division, test.tier, test.division)
			}
		})
	}
}
