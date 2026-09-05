package resolver

import (
	"github.com/crafff/gogg/apps/api/internal/service/champion"
	gqlgenerated "github.com/crafff/gogg/apps/api/internal/transport/graphql/generated"
)

func mapBuilds(builds []champion.Build) []*gqlgenerated.BuildChoice {
	out := make([]*gqlgenerated.BuildChoice, 0, len(builds))
	for _, b := range builds {
		out = append(out, &gqlgenerated.BuildChoice{Ids: b.IDs, Games: b.Games, EligibleGames: b.EligibleGames, PickRate: b.PickRate, WinRate: b.WinRate})
	}
	return out
}
