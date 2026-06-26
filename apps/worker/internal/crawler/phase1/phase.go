// Package phase1 syncs player rank data for configured tiers.
package phase1

import (
	"context"

	"github.com/crafff/gogg/apps/worker/internal/crawler"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phaselog"
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
	started := state.CurrentTier == ""
	for _, tier := range state.Profile.RankPrefetchTiers {
		if !started {
			if tier != state.CurrentTier {
				continue
			}
			started = true
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		phaselog.Step(phaseMeta(state, p, tier, ""), "tier_started")
		tierCopy := tier
		if err := state.SaveCheckpoint(ctx, 1, &tierCopy); err != nil {
			return err
		}
		if err := p.syncTier(ctx, state, tier); err != nil {
			return err
		}
		if tier == state.CurrentTier {
			state.CurrentTier = ""
			state.CurrentDivision = ""
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
	phaselog.Completed(phaseMeta(state, p, tier, ""), "scope", "tier", "count", len(list.Entries))
	return nil
}

func (p *Phase) syncDivisionTier(ctx context.Context, state *crawler.RunState, tier string) error {
	total := 0
	started := state.CurrentDivision == "" || state.CurrentTier != tier
	for _, div := range divisions {
		if !started {
			if div != state.CurrentDivision {
				continue
			}
			started = true
		}
		tierCopy := tier
		divCopy := div
		if err := state.SaveCheckpointDetail(ctx, 1, &tierCopy, &divCopy); err != nil {
			return err
		}
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
		phaselog.Completed(phaseMeta(state, p, tier, div), "scope", "division", "count", count)
	}
	phaselog.Completed(phaseMeta(state, p, tier, ""), "scope", "tier", "count", total)
	return nil
}

func phaseMeta(state *crawler.RunState, p *Phase, tier, division string) phaselog.Meta {
	return phaselog.Meta{
		RunID:    state.ID,
		Region:   state.Region(),
		Phase:    p.Name(),
		PhaseID:  p.ID(),
		Version:  state.Profile.Version,
		Tier:     tier,
		Division: division,
		Queue:    state.Profile.Queue,
	}
}

func (p *Phase) upsertPlayer(ctx context.Context, state *crawler.RunState, puuid, leagueID, tier, division string, item riotapi.LeagueItemDTO) error {
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
	return p.store.SavePlayerRankSnapshot(ctx, puuid, state.Region(), nil, nil, snap)
}
