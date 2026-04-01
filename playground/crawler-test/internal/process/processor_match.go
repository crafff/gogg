package process

import (
	"context"
	"fmt"

	"crawler-test/internal/crawler"
	"crawler-test/internal/storage"
)

func (p *ResultProcessor) processMatchByPuuid(ctx context.Context, task crawler.Task, matches []string) error {
	fmt.Printf("处理比赛列表结果: task_id=%s total_matches=%d\n", task.ID, len(matches))
	seeds := buildMatchSeeds(task.ID, matches)
	fmt.Printf("生成比赛种子: task_id=%s seeds=%d\n", task.ID, len(seeds))
	return p.store.UpsertMatchIDs(ctx, seeds)
}

func buildMatchSeeds(taskID string, matches []string) []storage.MatchIdSeed {
	if matches == nil || len(matches) == 0 {
		return nil
	}

	seeds := make([]storage.MatchIdSeed, 0, len(matches))
	for _, m := range matches {
		if m == "" {
			continue
		}

		seeds = append(seeds, storage.MatchIdSeed{
			MatchID: m,
		})
	}

	return seeds
}
			