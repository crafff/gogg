// Package phase5 fetches match timelines and extracts item events and
// per-minute performance snapshots.
package phase5

import (
	"context"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/crawler"
	"github.com/crafff/gogg/apps/worker/internal/crawler/heartbeat"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phaselog"
	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
)

// isSnapshotMinute returns true for every 5-minute mark (5, 10, 15, ...).
// Recording continues until the last frame, so long games are fully covered.
func isSnapshotMinute(minute int) bool {
	return minute > 0 && minute%5 == 0
}

const batchSize = 200 // timeline responses are large; keep batches small

type Phase struct {
	riot  *riotapi.Client
	store *storage.Store
}

func New(riot *riotapi.Client, store *storage.Store) *Phase {
	return &Phase{riot: riot, store: store}
}

func (p *Phase) ID() int      { return 5 }
func (p *Phase) Name() string { return "Phase5:Timeline" }

func (p *Phase) IsDone(ctx context.Context, state *crawler.RunState) (bool, error) {
	ids, err := p.store.GetMatchesNeedingTimeline(ctx, state.Region(), state.Profile.Version, 1)
	if err != nil {
		return false, err
	}
	return len(ids) == 0, nil
}

func (p *Phase) Run(ctx context.Context, state *crawler.RunState) error {
	region := state.Region()
	version := state.Profile.Version
	meta := phaselog.Meta{RunID: state.ID, Region: region, Phase: p.Name(), PhaseID: p.ID(), Version: version}

	total, err := p.store.CountMatchesNeedingTimeline(ctx, region, version)
	if err != nil {
		return err
	}
	phaselog.Step(meta, "pending_loaded", "pending", total)

	processed, failed := 0, 0
	start := time.Now()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		ids, err := p.store.GetMatchesNeedingTimeline(ctx, region, version, batchSize)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		heartbeat.Record(ctx, map[string]any{
			"run_id":     state.ID,
			"region":     region,
			"version":    version,
			"batch_size": len(ids),
			"processed":  processed,
			"total":      total,
			"failed":     failed,
		})

		for idx, matchID := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			heartbeat.Record(ctx, map[string]any{
				"run_id":    state.ID,
				"region":    region,
				"version":   version,
				"match_id":  matchID,
				"processed": processed,
				"total":     total,
				"failed":    failed,
			})
			if err := p.processTimeline(ctx, matchID); err != nil {
				phaselog.Warn(meta, "match_failed", "match_id", matchID, "err", err)
				if err2 := p.store.MarkTimelineError(ctx, matchID); err2 != nil {
					return err2
				}
				failed++
			}
			processed++
			if (idx+1)%10 == 0 {
				heartbeat.Record(ctx, map[string]any{
					"run_id":             state.ID,
					"region":             region,
					"version":            version,
					"processed_in_batch": idx + 1,
					"batch_size":         len(ids),
					"processed":          processed,
					"total":              total,
					"failed":             failed,
				})
			}
		}
		phaselog.Progress(meta, processed, total, failed, start)
	}
	return nil
}

func (p *Phase) processTimeline(ctx context.Context, matchID string) error {
	dto, err := p.riot.GetMatchTimeline(ctx, matchID)
	if err != nil {
		return err
	}

	itemEvents, skillEvents, snapshots := extractTimeline(matchID, dto)

	return p.store.SaveTimeline(ctx, matchID, itemEvents, skillEvents, snapshots)
}

