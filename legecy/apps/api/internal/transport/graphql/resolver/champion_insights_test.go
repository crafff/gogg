package resolver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/crafff/gogg/apps/api/internal/service/championinsights"
	"github.com/crafff/gogg/apps/api/internal/transport/graphql/domainerr"
	gqlgenerated "github.com/crafff/gogg/apps/api/internal/transport/graphql/generated"
)

type fakeChampionInsightsService struct {
	result *championinsights.Result
	err    error
	filter championinsights.Filter
}

func (f *fakeChampionInsightsService) Get(_ context.Context, _ int, filter championinsights.Filter) (*championinsights.Result, error) {
	f.filter = filter
	return f.result, f.err
}

func TestChampionWinFactorsMapsObservedPublication(t *testing.T) {
	now := time.Now().UTC()
	service := &fakeChampionInsightsService{result: &championinsights.Result{
		ChampionID: 64, ChampionName: "LeeSin", Position: "JUNGLE",
		ResolvedVersion: "16.14", RegionScope: "KR", TierGroup: "MASTER_PLUS",
		Revision: "revision", Algorithm: "OBSERVED_PERCENTILES_V1",
		DataThrough: &now, PublishedAt: &now, Availability: "AVAILABLE",
		CohortScope: "CHAMPION_POSITION", SampleGames: 1000, SamplePlayers: 700,
		Factors: []championinsights.Factor{{
			MetricKey: "JUNGLE_CS_10", Kind: "BEHAVIOR_METRIC", EndMinute: 10,
			Unit: "COUNT", P50: 60, P70: 70, P90: 80, EvidenceGrade: "OBSERVED",
			DisplayOrder: 1, Buckets: []championinsights.Bucket{{
				Ordinal: 1, LowerBound: 20, UpperBound: 59, Games: 100,
				Wins: 40, SamplePlayers: 75, ObservedWinRate: 40, ObservedWinRateDelta: -10,
			}},
		}},
	}}
	queueID, version, region := 420, "16.14", "KR"
	tier := gqlgenerated.TierGroupMasterPlus
	resolver := &queryResolver{Resolver: &Resolver{ChampionInsights: service}}

	got, err := resolver.ChampionWinFactors(context.Background(), 64, gqlgenerated.ChampionWinFactorsFilter{
		QueueID: &queueID, Version: &version, Region: &region, TierGroup: &tier, Position: "JUNGLE",
	})
	require.NoError(t, err)
	require.Equal(t, "16.14", *got.ResolvedVersion)
	require.Equal(t, "revision", *got.Revision)
	require.Equal(t, "CHAMPION_POSITION", *got.CohortScope)
	require.Equal(t, 75, got.Factors[0].Buckets[0].SamplePlayers)
	require.Equal(t, "KR", service.filter.Region)
	require.Equal(t, "MASTER_PLUS", service.filter.TierGroup)
}

func TestChampionWinFactorsMapsValidationError(t *testing.T) {
	service := &fakeChampionInsightsService{err: &championinsights.ValidationError{
		Field: "version", Message: "must use major.minor format",
	}}
	resolver := &queryResolver{Resolver: &Resolver{ChampionInsights: service}}
	_, err := resolver.ChampionWinFactors(context.Background(), 64, gqlgenerated.ChampionWinFactorsFilter{Position: "JUNGLE"})
	var public *domainerr.Error
	require.ErrorAs(t, err, &public)
	require.Equal(t, "BAD_USER_INPUT", public.Code)
}
