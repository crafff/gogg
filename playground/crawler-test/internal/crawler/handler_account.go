package crawler

import (
	"context"
	"fmt"

	"crawler-test/internal/riot"
)

func HandleAccountByRiotID(ctx context.Context, payload interface{}, client *riot.Client) (interface{}, error) {
	p, ok := payload.(AccountByRiotIDPayload)
	if !ok {
		return nil, fmt.Errorf("invalid payload for %s", TaskTypeAccountByRiotID)
	}

	return client.GetAccountByRiotID(ctx, p.GameName, p.TagLine)
}
