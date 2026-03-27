package crawler

import (
	"context"
	"fmt"

	"crawler-test/internal/riot"
)

func HandleChallengeLeaguesByQueue(ctx context.Context, payload interface{}, client *riot.Client) (interface{}, error) {
	p, ok := payload.(ChallengeLeaguesByQueuePayload)
	if !ok {
		return nil, fmt.Errorf("invalid payload for %s", TaskTypeChallengeLeaguesByQueue)
	}

	return client.GetChallengerLeaguesByQueue(ctx, p.Queue)
}
