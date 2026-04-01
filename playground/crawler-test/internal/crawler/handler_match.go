package crawler

import (
	"context"
	"fmt"

	"crawler-test/internal/riot"
)

func HandleMatchByPuuid(ctx context.Context, payload interface{}, client *riot.Client) (interface{}, error) {
	p, ok := payload.(MatchByPUUIDPayload)
	if !ok {
		return nil, fmt.Errorf("invalid payload for %s", TaskTypeMatchByPUUID)
	}

	return client.GetMatchsByPuuid(ctx, p.Puuid, p.StartTime, p.EndTime, p.MatchType, p.Start, p.Count)
}
