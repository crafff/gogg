package champion

import (
	"context"
	"testing"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type fakeQuerier struct {
	identity    sqlcgen.GetChampionIdentityRow
	identityErr error
	rows        []sqlcgen.ListChampionDetailBuildsRow
	params      sqlcgen.ListChampionDetailBuildsParams
}

func (f *fakeQuerier) GetChampionIdentity(context.Context, int32) (sqlcgen.GetChampionIdentityRow, error) {
	return f.identity, f.identityErr
}
func (f *fakeQuerier) ListChampionDetailBuilds(_ context.Context, p sqlcgen.ListChampionDetailBuildsParams) ([]sqlcgen.ListChampionDetailBuildsRow, error) {
	f.params = p
	return f.rows, nil
}

type fakeVersions struct{}

func (fakeVersions) GetLatestVersion(context.Context) (string, error) { return "16.14", nil }

func TestGetMapsBuildsAndResolvesFilters(t *testing.T) {
	q := &fakeQuerier{identity: sqlcgen.GetChampionIdentityRow{ChampionID: 1, ChampionName: "Annie"}, rows: []sqlcgen.ListChampionDetailBuildsRow{
		{Category: "RUNES", Signature: []int32{8000, 8300, 8005, 9111, 9104, 8014, 8347, 8304, 5005, 5008, 5001}, Games: 40, Wins: 22, EligibleGames: 100},
		{Category: "ITEMS", Stage: 3, Signature: []int32{1, 2, 3}, Games: 25, Wins: 15, EligibleGames: 50},
	}}
	got, err := New(q, fakeVersions{}).Get(context.Background(), 1, Filter{QueueID: 420, Version: "latest", Region: "kr", TierGroup: "master_plus", Position: "middle"})
	require.NoError(t, err)
	require.Equal(t, "16.14", got.ResolvedVersion)
	require.Equal(t, []int{8005, 9111, 9104, 8014, 8347, 8304}, got.RuneBuilds[0].PerkIDs)
	require.Equal(t, []int{5005, 5008, 5001}, got.RuneBuilds[0].StatShardIDs)
	require.InDelta(t, 40, got.RuneBuilds[0].PickRate, 0.001)
	require.InDelta(t, 60, got.ItemBuilds[0].Builds[0].WinRate, 0.001)
	require.Equal(t, "KR", q.params.RegionFilter)
	require.Equal(t, "MIDDLE", q.params.PositionFilter)
	require.Equal(t, []string{"MASTER", "GRANDMASTER", "CHALLENGER"}, q.params.AvgTiers)
}

func TestGetReturnsNilForUnknownChampion(t *testing.T) {
	got, err := New(&fakeQuerier{identityErr: pgx.ErrNoRows}, fakeVersions{}).Get(context.Background(), 999, Filter{})
	require.NoError(t, err)
	require.Nil(t, got)
}
