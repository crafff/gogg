package process

import (
	"context"
	"time"
	"fmt"

	"crawler-test/internal/crawler"
	"crawler-test/internal/riot"
	"crawler-test/internal/storage"
)

func (p *ResultProcessor) processLeagueList(ctx context.Context, task crawler.Task, dto *riot.LeagueListDTO) error {
	fmt.Printf("处理挑战者榜结果: task_id=%s queue=%s tier=%s entries=%d\n", task.ID, dto.Queue, dto.Tier, len(dto.Entries))
	seeds := buildPlayerRankSeeds(task.ID, dto, time.Now().UTC())
	fmt.Printf("生成玩家排名种子: task_id=%s seeds=%d\n", task.ID, len(seeds))
	return p.store.UpsertPlayersAndRanks(ctx, seeds)
}

func buildPlayerRankSeeds(taskID string, dto *riot.LeagueListDTO, fetchedAt time.Time) []storage.PlayerRankSeed {
	if dto == nil || len(dto.Entries) == 0 {
		return nil
	}

	seeds := make([]storage.PlayerRankSeed, 0, len(dto.Entries))
	for _, e := range dto.Entries {
		if e.Puuid == "" {
			continue
		}

		seeds = append(seeds, storage.PlayerRankSeed{
			Puuid:        e.Puuid,
			Platform:     "na1",
			Region:       "americas",
			Queue:        dto.Queue,
			Tier:         dto.Tier,
			Rank:         e.Rank,
			LeaguePoints: e.LeaguePoints,
			Wins:         e.Wins,
			Losses:       e.Losses,
			Veteran:      e.Veteran,
			Inactive:     e.Inactive,
			FreshBlood:   e.FreshBlood,
			HotStreak:    e.HotStreak,
			SourceTaskID: taskID,
			FetchedAt:    fetchedAt,
		})
	}

	return seeds
}
