package crawl

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
)

type Phase6Output struct {
	SourceMatches int64 `json:"source_matches"`
	AggregateRows int64 `json:"aggregate_rows"`
}

func (a *Activities) Phase6ChampionDetailRollup(ctx context.Context) (Phase6Output, error) {
	activity.RecordHeartbeat(ctx, "champion_detail_rollup_starting")
	result, err := a.rt.Store.RebuildChampionDetailRollups(ctx)
	if err != nil {
		return Phase6Output{}, fmt.Errorf("phase6 champion detail rollup: %w", err)
	}
	return Phase6Output{SourceMatches: result.SourceMatches, AggregateRows: result.AggregateRows}, nil
}
