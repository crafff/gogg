package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// UpsertSnapshot writes a rank snapshot row. Retries for the same
// run/player/rank scope update the existing row instead of duplicating
// snapshots; top tiers use division "I" for a non-null key.
func (s *Store) UpsertSnapshot(ctx context.Context, snap *RankSnapshot) error {
	return upsertSnapshotTx(ctx, s.Pool, snap)
}

type execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func upsertSnapshotTx(ctx context.Context, exec execer, snap *RankSnapshot) error {
	division := "I"
	if snap.Division != nil && *snap.Division != "" {
		division = *snap.Division
	}
	_, err := exec.Exec(ctx, `
		INSERT INTO player_rank_snapshots
		  (run_id, puuid, region, source, league_id, queue, tier, division,
		   league_points, wins, losses, veteran, inactive, fresh_blood,
		   hot_streak, rank_status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		ON CONFLICT (run_id, puuid, region, source, queue, tier, division)
		DO UPDATE SET
			league_id = EXCLUDED.league_id,
			league_points = EXCLUDED.league_points,
			wins = EXCLUDED.wins,
			losses = EXCLUDED.losses,
			veteran = EXCLUDED.veteran,
			inactive = EXCLUDED.inactive,
			fresh_blood = EXCLUDED.fresh_blood,
			hot_streak = EXCLUDED.hot_streak,
			rank_status = EXCLUDED.rank_status,
			created_at = now()`,
		snap.RunID, snap.PUUID, snap.Region, snap.Source, snap.LeagueID, snap.Queue,
		snap.Tier, division, snap.LeaguePoints, snap.Wins, snap.Losses,
		snap.Veteran, snap.Inactive, snap.FreshBlood, snap.HotStreak, snap.RankStatus,
	)
	return err
}

// SaveRankBackfill atomically records an on-demand snapshot and applies it to
// all pending participant rows for the PUUID.
func (s *Store) SaveRankBackfill(ctx context.Context, snap *RankSnapshot, tier, division string, lp int) error {
	return s.WithTx(ctx, func(tx pgx.Tx) error {
		if err := upsertSnapshotTx(ctx, tx, snap); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE match_participants mp
			SET tier_at_match        = $1,
			    division_at_match    = $2,
			    lp_at_match          = $3,
			    tier_snapshot_delta_h = ROUND(ABS(EXTRACT(EPOCH FROM (NOW() - m.game_start_ts)) / 3600))::int
			FROM matches m
			WHERE mp.match_id = m.match_id
			  AND mp.puuid = $4
			  AND mp.tier_at_match IS NULL`,
			tier, division, lp, snap.PUUID)
		return err
	})
}

// InsertSnapshot is kept for older call sites; new code should call
// UpsertSnapshot so retry semantics are explicit.
func (s *Store) InsertSnapshot(ctx context.Context, snap *RankSnapshot) error {
	return s.UpsertSnapshot(ctx, snap)
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
