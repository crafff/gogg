package rankings

import (
	"context"
	"errors"
	"reflect"
	"testing"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

type fakeQuerier struct {
	overall    []sqlcgen.ListOverallRankingsRow
	overallArg sqlcgen.ListOverallRankingsParams
	overallErr error

	byPos    []sqlcgen.ListRankingsByPositionRow
	byPosArg sqlcgen.ListRankingsByPositionParams
	byPosErr error
}

func (f *fakeQuerier) ListOverallRankings(_ context.Context, arg sqlcgen.ListOverallRankingsParams) ([]sqlcgen.ListOverallRankingsRow, error) {
	f.overallArg = arg
	return f.overall, f.overallErr
}
func (f *fakeQuerier) ListRankingsByPosition(_ context.Context, arg sqlcgen.ListRankingsByPositionParams) ([]sqlcgen.ListRankingsByPositionRow, error) {
	f.byPosArg = arg
	return f.byPos, f.byPosErr
}

type fakeVersions struct {
	latest    string
	latestErr error
}

func (f fakeVersions) GetLatestVersion(_ context.Context) (string, error) {
	return f.latest, f.latestErr
}

func TestTierGroupToAvgTiers(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"challenger", []string{"CHALLENGER"}},
		{"grandmaster_plus", []string{"GRANDMASTER", "CHALLENGER"}},
		{"grandmaster", []string{"GRANDMASTER"}},
		{"master_plus", []string{"MASTER", "GRANDMASTER", "CHALLENGER"}},
		{"master", []string{"MASTER"}},
		{"", []string{}},
		{"unrecognized", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := tierGroupToAvgTiers(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGetOverall_passesFilterToQuerier(t *testing.T) {
	q := &fakeQuerier{
		overall: []sqlcgen.ListOverallRankingsRow{
			{
				ChampionID: 99, ChampionName: "Lux",
				TeamPosition: []string{"MIDDLE", "BOTTOM"},
				Games:        100, Wins: 55, Losses: 45,
				WinRate: 55.0, PickRate: 12.3, BanRate: 8.1, Kda: 3.4,
				TotalMatches: 1000,
			},
		},
	}
	svc := New(q, fakeVersions{})

	res, err := svc.GetOverall(context.Background(), Filter{
		QueueID:           420,
		Version:           "15.1.1",
		Region:            "kr", // gets uppercased
		TierGroup:         "master_plus",
		MinGames:          20,
		Limit:             50,
		PositionThreshold: 5.0,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}

	// Filter must arrive at sqlc with: uppercase region, expanded tiers, ints converted.
	if q.overallArg.QueueID != 420 || q.overallArg.RegionFilter != "KR" {
		t.Errorf("queue/region passed wrong: %+v", q.overallArg)
	}
	if !reflect.DeepEqual([]string(q.overallArg.AvgTiers), []string{"MASTER", "GRANDMASTER", "CHALLENGER"}) {
		t.Errorf("avg_tiers = %v", q.overallArg.AvgTiers)
	}
	if q.overallArg.PositionThreshold != 5.0 {
		t.Errorf("position_threshold = %v", q.overallArg.PositionThreshold)
	}

	if len(res.Items) != 1 {
		t.Fatalf("len items = %d", len(res.Items))
	}
	got := res.Items[0]
	if got.ChampionID != 99 || got.ChampionName != "Lux" || got.WinRate != 55.0 {
		t.Errorf("row not propagated: %+v", got)
	}
	if !reflect.DeepEqual(got.TeamPosition, []string{"MIDDLE", "BOTTOM"}) {
		t.Errorf("positions = %v", got.TeamPosition)
	}
	if res.TotalMatches != 1000 {
		t.Errorf("total = %d", res.TotalMatches)
	}
	if res.ResolvedVersion != "" {
		t.Errorf("resolved version should be empty when caller passed an explicit version, got %q", res.ResolvedVersion)
	}
}

func TestGetOverall_resolvesLatestVersion(t *testing.T) {
	q := &fakeQuerier{}
	svc := New(q, fakeVersions{latest: "15.1.3"})

	res, err := svc.GetOverall(context.Background(), Filter{
		QueueID: 420,
		Version: "latest",
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if q.overallArg.VersionFilter != "15.1.3" {
		t.Errorf("version sent to sqlc = %q, want 15.1.3", q.overallArg.VersionFilter)
	}
	if res.ResolvedVersion != "15.1.3" {
		t.Errorf("ResolvedVersion = %q, want 15.1.3", res.ResolvedVersion)
	}
}

func TestGetOverall_latestResolutionError(t *testing.T) {
	svc := New(&fakeQuerier{}, fakeVersions{latestErr: errors.New("db down")})
	_, err := svc.GetOverall(context.Background(), Filter{Version: "latest"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetOverall_wrapsQuerierError(t *testing.T) {
	want := errors.New("boom")
	svc := New(&fakeQuerier{overallErr: want}, fakeVersions{})
	_, err := svc.GetOverall(context.Background(), Filter{})
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want errors.Is(_, want)", err)
	}
}

func TestGetByPosition_requiresPosition(t *testing.T) {
	svc := New(&fakeQuerier{}, fakeVersions{})
	_, err := svc.GetByPosition(context.Background(), Filter{Position: "  "})
	if err == nil {
		t.Fatal("expected error for empty position")
	}
}

func TestGetByPosition_setsPositionOnRows(t *testing.T) {
	q := &fakeQuerier{
		byPos: []sqlcgen.ListRankingsByPositionRow{
			{ChampionID: 1, ChampionName: "Annie", Games: 10, Wins: 6, Losses: 4, TotalMatches: 100},
		},
	}
	svc := New(q, fakeVersions{})
	res, err := svc.GetByPosition(context.Background(), Filter{Position: "middle"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if q.byPosArg.PositionFilter != "MIDDLE" {
		t.Errorf("position uppercased to %q, want MIDDLE", q.byPosArg.PositionFilter)
	}
	if got := res.Items[0].TeamPosition; len(got) != 1 || got[0] != "MIDDLE" {
		t.Errorf("TeamPosition = %v, want [MIDDLE]", got)
	}
}
