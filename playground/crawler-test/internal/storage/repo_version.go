package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type VersionSeed struct {
	Version      string
	StartTime	time.Time
}

func (s *Store) UpsertVersions(ctx context.Context, seeds []VersionSeed) error {
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
	
	fmt.Printf("开始批量 upsert 版本信息: seeds=%d\n", len(seeds))
	for _, seed := range seeds {
		if seed.Version == "" {
			continue
		}
		
		_, err := tx.Exec(ctx, `
INSERT INTO game_versions (version, start_at, updated_at)
VALUES ($1, $2, NOW())
ON CONFLICT (version) DO UPDATE
SET start_at = EXCLUDED.start_at,
	updated_at = NOW();
`, seed.Version, seed.StartTime)
		if err != nil {
			return fmt.Errorf("upsert version %s: %w", seed.Version, err)
		}
	}

	// 把时间最新的版本的is_active设置为true，其他的设置为false
	_, err = tx.Exec(ctx, `
WITH latest AS (
	SELECT version FROM game_versions
	ORDER BY start_at DESC
	LIMIT 1
)
UPDATE game_versions
SET is_active = (version = (SELECT version FROM latest)),
	updated_at = NOW();
`)	
	if err != nil {
		return fmt.Errorf("update active version: %w", err)
	}
	
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	
	return nil
}

