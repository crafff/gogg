package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ItemCatalogEntry is one row in item_catalog.
type ItemCatalogEntry struct {
	ItemID      int
	Patch       string
	Name        string
	PriceTotal  int
	IsCompleted bool
	IsBoots     bool
	IsSkippable bool // consumable / trinket / vision — excluded from build analysis
	FromIDs     []int
	ToIDs       []int
}

// ItemSets groups item classification sets for a single patch.
type ItemSets struct {
	Completed       map[int]bool // terminal recipe items
	Boots           map[int]bool // completed boots
	Skippable       map[int]bool // consumables / trinkets / vision wards
	StarterEligible map[int]bool // price_total <= 500, excluding yellow trinket (3340)
	Prices          map[int]int  // item_id → price_total, used for cumulative starter check
}

// CompletedItem is one row in match_completed_items.
type CompletedItem struct {
	MatchID       string
	ParticipantID int
	Slot          int
	ItemID        int
	TimestampMs   int
	IsBoots       bool
}

// StarterItem is one row in match_starter_items.
type StarterItem struct {
	MatchID       string
	ParticipantID int
	ItemID        int
	TimestampMs   int
}

// BootsItem is one row in match_boots — the latest completed boots per participant.
type BootsItem struct {
	MatchID       string
	ParticipantID int
	ItemID        int
	TimestampMs   int
}

// IsCatalogLoaded returns true if item_catalog already has entries for this patch.
func (s *Store) IsCatalogLoaded(ctx context.Context, patch string) (bool, error) {
	var n int
	err := s.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM item_catalog WHERE patch = $1`, patch).Scan(&n)
	return n > 0, err
}

// UpsertItemCatalog bulk-inserts or updates item catalog entries for a patch.
func (s *Store) UpsertItemCatalog(ctx context.Context, entries []ItemCatalogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, e := range entries {
		batch.Queue(`
			INSERT INTO item_catalog
			  (item_id, patch, name, price_total, is_completed, is_boots, is_skippable, from_ids, to_ids)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (item_id, patch) DO NOTHING`,
			e.ItemID, e.Patch, e.Name, e.PriceTotal,
			e.IsCompleted, e.IsBoots, e.IsSkippable, e.FromIDs, e.ToIDs,
		)
	}
	return s.Pool.SendBatch(ctx, batch).Close()
}

const (
	yellowTrinketID = 3340
	starterMaxPrice = 500
)

// GetItemSets returns all item classification sets for a patch.
func (s *Store) GetItemSets(ctx context.Context, patch string) (*ItemSets, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT item_id, is_completed, is_boots, is_skippable, price_total
		 FROM item_catalog WHERE patch = $1`, patch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sets := &ItemSets{
		Completed:       make(map[int]bool),
		Boots:           make(map[int]bool),
		Skippable:       make(map[int]bool),
		StarterEligible: make(map[int]bool),
		Prices:          make(map[int]int),
	}
	for rows.Next() {
		var id, priceTotal int
		var completed, boots, skippable bool
		if err := rows.Scan(&id, &completed, &boots, &skippable, &priceTotal); err != nil {
			return nil, err
		}
		sets.Prices[id] = priceTotal
		if completed {
			sets.Completed[id] = true
		}
		if boots {
			sets.Boots[id] = true
		}
		if skippable {
			sets.Skippable[id] = true
		}
		if priceTotal <= starterMaxPrice && id != yellowTrinketID {
			sets.StarterEligible[id] = true
		}
	}
	return sets, rows.Err()
}

// GetMatchesNeedingItems returns match_ids with timeline_status='done' and
// items_status='pending' for the given region and version.
func (s *Store) GetMatchesNeedingItems(ctx context.Context, region, version string, limit int) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT match_id FROM matches
		WHERE fetch_status    = 'done'
		  AND timeline_status = 'done'
		  AND items_status    = 'pending'
		  AND region          = $1
		  AND version         = $2
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

