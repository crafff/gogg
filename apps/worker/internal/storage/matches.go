package storage

import (
	"context"
	"time"
)

// UpsertMatchID inserts a match_id with fetch_status=pending.
// If it already exists, it's a no-op.
func (s *Store) UpsertMatchID(ctx context.Context, matchID, region, version string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO matches (match_id, region, version) VALUES ($1, $2, $3)
		ON CONFLICT (match_id) DO NOTHING`, matchID, region, version)
	return err
}

// CountPendingMatchIDs returns the total number of pending matches for the region and version.
func (s *Store) CountPendingMatchIDs(ctx context.Context, region, version string) (int, error) {
	var count int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM matches
		WHERE fetch_status = 'pending' AND region = $1 AND version = $2`,
		region, version).Scan(&count)
	return count, err
}

// GetPendingMatchIDs returns match_ids that still need details fetched for the region and version.
func (s *Store) GetPendingMatchIDs(ctx context.Context, region, version string, limit int) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT match_id FROM matches
		WHERE fetch_status = 'pending' AND region = $1 AND version = $2
		ORDER BY created_at
		LIMIT $3`, region, version, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type MatchHeader struct {
	MatchID         string
	DataVersion     *string
	PlatformID      *string
	QueueID         *int
	GameVersion     *string
	GameMode        *string
	GameType        *string
	GameStartTS     *time.Time
	GameEndTS       *time.Time
	GameDuration    *int
	EndOfGameResult *string
}

// UpdateMatchDetail writes the header fields and marks fetch_status=done.
func (s *Store) UpdateMatchDetail(ctx context.Context, h *MatchHeader) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE matches SET
			data_version       = $2,
			platform_id        = $3,
			queue_id           = $4,
			game_version       = $5,
			game_mode          = $6,
			game_type          = $7,
			game_start_ts      = $8,
			game_end_ts        = $9,
			game_duration      = $10,
			end_of_game_result = $11,
			fetch_status       = 'done'
		WHERE match_id = $1`,
		h.MatchID, h.DataVersion, h.PlatformID, h.QueueID,
		h.GameVersion, h.GameMode, h.GameType,
		h.GameStartTS, h.GameEndTS, h.GameDuration, h.EndOfGameResult,
	)
	return err
}

const maxMatchRetries = 3

// IncrementMatchRetry bumps retry_count. Status stays 'pending' until
// maxMatchRetries is reached, then becomes 'error'.
func (s *Store) IncrementMatchRetry(ctx context.Context, matchID string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE matches
		SET retry_count  = retry_count + 1,
		    fetch_status = CASE WHEN retry_count + 1 >= $2 THEN 'error' ELSE 'pending' END
		WHERE match_id = $1`, matchID, maxMatchRetries)
	return err
}

// GetMatchesNeedingTierCalc returns match_ids where avg_tier_score is null
// and fetch_status=done for the given region.
func (s *Store) GetMatchesNeedingTierCalc(ctx context.Context, region string, limit int) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT match_id FROM matches
		WHERE fetch_status = 'done' AND avg_tier_score IS NULL AND region = $1
		ORDER BY game_start_ts
		LIMIT $2`, region, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UpdateAvgTier writes the computed avg_tier_score, avg_tier, avg_division and coverage for a match.
func (s *Store) UpdateAvgTier(ctx context.Context, matchID, avgTier, avgDivision string, avgScore, coverage int) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE matches
		SET avg_tier_score = $2, tier_coverage = $3, avg_tier = $4, avg_division = $5
		WHERE match_id = $1`, matchID, avgScore, coverage, avgTier, avgDivision)
	return err
}

// ApexThresholds holds the minimum LP score for Challenger and Grandmaster
// players in a given run, used to classify matches above Master.
type ApexThresholds struct {
	ChallengerMinScore  int // 2800 + min LP of Challenger players
	GrandmasterMinScore int // 2800 + min LP of Grandmaster players
}

// GetApexThresholds queries the minimum LP for Challenger and Grandmaster
// players from phase1 snapshots of the given run.
func (s *Store) GetApexThresholds(ctx context.Context, runID int, region string) (ApexThresholds, error) {
	const masterBase = 2800
	const noThreshold = 1<<31 - 1 // effectively unreachable

	var challengerMin, grandmasterMin *int
	err := s.Pool.QueryRow(ctx, `
		SELECT
		    MIN(CASE WHEN UPPER(tier) = 'CHALLENGER'   THEN COALESCE(league_points, 0) END),
		    MIN(CASE WHEN UPPER(tier) = 'GRANDMASTER'  THEN COALESCE(league_points, 0) END)
		FROM player_rank_snapshots
		WHERE run_id = $1 AND region = $2 AND source = 'phase1'
		  AND UPPER(tier) IN ('CHALLENGER', 'GRANDMASTER')`,
		runID, region,
	).Scan(&challengerMin, &grandmasterMin)
	if err != nil {
		return ApexThresholds{}, err
	}

	t := ApexThresholds{
		ChallengerMinScore:  noThreshold,
		GrandmasterMinScore: noThreshold,
	}
	if challengerMin != nil {
		t.ChallengerMinScore = masterBase + *challengerMin
	}
	if grandmasterMin != nil {
		t.GrandmasterMinScore = masterBase + *grandmasterMin
	}
	return t, nil
}
