package crawler

import (
	"context"
	"fmt"

	"crawler-test/internal/riot"
)

func HandleVersions(ctx context.Context, payload interface{}, client *riot.Client) (interface{}, error) {
	_, ok := payload.(VersionPayload)
	if !ok {
		return nil, fmt.Errorf("invalid payload for %s", TaskTypeVersion)
	}

	return client.GetVersions(ctx)
}