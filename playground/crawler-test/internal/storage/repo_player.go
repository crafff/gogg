package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type PlayerRankSeed struct {
	Puuid        string
	Platform     string
	Region       string
	Queue        string
	Tier         string
	Rank         string
	LeaguePoints int
	Wins         int
	Losses       int
	Veteran      bool
	Inactive     bool
	FreshBlood   bool
	HotStreak    bool
	SourceTaskID string
	FetchedAt    time.Time
}

func (s *Store) UpsertPlayersAndRanks(ctx context.Context, seeds []PlayerRankSeed) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}
	if len(seeds) == 0 {
		return nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	fmt.Printf("开始批量 upsert 玩家和排名: seeds=%d\n", len(seeds))
	for _, seed := range seeds {
		if seed.Puuid == "" {
			continue
		}

		var playerID int64
		err := tx.QueryRow(ctx, `
INSERT INTO players (puuid, platform, region, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (puuid) DO UPDATE
SET platform = CASE WHEN EXCLUDED.platform = '' THEN players.platform ELSE EXCLUDED.platform END,
    region = CASE WHEN EXCLUDED.region = '' THEN players.region ELSE EXCLUDED.region END,
    updated_at = NOW()
RETURNING id;
`, seed.Puuid, seed.Platform, seed.Region).Scan(&playerID)
		if err != nil {
			return fmt.Errorf("upsert player puuid %s: %w", seed.Puuid, err)
		}

		if _, err := tx.Exec(ctx, `
INSERT INTO player_rank_current (
  player_id, queue, tier, rank, league_points, wins, losses,
  veteran, inactive, fresh_blood, hot_streak,
  source_task_id, fetched_at, updated_at
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7,
  $8, $9, $10, $11,
  $12, $13, NOW()
)
ON CONFLICT (player_id, queue) DO UPDATE
SET tier = EXCLUDED.tier,
    rank = EXCLUDED.rank,
    league_points = EXCLUDED.league_points,
    wins = EXCLUDED.wins,
    losses = EXCLUDED.losses,
    veteran = EXCLUDED.veteran,
    inactive = EXCLUDED.inactive,
    fresh_blood = EXCLUDED.fresh_blood,
    hot_streak = EXCLUDED.hot_streak,
    source_task_id = EXCLUDED.source_task_id,
    fetched_at = EXCLUDED.fetched_at,
    updated_at = NOW();
`, playerID, seed.Queue, seed.Tier, seed.Rank, seed.LeaguePoints, seed.Wins, seed.Losses, seed.Veteran, seed.Inactive, seed.FreshBlood, seed.HotStreak, seed.SourceTaskID, seed.FetchedAt); err != nil {
			return fmt.Errorf("upsert rank for puuid %s: %w", seed.Puuid, err)
		}
	}

	fmt.Printf("批量 upsert 玩家和排名完成: seeds=%d\n", len(seeds))
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
