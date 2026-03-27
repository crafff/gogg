package process

import (
	"context"
	"fmt"

	"crawler-test/internal/crawler"
	"crawler-test/internal/riot"
	"crawler-test/internal/storage"
)

type ResultProcessor struct {
	store *storage.Store
}

func NewResultProcessor(store *storage.Store) *ResultProcessor {
	return &ResultProcessor{store: store}
}

func (p *ResultProcessor) Process(ctx context.Context, task crawler.Task, result interface{}) error {
	if p == nil || p.store == nil {
		return nil
	}

	switch task.Type {
	case crawler.TaskTypeChallengeLeaguesByQueue:
		dto, ok := result.(*riot.LeagueListDTO)
		if !ok {
			return fmt.Errorf("unexpected result type for %s", task.Type)
		}
		return p.processLeagueList(ctx, task, dto)
	default:
		return nil
	}
}
