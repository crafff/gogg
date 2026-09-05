package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type PerkBackfillMatch struct {
	MatchID string
	Region  string
}

func (s *Store) CountMatchesNeedingPerkBackfill(ctx context.Context, region string) (int64, error) {
	var count int64
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT mp.match_id)
		FROM match_perks mp
		JOIN matches m ON m.match_id=mp.match_id
		WHERE m.fetch_status='done'
		  AND (mp.perk5 IS NULL OR mp.perk5=0)
		  AND ($1='' OR m.region=$1)`, region).Scan(&count)
	return count, err
}

// ListMatchesNeedingPerkBackfill uses a match_id keyset so one invocation
// visits a failing match at most once. A later invocation naturally retries
// failures while already repaired matches disappear from the predicate.
func (s *Store) ListMatchesNeedingPerkBackfill(ctx context.Context, region, after string, limit int) ([]PerkBackfillMatch, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT mp.match_id,MAX(m.region)::text AS region
		FROM match_perks mp
		JOIN matches m ON m.match_id=mp.match_id
		WHERE m.fetch_status='done'
		  AND (mp.perk5 IS NULL OR mp.perk5=0)
		  AND ($1='' OR m.region=$1)
		  AND mp.match_id>$2
		GROUP BY mp.match_id
		ORDER BY mp.match_id
		LIMIT $3`, region, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PerkBackfillMatch, 0, limit)
	for rows.Next() {
		var row PerkBackfillMatch
		if err := rows.Scan(&row.MatchID, &row.Region); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ReplaceMatchPerks changes only match_perks. Other match facts and status
// columns are deliberately untouched by the repair operation.
func (s *Store) ReplaceMatchPerks(ctx context.Context, matchID string, perks []PerkRow) error {
	if len(perks) != 10 {
		return fmt.Errorf("replace match perks %s: got %d participants, want 10", matchID, len(perks))
	}
	return s.WithTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM match_perks WHERE match_id=$1`, matchID); err != nil {
			return err
		}
		if err := insertPerksTx(ctx, tx, perks); err != nil {
			return err
		}
		var written int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM match_perks WHERE match_id=$1 AND perk5>0`, matchID).Scan(&written); err != nil {
			return err
		}
		if written != 10 {
			return fmt.Errorf("replace match perks %s: wrote %d complete rows, want 10", matchID, written)
		}
		return nil
	})
}