// extractTimeline parses a TimelineDTO and returns item purchase events and
// per-minute participant snapshots.
func extractTimeline(matchID string, dto *riotapi.TimelineDTO) ([]storage.ItemEvent, []storage.SkillEvent, []storage.ParticipantSnapshot) {
	var itemEvents []storage.ItemEvent
	var skillEvents []storage.SkillEvent
	var snapshots []storage.ParticipantSnapshot

	// running ward counters per participantId
	wardsPlaced := make(map[int]int)
	wardsKilled := make(map[int]int)

	type pendingPurchase struct {
		idx    int
		itemID int
	}
	pending := make(map[int][]pendingPurchase)

	seenMinutes := make(map[int]bool)

	for _, frame := range dto.Info.Frames {
		minute := frame.Timestamp / 60000

		// Process events first (ward counters must be updated before snapshot).
		for _, ev := range frame.Events {
			switch ev.Type {
			case "ITEM_PURCHASED":
				if ev.ParticipantId == 0 {
					continue
				}
				idx := len(itemEvents)
				itemEvents = append(itemEvents, storage.ItemEvent{
					MatchID:       matchID,
					ParticipantID: ev.ParticipantId,
					TimestampMs:   ev.Timestamp,
					ItemID:        ev.ItemId,
				})
				pending[ev.ParticipantId] = append(pending[ev.ParticipantId], pendingPurchase{idx, ev.ItemId})

			case "ITEM_UNDO":
				// Refund within ~10 seconds: mark the purchase as undone.
				if ev.ParticipantId == 0 {
					continue
				}
				pp := pending[ev.ParticipantId]
				for i := len(pp) - 1; i >= 0; i-- {
					if pp[i].itemID == ev.BeforeId {
						itemEvents[pp[i].idx].RemovalType = "undo"
						pending[ev.ParticipantId] = append(pp[:i], pp[i+1:]...)
						break
					}
				}

			case "ITEM_SOLD":
				// Deliberate sell: mark the most recent active purchase of this item.
				if ev.ParticipantId == 0 {
					continue
				}
				pp := pending[ev.ParticipantId]
				for i := len(pp) - 1; i >= 0; i-- {
					if pp[i].itemID == ev.ItemId {
						itemEvents[pp[i].idx].RemovalType = "sold"
						pending[ev.ParticipantId] = append(pp[:i], pp[i+1:]...)
						break
					}
				}

			case "SKILL_LEVEL_UP":
				if ev.ParticipantId > 0 && ev.SkillSlot >= 1 && ev.SkillSlot <= 4 {
					skillEvents = append(skillEvents, storage.SkillEvent{
						MatchID:       matchID,
						ParticipantID: ev.ParticipantId,
						TimestampMs:   ev.Timestamp,
						SkillSlot:     ev.SkillSlot,
						LevelUpType:   ev.LevelUpType,
					})
				}

			case "WARD_PLACED":
				if ev.CreatorId > 0 {
					wardsPlaced[ev.CreatorId]++
				}

			case "WARD_KILL":
				if ev.KillerId > 0 {
					wardsKilled[ev.KillerId]++
				}
			}
		}

		// Take snapshot at every 5-minute mark, for the full game duration.
		if isSnapshotMinute(minute) && !seenMinutes[minute] {
			seenMinutes[minute] = true
			for _, pf := range frame.ParticipantFrames {
				d := pf.DamageStats
				snapshots = append(snapshots, storage.ParticipantSnapshot{
					MatchID:        matchID,
					ParticipantID:  pf.ParticipantId,
					Minute:         minute,
					TotalGold:      pf.TotalGold,
					CurrentGold:    pf.CurrentGold,
					CS:             pf.MinionsKilled,
					JungleCS:       pf.JungleMinionsKilled,
					Level:          pf.Level,
					XP:             pf.XP,
					PosX:           pf.Position.X,
					PosY:           pf.Position.Y,
					TimeEnemyCC:    pf.TimeEnemySpentControlled,
					WardsPlaced:    wardsPlaced[pf.ParticipantId],
					WardsKilled:    wardsKilled[pf.ParticipantId],
					DmgTotal:       d.TotalDamageDone,
					DmgToChamps:    d.TotalDamageDoneToChampions,
					DmgMagicChamps: d.MagicDamageDoneToChampions,
					DmgPhysChamps:  d.PhysicalDamageDoneToChampions,
					DmgTrueChamps:  d.TrueDamageDoneToChampions,
					DmgTaken:       d.TotalDamageTaken,
				})
			}
		}
	}

	return itemEvents, skillEvents, snapshots
}
