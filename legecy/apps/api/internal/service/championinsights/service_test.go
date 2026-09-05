package championinsights

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

type fakeQuerier struct {
	identity    sqlcgen.GetChampionIdentityRow
	identityErr error
	latest      string
	latestErr   error
	cohort      sqlcgen.GetPublishedChampionInsightCohortRow
	cohortErr   error
	rows        []sqlcgen.ListPublishedChampionInsightFactorsRow
	params      sqlcgen.GetPublishedChampionInsightCohortParams
}

func (f *fakeQuerier) GetChampionIdentity(context.Context, int32) (sqlcgen.GetChampionIdentityRow, error) {
	return f.identity, f.identityErr
}
func (f *fakeQuerier) GetPublishedChampionInsightIdentity(context.Context, int32) (sqlcgen.GetPublishedChampionInsightIdentityRow, error) {
	if f.identityErr != nil {
		return sqlcgen.GetPublishedChampionInsightIdentityRow{}, pgx.ErrNoRows
	}
	return sqlcgen.GetPublishedChampionInsightIdentityRow{
		ChampionID: f.identity.ChampionID, ChampionName: f.identity.ChampionName,
	}, nil
}
func (f *fakeQuerier) GetLatestChampionInsightVersion(context.Context) (string, error) {
	return f.latest, f.latestErr
}
func (f *fakeQuerier) GetPublishedChampionInsightCohort(_ context.Context, p sqlcgen.GetPublishedChampionInsightCohortParams) (sqlcgen.GetPublishedChampionInsightCohortRow, error) {
	f.params = p
	return f.cohort, f.cohortErr
}
func (f *fakeQuerier) ListPublishedChampionInsightFactors(context.Context, int64) ([]sqlcgen.ListPublishedChampionInsightFactorsRow, error) {
	return f.rows, nil
}

func TestGetAssemblesPublishedObservedFactors(t *testing.T) {
	now := time.Now().UTC()
	q := &fakeQuerier{
		identity: sqlcgen.GetChampionIdentityRow{ChampionID: 421, ChampionName: "Rek'Sai"},
		latest:   "16.15",
		cohort: sqlcgen.GetPublishedChampionInsightCohortRow{
			CohortID: 7, ChampionID: 421, ChampionName: "Rek'Sai", TeamPosition: "JUNGLE",
			SampleGames: 1000, SamplePlayers: 200, Availability: "AVAILABLE",
			CohortScope: "CHAMPION_POSITION", Revision: "r1", AlgorithmVersion: "OBSERVED_PERCENTILES_V1",
			DataThrough: pgtype.Timestamptz{Time: now, Valid: true},
			PublishedAt: pgtype.Timestamptz{Time: now, Valid: true},
		},
		rows: []sqlcgen.ListPublishedChampionInsightFactorsRow{
			{MetricKey: "JUNGLE_CS_10", Kind: "BEHAVIOR_METRIC", EndMinute: 10, Unit: "COUNT", Direction: "OBSERVED_TREND", P50: 60, P70: 70, P90: 80, EvidenceGrade: "OBSERVED", DisplayOrder: 1, Ordinal: 1, LowerBound: 20, UpperBound: 59, Games: 500, Wins: 200, SamplePlayers: 120},
			{MetricKey: "JUNGLE_CS_10", Kind: "BEHAVIOR_METRIC", EndMinute: 10, Unit: "COUNT", Direction: "OBSERVED_TREND", P50: 60, P70: 70, P90: 80, EvidenceGrade: "OBSERVED", DisplayOrder: 1, Ordinal: 2, LowerBound: 60, UpperBound: 100, Games: 500, Wins: 300, SamplePlayers: 150},
		},
	}
	got, err := New(q).Get(context.Background(), 421, Filter{Version: "latest", Region: "kr", TierGroup: "master_plus", Position: "jungle"})
	require.NoError(t, err)
	require.Equal(t, "16.15", got.ResolvedVersion)
	require.Equal(t, "KR", q.params.RegionScope)
	require.Equal(t, "MASTER_PLUS", q.params.TierGroup)
	require.Len(t, got.Factors, 1)
	require.InDelta(t, -10, got.Factors[0].Buckets[0].ObservedWinRateDelta, 0.001)
	require.InDelta(t, 10, got.Factors[0].Buckets[1].ObservedWinRateDelta, 0.001)
	require.Equal(t, 120, got.Factors[0].Buckets[0].SamplePlayers)
}

func TestGetDistinguishesUnknownUnsupportedAndUnpublished(t *testing.T) {
	unknown, err := New(&fakeQuerier{identityErr: pgx.ErrNoRows}).Get(context.Background(), 999, Filter{Position: "JUNGLE"})
	require.NoError(t, err)
	require.Nil(t, unknown)

	q := &fakeQuerier{identity: sqlcgen.GetChampionIdentityRow{ChampionID: 1, ChampionName: "Annie"}, latest: "16.15"}
	unsupported, err := New(q).Get(context.Background(), 1, Filter{Position: "MIDDLE"})
	require.NoError(t, err)
	require.Equal(t, "UNSUPPORTED_POSITION", *unsupported.UnavailableReason)

	q.cohortErr = pgx.ErrNoRows
	unpublished, err := New(q).Get(context.Background(), 1, Filter{Version: "16.14", Position: "JUNGLE"})
	require.NoError(t, err)
	require.Equal(t, "NOT_PUBLISHED", *unpublished.UnavailableReason)
}

func TestGetRejectsMissingPosition(t *testing.T) {
	_, err := New(&fakeQuerier{}).Get(context.Background(), 1, Filter{})
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
	require.Equal(t, "position", validation.Field)
}

func TestGetRejectsInvalidExplicitVersion(t *testing.T) {
	q := &fakeQuerier{identity: sqlcgen.GetChampionIdentityRow{ChampionID: 1, ChampionName: "Annie"}}
	_, err := New(q).Get(context.Background(), 1, Filter{Version: "current", Position: "JUNGLE"})
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
	require.Equal(t, "version", validation.Field)
}
