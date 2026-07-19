package resolver

import (
	"context"
	"errors"
	"testing"

	"github.com/crafff/gogg/apps/api/internal/service/rankings"
	"github.com/crafff/gogg/apps/api/internal/transport/graphql/domainerr"
	gqlgenerated "github.com/crafff/gogg/apps/api/internal/transport/graphql/generated"
)

func TestFilterFromInput_NilUsesDefaults(t *testing.T) {
	f := filterFromInput(nil)
	want := rankings.Filter{
		QueueID:           420,
		Version:           "latest",
		MinGames:          20,
		PositionThreshold: 5.0,
	}
	if f != want {
		t.Errorf("nil input: got %+v want %+v", f, want)
	}
}

func TestChampionRankings_InvalidFilterReturnsBadUserInput(t *testing.T) {
	region := "EUW1"
	_, err := (&queryResolver{Resolver: &Resolver{}}).ChampionRankings(
		context.Background(),
		&gqlgenerated.ChampionRankingsFilter{Region: &region},
	)
	if err == nil {
		t.Fatal("invalid filter accepted")
	}
	var domainError *domainerr.Error
	if !errors.As(err, &domainError) {
		t.Fatalf("error type = %T, want *domainerr.Error", err)
	}
	if domainError.Code != "BAD_USER_INPUT" {
		t.Fatalf("code = %q, want BAD_USER_INPUT", domainError.Code)
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

func TestFilterFromInput_PreservesInvalidValuesForValidation(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*gqlgenerated.ChampionRankingsFilter)
		want rankings.Filter
	}{
		{
			name: "queueId high",
			mut: func(in *gqlgenerated.ChampionRankingsFilter) {
				v := 99999
				in.QueueID = &v
			},
			want: rankings.Filter{QueueID: 99999, Version: "latest", MinGames: 20, PositionThreshold: 5.0},
		},
		{
			name: "minGames low",
			mut: func(in *gqlgenerated.ChampionRankingsFilter) {
				v := 0
				in.MinGames = &v
			},
			want: rankings.Filter{QueueID: 420, Version: "latest", MinGames: 0, PositionThreshold: 5.0},
		},
		{
			name: "positionThreshold low",
			mut: func(in *gqlgenerated.ChampionRankingsFilter) {
				v := -1.0
				in.PositionThreshold = &v
			},
			want: rankings.Filter{QueueID: 420, Version: "latest", MinGames: 20, PositionThreshold: -1},
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
