package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type ActiveVersion struct {
	ID      int64
	Version string
	StartAt time.Time
}

type PlayerRecord struct {
	ID    int64
	Puuid string
}

type PlayerMatchSyncState struct {
	PlayerID      int64
	VersionID     int64
	LastCheckedAt *time.Time
	LastMatchTime *time.Time
	Cursor        *string
}

type PlayerMatchSyncStateSeed struct {
	PlayerID      int64
	VersionID     int64
	LastCheckedAt *time.Time
	LastMatchTime *time.Time
	Cursor        *string
}

func (s *Store) GetLatestActiveVersion(ctx context.Context) (*ActiveVersion, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("store is not initialized")
	}

	var v ActiveVersion
	err := s.pool.QueryRow(ctx, `
SELECT id, version, start_at
FROM game_versions
WHERE is_active = TRUE
ORDER BY start_at DESC NULLS LAST, id DESC
LIMIT 1;
`).Scan(&v.ID, &v.Version, &v.StartAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("no active version found")
		}
		return nil, fmt.Errorf("query active version: %w", err)
	}

	return &v, nil
}

func (s *Store) ListPlayers(ctx context.Context) ([]PlayerRecord, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("store is not initialized")
	}

	rows, err := s.pool.Query(ctx, `
SELECT id, puuid
FROM players
WHERE puuid <> ''
ORDER BY id ASC;
`)
	if err != nil {
		return nil, fmt.Errorf("query players: %w", err)
	}
	defer rows.Close()

	players := make([]PlayerRecord, 0)
	for rows.Next() {
		var p PlayerRecord
		if err := rows.Scan(&p.ID, &p.Puuid); err != nil {
			return nil, fmt.Errorf("scan player: %w", err)
		}
		players = append(players, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate players: %w", err)
	}

	return players, nil
}

func (s *Store) GetPlayerMatchSyncState(ctx context.Context, playerID, versionID int64) (*PlayerMatchSyncState, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("store is not initialized")
	}

	var state PlayerMatchSyncState
	err := s.pool.QueryRow(ctx, `
SELECT player_id, version_id, last_checked_at, last_match_time, cursor
FROM player_match_sync_state
WHERE player_id = $1 AND version_id = $2;
`, playerID, versionID).Scan(
		&state.PlayerID,
		&state.VersionID,
		&state.LastCheckedAt,
		&state.LastMatchTime,
		&state.Cursor,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query sync state player_id=%d version_id=%d: %w", playerID, versionID, err)
	}

	return &state, nil
}

func (s *Store) UpsertPlayerMatchSyncState(ctx context.Context, seed PlayerMatchSyncStateSeed) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}
	if seed.PlayerID == 0 || seed.VersionID == 0 {
		return fmt.Errorf("player_id and version_id are required")
	}

	_, err := s.pool.Exec(ctx, `
INSERT INTO player_match_sync_state (
	player_id,
	version_id,
	last_checked_at,
	last_match_time,
	cursor,
	updated_at
)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (player_id, version_id) DO UPDATE
SET last_checked_at = EXCLUDED.last_checked_at,
	last_match_time = CASE
		WHEN EXCLUDED.last_match_time IS NULL THEN player_match_sync_state.last_match_time
		ELSE EXCLUDED.last_match_time
	END,
	cursor = EXCLUDED.cursor,
	updated_at = NOW();
`, seed.PlayerID, seed.VersionID, seed.LastCheckedAt, seed.LastMatchTime, seed.Cursor)
	if err != nil {
		return fmt.Errorf("upsert sync state player_id=%d version_id=%d: %w", seed.PlayerID, seed.VersionID, err)
	}

	return nil
}

func (s *Store) ListPendingMatchIDs(ctx context.Context, limit int) ([]string, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("store is not initialized")
	}

	query := `
SELECT match_id
FROM matches
WHERE processed_at IS NULL
ORDER BY created_at ASC;
`
	args := []interface{}{}
	if limit > 0 {
		query = `
SELECT match_id
FROM matches
WHERE processed_at IS NULL
ORDER BY created_at ASC
LIMIT $1;
`
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pending matches: %w", err)
	}
	defer rows.Close()

	matchIDs := make([]string, 0)
	for rows.Next() {
		var matchID string
		if err := rows.Scan(&matchID); err != nil {
			return nil, fmt.Errorf("scan pending match_id: %w", err)
		}
		matchIDs = append(matchIDs, matchID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending matches: %w", err)
	}

	return matchIDs, nil
}
