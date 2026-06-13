package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type ItemEvent struct {
	MatchID       string
	ParticipantID int
	TimestampMs   int
	ItemID        int
	RemovalType   string // "" = active, "undo" = refunded, "sold" = sold
}

type SkillEvent struct {
	MatchID       string
	ParticipantID int
	TimestampMs   int
	SkillSlot     int    // 1=Q 2=W 3=E 4=R
	LevelUpType   string // "NORMAL" | "EVOLVE"
}

type ParticipantSnapshot struct {
	MatchID       string
	ParticipantID int
	Minute        int
	TotalGold     int
	CurrentGold   int
	CS            int
	JungleCS      int
	Level         int
	XP            int
	PosX          int
	PosY          int
	TimeEnemyCC   int
	WardsPlaced   int
	WardsKilled   int
	// cumulative damage at this minute
	DmgTotal        int
	DmgToChamps     int
	DmgMagicChamps  int
	DmgPhysChamps   int
	DmgTrueChamps   int
	DmgTaken        int
}

// GetMatchesNeedingTimeline returns match_ids with fetch_status='done' and
// timeline_status='pending' for the given region and version.
func (s *Store) GetMatchesNeedingTimeline(ctx context.Context, region, version string, limit int) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT match_id FROM matches
		WHERE fetch_status = 'done'
		  AND timeline_status = 'pending'
		  AND region = $1
		  AND version = $2
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

// CountMatchesNeedingTimeline returns the total pending timeline count.
func (s *Store) CountMatchesNeedingTimeline(ctx context.Context, region, version string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM matches
		WHERE fetch_status = 'done'
		  AND timeline_status = 'pending'
		  AND region = $1
		  AND version = $2`, region, version).Scan(&n)
	return n, err
}

// MarkTimelineDone sets timeline_status = 'done' for a match.
func (s *Store) MarkTimelineDone(ctx context.Context, matchID string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE matches SET timeline_status = 'done' WHERE match_id = $1`, matchID)
	return err
}

const maxTimelineRetries = 3

// MarkTimelineError increments timeline_retry_count. Status stays 'pending'
// until maxTimelineRetries is reached, then becomes 'error'.
func (s *Store) MarkTimelineError(ctx context.Context, matchID string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE matches
		SET timeline_retry_count = timeline_retry_count + 1,
		    timeline_status = CASE
		        WHEN timeline_retry_count + 1 >= $2 THEN 'error'
		        ELSE 'pending'
		    END
		WHERE match_id = $1`, matchID, maxTimelineRetries)
	return err
}

// InsertSkillEvents bulk-inserts skill level-up events for a match.
func (s *Store) InsertSkillEvents(ctx context.Context, events []SkillEvent) error {
	if len(events) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, e := range events {
		batch.Queue(`
			INSERT INTO match_skill_events (match_id, participant_id, timestamp_ms, skill_slot, level_up_type)
			VALUES ($1, $2, $3, $4, $5)`,
			e.MatchID, e.ParticipantID, e.TimestampMs, e.SkillSlot, e.LevelUpType)
	}
	return s.Pool.SendBatch(ctx, batch).Close()
}

// InsertItemEvents bulk-inserts item purchase events for a match.
func (s *Store) InsertItemEvents(ctx context.Context, events []ItemEvent) error {
	if len(events) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, e := range events {
		var removal *string
		if e.RemovalType != "" {
			removal = &e.RemovalType
		}
		batch.Queue(`
			INSERT INTO match_item_events (match_id, participant_id, timestamp_ms, item_id, removal_type)
			VALUES ($1, $2, $3, $4, $5)`,
			e.MatchID, e.ParticipantID, e.TimestampMs, e.ItemID, removal)
	}
	return s.Pool.SendBatch(ctx, batch).Close()
}

// InsertParticipantSnapshots bulk-inserts per-minute snapshots for a match.
func (s *Store) InsertParticipantSnapshots(ctx context.Context, snaps []ParticipantSnapshot) error {
	if len(snaps) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, sn := range snaps {
		batch.Queue(`
			INSERT INTO match_participant_snapshots
			  (match_id, participant_id, minute,
			   total_gold, current_gold, cs, jungle_cs, level, xp,
			   pos_x, pos_y, time_enemy_cc, wards_placed, wards_killed,
			   dmg_total, dmg_to_champs, dmg_magic_champs, dmg_phys_champs,
			   dmg_true_champs, dmg_taken)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
			ON CONFLICT (match_id, participant_id, minute) DO NOTHING`,
			sn.MatchID, sn.ParticipantID, sn.Minute,
			sn.TotalGold, sn.CurrentGold, sn.CS, sn.JungleCS,
			sn.Level, sn.XP, sn.PosX, sn.PosY,
			sn.TimeEnemyCC, sn.WardsPlaced, sn.WardsKilled,
			sn.DmgTotal, sn.DmgToChamps, sn.DmgMagicChamps, sn.DmgPhysChamps,
			sn.DmgTrueChamps, sn.DmgTaken,
		)
	}
	return s.Pool.SendBatch(ctx, batch).Close()
}