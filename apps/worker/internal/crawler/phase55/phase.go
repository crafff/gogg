// Package phase55 classifies item purchase events into completed items (大件),
// boots, and starter items (出门装) per participant.
package phase55

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/crafff/gogg/apps/worker/internal/crawler"
	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
)

const (
	batchSize        = 500
	maxSlots         = 6     // maximum item slots in League of Legends
	starterCutoff    = 90000 // ms — items bought before this are considered starter items
	starterGoldLimit = 500   // cumulative gold limit for starter items
)

type Phase struct {
	store *storage.Store
}

func New(store *storage.Store) *Phase {
	return &Phase{store: store}
}

func (p *Phase) ID() int      { return 55 }
func (p *Phase) Name() string { return "Phase55:ItemClassification" }

func (p *Phase) IsDone(ctx context.Context, state *crawler.RunState) (bool, error) {
	ids, err := p.store.GetMatchesNeedingItems(ctx, state.Region(), state.Profile.Version, 1)
	if err != nil {
		return false, err
	}
	return len(ids) == 0, nil
}

func (p *Phase) Run(ctx context.Context, state *crawler.RunState) error {
	region := state.Region()
	version := state.Profile.Version

	total, err := p.store.CountMatchesNeedingItems(ctx, region, version)
	if err != nil {
		return err
	}
	slog.Info("phase55: start", "pending", total)

	// Cache ItemSets per CDragon patch across the batch.
	setsCache := make(map[string]*storage.ItemSets)

	processed, failed := 0, 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		ids, err := p.store.GetMatchesNeedingItems(ctx, region, version, batchSize)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		for _, matchID := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := p.processMatch(ctx, matchID, setsCache); err != nil {
				slog.Warn("phase55: failed", "match_id", matchID, "err", err)
				if err2 := p.store.MarkItemsError(ctx, matchID); err2 != nil {
					return err2
				}
				failed++
			}
			processed++
		}
		slog.Info("phase55: progress", "processed", processed, "total", total, "failed", failed)
	}
	return nil
}

func (p *Phase) processMatch(ctx context.Context, matchID string, cache map[string]*storage.ItemSets) error {
	gameVersion, err := p.store.GetGameVersionForMatch(ctx, matchID)
	if err != nil {
		return err
	}
	patch := riotapi.ExtractCDragonPatch(gameVersion)
	if patch == "" {
		return fmt.Errorf("match %s has no version, skipping item classification", matchID)
	}

	if _, ok := cache[patch]; !ok {
		sets, err := p.loadCatalog(ctx, patch)
		if err != nil {
			return err
		}
		cache[patch] = sets
	}
	sets := cache[patch]

	events, err := p.store.GetItemEventsForMatch(ctx, matchID)
	if err != nil {
		return err
	}

	completedItems, starterItems, bootsItems := classify(matchID, events, sets)

	if err := p.store.InsertCompletedItems(ctx, completedItems); err != nil {
		return err
	}
	if err := p.store.InsertStarterItems(ctx, starterItems); err != nil {
		return err
	}
	if err := p.store.InsertBoots(ctx, bootsItems); err != nil {
		return err
	}
	return p.store.MarkItemsDone(ctx, matchID)
}

// classify processes item events for one match and returns:
//   - completed items (slots 1-6, boots flagged)
//   - starter items (bought within 90s, price <= 500, not yellow trinket)
//   - boots (latest purchased completed boots per participant, undo excluded)
func classify(matchID string, events []storage.ItemEvent, sets *storage.ItemSets) (
	[]storage.CompletedItem, []storage.StarterItem, []storage.BootsItem,
) {
	type buildEntry struct {
		itemID      int
		timestampMs int
	}

	builds := make(map[int][]buildEntry)
	starters := make(map[int][]storage.StarterItem)
	starterTotals := make(map[int]int) // pid → cumulative gold spent on starter items
	latestBoots := make(map[int]storage.BootsItem)

	for _, ev := range events {
		pid := ev.ParticipantID

		// Starter items: bought within 90s, not undo, not yellow trinket,
		// cumulative price_total per participant <= 500.
		if ev.TimestampMs < starterCutoff &&
			ev.RemovalType != "undo" &&
			sets.StarterEligible[ev.ItemID] {
			price := sets.Prices[ev.ItemID]
			if starterTotals[pid]+price <= starterGoldLimit {
				starterTotals[pid] += price
				starters[pid] = append(starters[pid], storage.StarterItem{
					MatchID:       matchID,
					ParticipantID: pid,
					ItemID:        ev.ItemID,
					TimestampMs:   ev.TimestampMs,
				})
			}
		}

		// Boots: latest completed boots, undo excluded (sold is OK).
		if sets.Boots[ev.ItemID] && ev.RemovalType != "undo" {
			if existing, ok := latestBoots[pid]; !ok || ev.TimestampMs > existing.TimestampMs {
				latestBoots[pid] = storage.BootsItem{
					MatchID:       matchID,
					ParticipantID: pid,
					ItemID:        ev.ItemID,
					TimestampMs:   ev.TimestampMs,
				}
			}
		}

		// Completed items: terminal recipe items still held at game end.
		if ev.RemovalType == "" && sets.Completed[ev.ItemID] && len(builds[pid]) < maxSlots {
			builds[pid] = append(builds[pid], buildEntry{ev.ItemID, ev.TimestampMs})
		}
	}

	var completedItems []storage.CompletedItem
	for pid, entries := range builds {
		for slot, e := range entries {
			completedItems = append(completedItems, storage.CompletedItem{
				MatchID:       matchID,
				ParticipantID: pid,
				Slot:          slot + 1,
				ItemID:        e.itemID,
				TimestampMs:   e.timestampMs,
				IsBoots:       sets.Boots[e.itemID],
			})
		}
	}

	var starterItems []storage.StarterItem
	for _, items := range starters {
		starterItems = append(starterItems, items...)
	}

	var bootsItems []storage.BootsItem
	for _, b := range latestBoots {
		bootsItems = append(bootsItems, b)
	}

	return completedItems, starterItems, bootsItems
}

// loadCatalog ensures item_catalog is populated for the patch and returns
// the item classification sets.
func (p *Phase) loadCatalog(ctx context.Context, patch string) (*storage.ItemSets, error) {
	loaded, err := p.store.IsCatalogLoaded(ctx, patch)
	if err != nil {
		return nil, err
	}
	if !loaded {
		slog.Info("phase55: fetching item catalog", "patch", patch)
		items, err := riotapi.GetItemCatalog(ctx, patch)
		if err != nil {
			return nil, err
		}
		entries := make([]storage.ItemCatalogEntry, 0, len(items))
		for _, item := range items {
			entries = append(entries, storage.ItemCatalogEntry{
				ItemID:      item.ID,
				Patch:       patch,
				Name:        item.Name,
				PriceTotal:  item.PriceTotal,
				IsCompleted: riotapi.IsCompletedItem(item),
				IsBoots:     riotapi.IsBoots(item),
				IsSkippable: riotapi.IsSkippable(item),
				FromIDs:     item.From,
				ToIDs:       item.To,
			})
		}
		if err := p.store.UpsertItemCatalog(ctx, entries); err != nil {
			return nil, err
		}
		slog.Info("phase55: catalog loaded", "patch", patch, "items", len(entries))
	}
	return p.store.GetItemSets(ctx, patch)
}
