// Package phase1 syncs player rank data for configured tiers.
package phase1

import (
	"context"
	"log/slog"

	"github.com/crafff/gogg/apps/worker/internal/crawler"
	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
)

var topTiers = map[string]bool{
	"CHALLENGER":  true,
	"GRANDMASTER": true,
	"MASTER":      true,
}

var divisions = []string{"I", "II", "III", "IV"}

type Phase struct {
	riot  *riotapi.Client
	store *storage.Store
}

func New(riot *riotapi.Client, store *storage.Store) *Phase {
	return &Phase{riot: riot, store: store}
}

func (p *Phase) ID() int      { return 1 }
func (p *Phase) Name() string { return "Phase1:RankSync" }

func (p *Phase) IsDone(_ context.Context, _ *crawler.RunState) (bool, error) {
	return false, nil // always refresh ranks
}

func (p *Phase) Run(ctx context.Context, state *crawler.RunState) error {
	for _, tier := range state.Profile.RankPrefetchTiers {
		if err := ctx.Err(); err != nil {
			return err
		}
		slog.Info("syncing tier", "tier", tier)
		tierCopy := tier
		if err := state.SaveCheckpoint(ctx, 1, &tierCopy); err != nil {
			return err
		}
		if err := p.syncTier(ctx, state, tier); err != nil {
			return err
		}
	}
	return nil
}

func (p *Phase) syncTier(ctx context.Context, state *crawler.RunState, tier string) error {
	if topTiers[tier] {
		return p.syncTopTier(ctx, state, tier)
	}
	return p.syncDivisionTier(ctx, state, tier)
}

func (p *Phase) syncTopTier(ctx context.Context, state *crawler.RunState, tier string) error {
	var (
		list *riotapi.LeagueListDTO
		err  error
	)
	q := state.Profile.Queue
	switch tier {
	case "CHALLENGER":
		list, err = p.riot.GetChallengerLeagues(ctx, q)
	case "GRANDMASTER":
		list, err = p.riot.GetGrandmasterLeagues(ctx, q)
	case "MASTER":
		list, err = p.riot.GetMasterLeagues(ctx, q)
	}
	if err != nil {
		return err
	}

	for _, item := range list.Entries {
		if err := p.upsertPlayer(ctx, state, item.Puuid, list.LeagueID, tier, item.Rank, item); err != nil {
			return err
		}
	}
	slog.Info("synced top tier", "tier", tier, "count", len(list.Entries))
	return nil
}

func (p *Phase) syncDivisionTier(ctx context.Context, state *crawler.RunState, tier string) error {
	total := 0
	for _, div := range divisions {
		count := 0
		for page := 1; ; page++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			entries, err := p.riot.GetLeagueEntries(ctx, state.Profile.Queue, tier, div, page)
			if err != nil {
				return err
			}
			for _, e := range entries {
				item := riotapi.LeagueItemDTO{
					Puuid:        e.Puuid,
					LeagueID:     e.LeagueID,
					LeaguePoints: e.LeaguePoints,
					Rank:         e.Rank,
					Wins:         e.Wins,
					Losses:       e.Losses,
					Veteran:      e.Veteran,
					Inactive:     e.Inactive,
					FreshBlood:   e.FreshBlood,
					HotStreak:    e.HotStreak,
				}
				if err := p.upsertPlayer(ctx, state, e.Puuid, e.LeagueID, tier, div, item); err != nil {
					return err
				}
			}
			count += len(entries)
			total += len(entries)
			if len(entries) < 205 { // Riot returns up to 205 per page
				break
			}
		}
		slog.Info("synced division", "tier", tier, "division", div, "count", count)
	}
	slog.Info("synced tier", "tier", tier, "count", total)
	return nil
}

func (p *Phase) upsertPlayer(ctx context.Context, state *crawler.RunState, puuid, leagueID, tier, division string, item riotapi.LeagueItemDTO) error {
	if err := p.store.UpsertPlayer(ctx, puuid, state.Region(), nil, nil); err != nil {
		return err
	}

	leagueIDPtr := &leagueID
	var divPtr *string
	if division != "" {
		divPtr = &division
	}

	snap := &storage.RankSnapshot{
		RunID:        &state.ID,
		PUUID:        puuid,
		Region:       state.Region(),
		Source:       "phase1",
		LeagueID:     leagueIDPtr,
		Queue:        state.Profile.Queue,
		Tier:         tier,
		Division:     divPtr,
		LeaguePoints: &item.LeaguePoints,
		Wins:         &item.Wins,
		Losses:       &item.Losses,
		Veteran:      &item.Veteran,
		Inactive:     &item.Inactive,
		FreshBlood:   &item.FreshBlood,
		HotStreak:    &item.HotStreak,
		RankStatus:   "active",
	}
	return p.store.InsertSnapshot(ctx, snap)
}
