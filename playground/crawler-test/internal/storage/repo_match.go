package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)


type MatchIdSeed struct {
	MatchID      string
}

func (s *Store) UpsertMatchIDs(ctx context.Context, seeds []MatchIdSeed) error {
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
	
	fmt.Printf("开始批量 upsert 比赛ID: seeds=%d\n", len(seeds))
	for _, seed := range seeds {
		if seed.MatchID == "" {
			continue
		}
		
		_, err := tx.Exec(ctx, `
INSERT INTO matches (match_id, created_at)
VALUES ($1, NOW())
ON CONFLICT (match_id) DO NOTHING;
`, seed.MatchID)
		if err != nil {
			return fmt.Errorf("upsert match_id %s: %w", seed.MatchID, err)
		}

	}
	
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	
	return nil
}
