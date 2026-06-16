package resolver

import (
	"testing"

	"github.com/crafff/gogg/apps/api/internal/service/rankings"
	gqlgenerated "github.com/crafff/gogg/apps/api/internal/transport/graphql/generated"
)

func TestFilterFromInput_NilUsesDefaults(t *testing.T) {
	f := filterFromInput(nil)
	want := rankings.Filter{
		QueueID:           420,
		Version:           "latest",
		MinGames:          20,
		Limit:             -1,
		PositionThreshold: 5.0,
	}
	if f != want {
		t.Errorf("nil input: got %+v want %+v", f, want)
	}
}

func TestFilterFromInput_NormalisesCase(t *testing.T) {
	region := "kr"
	pos := "middle"
	in := &gqlgenerated.ChampionRankingsFilter{Region: &region, Position: &pos}
	f := filterFromInput(in)
	if f.Region != "KR" {
		t.Errorf("region: got %q want KR", f.Region)
	}
	if f.Position != "MIDDLE" {
		t.Errorf("position: got %q want MIDDLE", f.Position)
	}
}

func TestFilterFromInput_TierGroupAllErases(t *testing.T) {
	all := gqlgenerated.TierGroupAll
	masterPlus := gqlgenerated.TierGroupMasterPlus
	cases := []struct {
		name  string
		input *gqlgenerated.TierGroup
		want  string
	}{
		{"nil → empty", nil, ""},
		{"ALL → empty (treated as no filter)", &all, ""},
		{"MASTER_PLUS → lowercase", &masterPlus, "master_plus"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := &gqlgenerated.ChampionRankingsFilter{TierGroup: tc.input}
			if got := filterFromInput(in).TierGroup; got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestFilterFromInput_Clamps(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*gqlgenerated.ChampionRankingsFilter)
		want rankings.Filter
	}{
		{
			name: "queueId clamps high",
			mut: func(in *gqlgenerated.ChampionRankingsFilter) {
				v := 99999
				in.QueueID = &v
			},
			want: rankings.Filter{QueueID: 9999, Version: "latest", MinGames: 20, Limit: -1, PositionThreshold: 5.0},
		},
		{
			name: "minGames clamps low",
			mut: func(in *gqlgenerated.ChampionRankingsFilter) {
				v := 0
				in.MinGames = &v
			},
			want: rankings.Filter{QueueID: 420, Version: "latest", MinGames: 1, Limit: -1, PositionThreshold: 5.0},
		},
		{
			name: "limit clamps high",
			mut: func(in *gqlgenerated.ChampionRankingsFilter) {
				v := 10000
				in.Limit = &v
			},
			want: rankings.Filter{QueueID: 420, Version: "latest", MinGames: 20, Limit: 500, PositionThreshold: 5.0},
		},
		{
			name: "positionThreshold clamps low",
			mut: func(in *gqlgenerated.ChampionRankingsFilter) {
				v := -1.0
				in.PositionThreshold = &v
			},
			want: rankings.Filter{QueueID: 420, Version: "latest", MinGames: 20, Limit: -1, PositionThreshold: 0},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := &gqlgenerated.ChampionRankingsFilter{}
			tc.mut(in)
			got := filterFromInput(in)
			if got != tc.want {
				t.Errorf("got %+v want %+v", got, tc.want)
			}
		})
	}
}
