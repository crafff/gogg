// Package phase35 fetches ranks on-demand for participants missing tier data.
package phase35

import (
	"context"
	"log/slog"

	"github.com/crafff/gogg/apps/worker/internal/crawler"
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
	queried := make(map[string]bool)
	total := 0

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
		for _, puuid := range puuids {
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
				slog.Warn("phase35: failed to fetch rank", "puuid", puuid[:8], "err", err)
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
				if err := p.store.InsertSnapshot(ctx, snap); err != nil {
					return err
				}
				if err := p.store.UpdateParticipantTierByPUUID(ctx, puuid,
					entry.Tier, entry.Rank, entry.LeaguePoints); err != nil {
					return err
				}
				total++
				found = true
				break
			}

			// No ranked data for this queue: mark as UNRANKED so future runs
			// skip this puuid instead of querying the API again.
			if !found {
				if err := p.store.MarkParticipantUnranked(ctx, puuid); err != nil {
					return err
				}
			}
		}
		slog.Info("phase35: backfilled tier data", "total", total)
		if newPUUIDs == 0 {
			break
		}
	}
	return nil
}
