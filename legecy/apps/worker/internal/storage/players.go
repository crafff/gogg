package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
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
	return upsertPlayerTx(ctx, s.Pool, puuid, region, gameName, tagLine)
}

// UpsertPlayerFromMatch fills missing identity fields but never replaces an
// identity already learned from Account-V1 or a current rank snapshot. Match
// details preserve the Riot ID used when that historical game was played, so
// treating them as current can make a renamed player temporarily unsearchable.
func (s *Store) UpsertPlayerFromMatch(ctx context.Context, puuid, region string, gameName, tagLine *string) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO players (puuid, region, game_name, tag_line)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (puuid) DO UPDATE
		SET region     = EXCLUDED.region,
		    game_name  = COALESCE(players.game_name, EXCLUDED.game_name),
		    tag_line   = COALESCE(players.tag_line, EXCLUDED.tag_line),
		    updated_at = now()`,
		puuid, region, gameName, tagLine)
	return err
}

func upsertPlayerTx(ctx context.Context, exec execer, puuid, region string, gameName, tagLine *string) error {
	_, err := exec.Exec(ctx, `
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
	if err == pgx.ErrNoRows {
		return time.Time{}, nil
	}
	return t, err
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

// SavePlayerMatchIDs atomically records collected match IDs and only then
// advances the player's sync watermark.
func (s *Store) SavePlayerMatchIDs(ctx context.Context, runID int, puuid, region, version string, ids []string, syncedAt time.Time) error {
	return s.WithTx(ctx, func(tx pgx.Tx) error {
		for _, id := range ids {
			if _, err := tx.Exec(ctx, `
				INSERT INTO matches (match_id, region, version) VALUES ($1, $2, $3)
				ON CONFLICT (match_id) DO NOTHING`, id, region, version); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO statistics_match_membership (dataset_key, match_id, run_id)
				VALUES ('ranked-solo-v1', $1, $2)
				ON CONFLICT (dataset_key, match_id) DO UPDATE
				SET run_id = COALESCE(statistics_match_membership.run_id, EXCLUDED.run_id)`, id, runID); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO player_match_sync (puuid, region, last_synced_at)
			VALUES ($1, $2, $3)
			ON CONFLICT (puuid, region) DO UPDATE SET last_synced_at = EXCLUDED.last_synced_at`,
			puuid, region, syncedAt)
		return err
	})
}

func (s *Store) SavePlayerRankSnapshot(ctx context.Context, puuid, region string, gameName, tagLine *string, snap *RankSnapshot) error {
	return s.WithTx(ctx, func(tx pgx.Tx) error {
		if err := upsertPlayerTx(ctx, tx, puuid, region, gameName, tagLine); err != nil {
			return err
		}
		return upsertSnapshotTx(ctx, tx, snap)
	})
}
