package storage

import (
	"context"
	"time"
)

type RankSnapshot struct {
	ID           int64
	RunID        *int
	PUUID        string
	Region       string
	Source       string
	LeagueID     *string
	Queue        string
	Tier         string
	Division     *string
	LeaguePoints *int
	Wins         *int
	Losses       *int
	Veteran      *bool
	Inactive     *bool
	FreshBlood   *bool
	HotStreak    *bool
	RankStatus   string
	CreatedAt    time.Time
}

// InsertSnapshot writes a new rank snapshot row.
func (s *Store) InsertSnapshot(ctx context.Context, snap *RankSnapshot) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO player_rank_snapshots
		  (run_id, puuid, region, source, league_id, queue, tier, division,
		   league_points, wins, losses, veteran, inactive, fresh_blood,
		   hot_streak, rank_status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		snap.RunID, snap.PUUID, snap.Region, snap.Source, snap.LeagueID, snap.Queue,
		snap.Tier, snap.Division, snap.LeaguePoints, snap.Wins, snap.Losses,
		snap.Veteran, snap.Inactive, snap.FreshBlood, snap.HotStreak, snap.RankStatus,
	)
	return err
}

// GetLatestSnapshot returns the most recent snapshot for a player in a region.
func (s *Store) GetLatestSnapshot(ctx context.Context, puuid, region string) (*RankSnapshot, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, run_id, puuid, region, source, league_id, queue, tier, division,
		       league_points, wins, losses, veteran, inactive, fresh_blood,
		       hot_streak, rank_status, created_at
		FROM player_rank_snapshots
		WHERE puuid = $1 AND region = $2
		ORDER BY created_at DESC
		LIMIT 1`, puuid, region)

	var snap RankSnapshot
	err := row.Scan(
		&snap.ID, &snap.RunID, &snap.PUUID, &snap.Region, &snap.Source, &snap.LeagueID,
		&snap.Queue, &snap.Tier, &snap.Division, &snap.LeaguePoints,
		&snap.Wins, &snap.Losses, &snap.Veteran, &snap.Inactive,
		&snap.FreshBlood, &snap.HotStreak, &snap.RankStatus, &snap.CreatedAt,
	)
	if err != nil {
		return nil, nil
	}
	return &snap, nil
}

// GetClosestSnapshot returns the snapshot whose created_at is closest to
// refTime for the given player in the given region.
func (s *Store) GetClosestSnapshot(ctx context.Context, puuid, region string, refTime time.Time) (*RankSnapshot, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, run_id, puuid, region, source, league_id, queue, tier, division,
		       league_points, wins, losses, veteran, inactive, fresh_blood,
		       hot_streak, rank_status, created_at
		FROM player_rank_snapshots
		WHERE puuid = $1 AND region = $2
		ORDER BY ABS(EXTRACT(EPOCH FROM (created_at - $3)))
		LIMIT 1`, puuid, region, refTime)

	var snap RankSnapshot
	err := row.Scan(
		&snap.ID, &snap.RunID, &snap.PUUID, &snap.Region, &snap.Source, &snap.LeagueID,
		&snap.Queue, &snap.Tier, &snap.Division, &snap.LeaguePoints,
		&snap.Wins, &snap.Losses, &snap.Veteran, &snap.Inactive,
		&snap.FreshBlood, &snap.HotStreak, &snap.RankStatus, &snap.CreatedAt,
	)
	if err != nil {
		return nil, nil
	}
	return &snap, nil
}

// GetActivePUUIDs returns all puuids that had status='active' in the given run.
func (s *Store) GetActivePUUIDs(ctx context.Context, runID int) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT puuid FROM player_rank_snapshots
		WHERE run_id = $1 AND source = 'phase1' AND rank_status = 'active'`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var puuids []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		puuids = append(puuids, p)
	}
	return puuids, rows.Err()
}

// GetActivePUUIDsByTiers returns puuids active in the given run filtered by tier.
func (s *Store) GetActivePUUIDsByTiers(ctx context.Context, runID int, tiers []string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT puuid FROM player_rank_snapshots
		WHERE run_id = $1 AND source = 'phase1' AND rank_status = 'active'
		  AND UPPER(tier) = ANY($2)`,
		runID, tiers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var puuids []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		puuids = append(puuids, p)
	}
	return puuids, rows.Err()
}
