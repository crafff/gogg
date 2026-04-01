package crawler

import (
	"context"
	"fmt"

	"crawler-test/internal/riot"
)


func HandleMatchDetailByMatchID(ctx context.Context, payload interface{}, client *riot.Client) (interface{}, error) {
	p, ok := payload.(MatchDetailByMatchIDPayload)
	if !ok {
		return nil, fmt.Errorf("invalid payload for %s", TaskTypeMatchDetailByMatchID)
	}

	return client.GetMatchDetailByMatchID(ctx, p.MatchID)
}