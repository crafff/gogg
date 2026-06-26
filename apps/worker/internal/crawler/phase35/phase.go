// Package phase35 fetches ranks on-demand for participants missing tier data.
package phase35

import (
	"context"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/crawler"
	"github.com/crafff/gogg/apps/worker/internal/crawler/heartbeat"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phaselog"
	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
)

const batchSize = 1000

type Phase struct {
	riot  *riotapi.Client
	store *storage.Store
}

func New(riot *riotapi.Client, store *storage.Store) *Phase {
	return &Phase{riot: riot, store: store}
}

func (p *Phase) ID() int      { return 35 }
func (p *Phase) Name() string { return "Phase35:OnDemandRank" }

func (p *Phase) IsDone(ctx context.Context, state *crawler.RunState) (bool, error) {
	puuids, err := p.store.GetParticipantsMissingTier(ctx, state.Region(), 1)
	if err != nil {
		return false, err
	}
	return len(puuids) == 0, nil
}

func (p *Phase) Run(ctx context.Context, state *crawler.RunState) error {
	meta := phaselog.Meta{RunID: state.ID, Region: state.Region(), Phase: p.Name(), PhaseID: p.ID(), Version: state.Profile.Version, Queue: state.Profile.Queue}
	pending, err := p.store.CountParticipantsMissingTier(ctx, state.Region())
	if err != nil {
		return err
	}
	phaselog.Step(meta, "pending_loaded", "pending", pending)
	queried := make(map[string]bool)
	processed, backfilled := 0, 0
	start := time.Now()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		puuids, err := p.store.GetParticipantsMissingTier(ctx, state.Region(), batchSize)
		if err != nil {
			return err
		}
		if len(puuids) == 0 {
			break
		}

		newPUUIDs := 0
		heartbeat.Record(ctx, map[string]any{
			"run_id":           state.ID,
			"region":           state.Region(),
			"batch_size":       len(puuids),
			"processed":        processed,
			"total":            pending,
			"backfilled_total": backfilled,
		})

		for idx, puuid := range puuids {
			if queried[puuid] {
				continue
			}
			queried[puuid] = true
			newPUUIDs++

			if err := ctx.Err(); err != nil {
				return err
			}

			entries, err := p.riot.GetEntriesByPUUID(ctx, puuid)
			if err != nil {
				phaselog.Warn(meta, "rank_fetch_failed", "puuid_prefix", puuid[:8], "err", err)
				processed++
				if processed%100 == 0 {
					phaselog.Progress(meta, processed, pending, 0, start, "backfilled_total", backfilled)
				}
				continue
			}

			found := false
			for _, entry := range entries {
				if entry.QueueType != state.Profile.Queue {
					continue
				}
				leagueID := entry.LeagueID
				divPtr := &entry.Rank
				snap := &storage.RankSnapshot{
					RunID:        &state.ID,
					PUUID:        puuid,
					Region:       state.Region(),
					Source:       "on_demand",
					LeagueID:     &leagueID,
					Queue:        entry.QueueType,
					Tier:         entry.Tier,
					Division:     divPtr,
					LeaguePoints: &entry.LeaguePoints,
					Wins:         &entry.Wins,
					Losses:       &entry.Losses,
					Veteran:      &entry.Veteran,
					Inactive:     &entry.Inactive,
					FreshBlood:   &entry.FreshBlood,
					HotStreak:    &entry.HotStreak,
					RankStatus:   "active",
				}
				if err := p.store.SaveRankBackfill(ctx, snap, entry.Tier, entry.Rank, entry.LeaguePoints); err != nil {
					return err
				}
				backfilled++
				found = true
				break
			}

			// No ranked data for this queue: mark as UNRANKED so future runs
			// skip this puuid instead of querying the API again.
			if !found {
				if err := p.store.SaveParticipantUnranked(ctx, puuid); err != nil {
					return err
				}
			}

			processed++
			if processed%100 == 0 {
				phaselog.Progress(meta, processed, pending, 0, start, "backfilled_total", backfilled)
			}
			if (idx+1)%25 == 0 {
				heartbeat.Record(ctx, map[string]any{
					"run_id":             state.ID,
					"region":             state.Region(),
					"processed_in_batch": idx + 1,
					"batch_size":         len(puuids),
					"processed":          processed,
					"total":              pending,
					"backfilled_total":   backfilled,
				})
			}
		}
		if newPUUIDs == 0 {
			break
		}
	}
	if processed%100 != 0 {
		phaselog.Progress(meta, processed, pending, 0, start, "backfilled_total", backfilled)
	}
	return nil
}
