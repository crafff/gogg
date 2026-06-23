package storage

import (
	"context"
	"time"
)

type Player struct {
	PUUID     string
	GameName  *string
	TagLine   *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UpsertPlayer inserts or updates a player's identity info.
func (s *Store) UpsertPlayer(ctx context.Context, puuid, region string, gameName, tagLine *string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO players (puuid, region, game_name, tag_line)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (puuid) DO UPDATE
		SET region     = EXCLUDED.region,
		    game_name  = COALESCE(EXCLUDED.game_name, players.game_name),
		    tag_line   = COALESCE(EXCLUDED.tag_line,  players.tag_line),
		    updated_at = now()`,
		puuid, region, gameName, tagLine)
	return err
}

// GetPlayerSyncTime returns the last synced match timestamp for a player in
// the given region, or zero time if never synced.
func (s *Store) GetPlayerSyncTime(ctx context.Context, puuid, region string) (time.Time, error) {
	var t time.Time
	err := s.Pool.QueryRow(ctx,
		`SELECT last_synced_at FROM player_match_sync WHERE puuid = $1 AND region = $2`,
		puuid, region).Scan(&t)
	if err != nil {
		return time.Time{}, nil
	}
	return t, nil
}

// SetPlayerSyncTime upserts the last synced match timestamp for a player in a region.
func (s *Store) SetPlayerSyncTime(ctx context.Context, puuid, region string, t time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO player_match_sync (puuid, region, last_synced_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (puuid, region) DO UPDATE SET last_synced_at = EXCLUDED.last_synced_at`,
		puuid, region, t)
	return err
}
