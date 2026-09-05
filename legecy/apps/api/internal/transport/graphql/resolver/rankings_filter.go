package resolver

import (
	"strings"

	"github.com/crafff/gogg/apps/api/internal/service/rankings"
	gqlgenerated "github.com/crafff/gogg/apps/api/internal/transport/graphql/generated"
)

// filterFromInput applies the same defaults and normalization as the REST
// handler. It lives outside the generated resolver file so gqlgen schema
// updates cannot discard it.
func filterFromInput(in *gqlgenerated.ChampionRankingsFilter) rankings.Filter {
	f := rankings.Filter{
		QueueID:           420,
		Version:           "latest",
		MinGames:          20,
		PositionThreshold: 5.0,
	}
	if in == nil {
		return f
	}
	if in.QueueID != nil {
		f.QueueID = *in.QueueID
	}
	if in.Version != nil {
		f.Version = strings.TrimSpace(*in.Version)
	}
	if in.Region != nil {
		f.Region = strings.ToUpper(strings.TrimSpace(*in.Region))
	}
	if in.TierGroup != nil && *in.TierGroup != gqlgenerated.TierGroupAll {
		f.TierGroup = strings.ToLower(string(*in.TierGroup))
	}
	if in.MinGames != nil {
		f.MinGames = *in.MinGames
	}
	if in.PositionThreshold != nil {
		f.PositionThreshold = *in.PositionThreshold
	}
	if in.Position != nil {
		f.Position = strings.ToUpper(strings.TrimSpace(*in.Position))
	}
	return f
}