// CountMatchesNeedingItems returns the count of matches pending item classification.
func (s *Store) CountMatchesNeedingItems(ctx context.Context, region, version string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM matches
		WHERE fetch_status    = 'done'
		  AND timeline_status = 'done'
		  AND items_status    = 'pending'
		  AND region          = $1
		  AND version         = $2`, region, version).Scan(&n)
	return n, err
}

// GetItemEventsForMatch returns all item purchase events for a match.
// RemovalType is included so callers can filter by NULL (kept) or 'sold'/'undo'.
func (s *Store) GetItemEventsForMatch(ctx context.Context, matchID string) ([]ItemEvent, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT participant_id, timestamp_ms, item_id, COALESCE(removal_type, '')
		FROM match_item_events
		WHERE match_id = $1
		ORDER BY timestamp_ms`, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []ItemEvent
	for rows.Next() {
		var e ItemEvent
		e.MatchID = matchID
		if err := rows.Scan(&e.ParticipantID, &e.TimestampMs, &e.ItemID, &e.RemovalType); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetGameVersionForMatch returns the game_version stored on a match.
func (s *Store) GetGameVersionForMatch(ctx context.Context, matchID string) (string, error) {
	var v string
	err := s.Pool.QueryRow(ctx,
		`SELECT COALESCE(NULLIF(game_version, ''), NULLIF(version, '')) FROM matches WHERE match_id = $1`, matchID).Scan(&v)
	return v, err
}

// InsertCompletedItems bulk-inserts completed-item records for a match.
func (s *Store) InsertCompletedItems(ctx context.Context, items []CompletedItem) error {
	return insertCompletedItemsTx(ctx, s.Pool, items)
}

func insertCompletedItemsTx(ctx context.Context, sender batchSender, items []CompletedItem) error {
	if len(items) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, ci := range items {
		batch.Queue(`
			INSERT INTO match_completed_items
			  (match_id, participant_id, slot, item_id, timestamp_ms, is_boots)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (match_id, participant_id, slot) DO NOTHING`,
			ci.MatchID, ci.ParticipantID, ci.Slot, ci.ItemID, ci.TimestampMs, ci.IsBoots,
		)
	}
	return sender.SendBatch(ctx, batch).Close()
}

// InsertStarterItems bulk-inserts starter-item records for a match.
func (s *Store) InsertStarterItems(ctx context.Context, items []StarterItem) error {
	return insertStarterItemsTx(ctx, s.Pool, items)
}

func insertStarterItemsTx(ctx context.Context, sender batchSender, items []StarterItem) error {
	if len(items) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, si := range items {
		batch.Queue(`
			INSERT INTO match_starter_items (match_id, participant_id, item_id, timestamp_ms)
			VALUES ($1,$2,$3,$4)`,
			si.MatchID, si.ParticipantID, si.ItemID, si.TimestampMs,
		)
	}
	return sender.SendBatch(ctx, batch).Close()
}

// InsertBoots upserts the boots item for each participant of a match.
func (s *Store) InsertBoots(ctx context.Context, items []BootsItem) error {
	return insertBootsTx(ctx, s.Pool, items)
}

func insertBootsTx(ctx context.Context, sender batchSender, items []BootsItem) error {
	if len(items) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, b := range items {
		batch.Queue(`
			INSERT INTO match_boots (match_id, participant_id, item_id, timestamp_ms)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (match_id, participant_id) DO UPDATE
			SET item_id = EXCLUDED.item_id, timestamp_ms = EXCLUDED.timestamp_ms`,
			b.MatchID, b.ParticipantID, b.ItemID, b.TimestampMs,
		)
	}
	return sender.SendBatch(ctx, batch).Close()
}

// MarkItemsDone sets items_status = 'done' for a match.
func (s *Store) MarkItemsDone(ctx context.Context, matchID string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE matches SET items_status = 'done' WHERE match_id = $1`, matchID)
	return err
}

// SaveItemClassification atomically replaces item classification rows for a
// match and marks item classification done only after all rows are written.
func (s *Store) SaveItemClassification(ctx context.Context, matchID string, completed []CompletedItem, starter []StarterItem, boots []BootsItem) error {
	return s.WithTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM match_completed_items WHERE match_id = $1`, matchID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM match_starter_items WHERE match_id = $1`, matchID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM match_boots WHERE match_id = $1`, matchID); err != nil {
			return err
		}
		if err := insertCompletedItemsTx(ctx, tx, completed); err != nil {
			return err
		}
		if err := insertStarterItemsTx(ctx, tx, starter); err != nil {
			return err
		}
		if err := insertBootsTx(ctx, tx, boots); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE matches SET items_status = 'done' WHERE match_id = $1`, matchID)
		return err
	})
}

const maxItemsRetries = 3

// MarkItemsError increments items_retry_count. Status stays 'pending'
// until maxItemsRetries is reached, then becomes 'error'.
func (s *Store) MarkItemsError(ctx context.Context, matchID string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE matches
		SET items_retry_count = items_retry_count + 1,
		    items_status = CASE
		        WHEN items_retry_count + 1 >= $2 THEN 'error'
		        ELSE 'pending'
		    END
		WHERE match_id = $1`, matchID, maxItemsRetries)
	return err
}
