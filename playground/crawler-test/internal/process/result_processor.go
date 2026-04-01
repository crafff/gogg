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
	case crawler.TaskTypeVersion:
		dto, ok := result.(*riot.VersionResponse)
		if !ok {
			return fmt.Errorf("unexpected result type for %s", task.Type)
		}
		return p.processVersions(ctx, task, dto)
	case crawler.TaskTypeMatchByPUUID:
		matches, ok := result.(*riot.Matchs)
		if !ok {
			return fmt.Errorf("unexpected result type for %s", task.Type)
		}
		return p.processMatchByPuuid(ctx, task, *matches)
	case crawler.TaskTypeMatchDetailByMatchID:
		dto, ok := result.(*riot.MatchDetailDTO)
		if !ok {
			return fmt.Errorf("unexpected result type for %s", task.Type)
		}
		return p.processMatchDetail(ctx, task, dto)
	default:
		return nil
	}
}
