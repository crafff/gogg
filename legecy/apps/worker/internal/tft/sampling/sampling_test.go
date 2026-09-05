package sampling

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSelectHighTierStratified(t *testing.T) {
	seeds := []Seed{{ID: 1, Puuid: "c", Tier: "CHALLENGER"}, {ID: 2, Puuid: "g", Tier: "GRANDMASTER"}}
	for i := 0; i < 100; i++ {
		seeds = append(seeds, Seed{ID: int64(10 + i), Puuid: fmt.Sprintf("m%03d", i), Tier: "MASTER", LeaguePoints: 100 - i})
	}
	for i := 0; i < 20; i++ {
		seeds = append(seeds, Seed{ID: int64(200 + i), Puuid: fmt.Sprintf("d%03d", i), Tier: "DIAMOND", Division: "I"})
	}
	got := Select(seeds, "KR|16.17", Limits{Master: 50, DiamondPerDivision: 5})
	require.True(t, got[1])
	require.True(t, got[2])
	require.Len(t, got, 57)
	require.Equal(t, got, Select(seeds, "KR|16.17", Limits{Master: 50, DiamondPerDivision: 5}))
}

func TestEffectiveLimitsScalesOnceAfterFortyEightHours(t *testing.T) {
	base := Limits{Master: 500, DiamondPerDivision: 50}
	scaled := Limits{Master: 1000, DiamondPerDivision: 100}
	require.Equal(t, base, EffectiveLimits(47*time.Hour, 0, base, scaled, 48*time.Hour, 10000))
	require.Equal(t, scaled, EffectiveLimits(49*time.Hour, 9999, base, scaled, 48*time.Hour, 10000))
	require.Equal(t, base, EffectiveLimits(49*time.Hour, 10000, base, scaled, 48*time.Hour, 10000))
}
