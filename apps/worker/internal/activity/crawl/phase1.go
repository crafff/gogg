package crawl

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
)

// Phase1Input mirrors the legacy phase1 inputs that come off RunState
// + Profile. Region+Queue+RankPrefetchTiers drive iteration; RunID is
// stamped on each snapshot so the legacy aggregation queries (which
// filter by run_id) keep working unchanged.
type Phase1Input struct {
	RunID             int      `json:"run_id"`
	Region            string   `json:"region"`
	Queue             string   `json:"queue"`
	RankPrefetchTiers []string `json:"rank_prefetch_tiers"`
	Division          string   `json:"division,omitempty"`
}

// Phase1Output reports per-tier counts back to the workflow for log /
// metric consumption.
type Phase1Output struct {
	TierCounts map[string]int `json:"tier_counts"`
}

var (
	// topTiers + divisions mirror internal/crawler/phase1's package
	// vars. Duplicated rather than imported because importing
	// crawler/phase1 would pull in the legacy RunState type — the
	// thing this chunk replaces.
	topTiers = map[string]bool{
		"CHALLENGER":  true,
		"GRANDMASTER": true,
		"MASTER":      true,
	}
	divisions = []string{"I", "II", "III", "IV"}
)

// Phase1RankSnapshot mirrors internal/crawler/phase1.Phase.Run for the
// configured RankPrefetchTiers. Each tier writes its checkpoint to the
// runs row before the API calls fire, so the legacy `runs.current_tier`
// audit column matches what the in-process runner used to write. The
// loop heartbeats per tier so cancellation is responsive even on the
// long division-tier paginations.
func (a *Activities) Phase1RankSnapshot(ctx context.Context, in Phase1Input) (Phase1Output, error) {
	logger := activity.GetLogger(ctx)

	riot, err := a.rt.RiotForRegion(in.Region)
	if err != nil {
		return Phase1Output{}, err
	}
	if in.Queue == "" {
		return Phase1Output{}, fmt.Errorf("queue must be set")
	}
	if len(in.RankPrefetchTiers) == 0 {
		return Phase1Output{}, fmt.Errorf("rank_prefetch_tiers must be non-empty")
	}

	counts := make(map[string]int, len(in.RankPrefetchTiers))
	for _, tier := range in.RankPrefetchTiers {
		if err := ctx.Err(); err != nil {
			return Phase1Output{}, err
		}
		activity.RecordHeartbeat(ctx, tier)
		tierCopy := tier
		if err := a.rt.Store.UpdateCheckpoint(ctx, in.RunID, 1, &tierCopy); err != nil {
			return Phase1Output{}, fmt.Errorf("checkpoint tier %s: %w", tier, err)
		}
		n, err := a.syncTier(ctx, riot, in, tier)
		if err != nil {
			return Phase1Output{}, fmt.Errorf("sync tier %s: %w", tier, err)
		}
		counts[tier] = n
		logDivision := in.Division
		if logDivision == "" {
			logDivision = "I"
		}
		logger.Info("phase1_tier_completed",
			"run_id", in.RunID,
			"region", in.Region,
			"queue", in.Queue,
			"tier", tier,
			"division", logDivision,
			"count", n,
		)
	}
	return Phase1Output{TierCounts: counts}, nil
}

// syncTier dispatches on top vs division tiers — same split the legacy
// phase1 makes. Returns the upserted count for telemetry.
func (a *Activities) syncTier(ctx context.Context, riot *riotapi.Client, in Phase1Input, tier string) (int, error) {
	if topTiers[tier] {
		return a.syncTopTier(ctx, riot, in, tier)
	}
	return a.syncDivisionTier(ctx, riot, in, tier)
}

func (a *Activities) syncTopTier(ctx context.Context, riot *riotapi.Client, in Phase1Input, tier string) (int, error) {
	var (
		list *riotapi.LeagueListDTO
		err  error
	)
	switch tier {
	case "CHALLENGER":
		list, err = riot.GetChallengerLeagues(ctx, in.Queue)
	case "GRANDMASTER":
		list, err = riot.GetGrandmasterLeagues(ctx, in.Queue)
	case "MASTER":
		list, err = riot.GetMasterLeagues(ctx, in.Queue)
	default:
		return 0, fmt.Errorf("syncTopTier: unsupported tier %q", tier)
	}
	if err != nil {
		return 0, err
	}
	for _, item := range list.Entries {
		if err := a.upsertSnapshot(ctx, in, item.Puuid, list.LeagueID, tier, item.Rank, item); err != nil {
			return 0, err
		}
	}
	return len(list.Entries), nil
}

func (a *Activities) syncDivisionTier(ctx context.Context, riot *riotapi.Client, in Phase1Input, tier string) (int, error) {
	total := 0
	targetDivisions := divisions
	if in.Division != "" {
		targetDivisions = []string{in.Division}
	}
	for _, div := range targetDivisions {
		for page := 1; ; page++ {
			if err := ctx.Err(); err != nil {
				return total, err
			}
			activity.RecordHeartbeat(ctx, map[string]any{
				"run_id":   in.RunID,
				"region":   in.Region,
				"queue":    in.Queue,
				"tier":     tier,
				"division": div,
				"page":     page,
			})
			entries, err := riot.GetLeagueEntries(ctx, in.Queue, tier, div, page)
			if err != nil {
				return total, err
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
				if err := a.upsertSnapshot(ctx, in, e.Puuid, e.LeagueID, tier, div, item); err != nil {
					return total, err
				}
			}
			total += len(entries)
			if len(entries) < 205 { // Riot caps division pages at 205.
				break
			}
		}
	}
	return total, nil
}

func (a *Activities) upsertSnapshot(ctx context.Context, in Phase1Input, puuid, leagueID, tier, division string, item riotapi.LeagueItemDTO) error {
	if err := a.rt.Store.UpsertPlayer(ctx, puuid, in.Region, nil, nil); err != nil {
		return err
	}
	leagueIDPtr := &leagueID
	var divPtr *string
	if division != "" {
		divPtr = &division
	}
	snap := &storage.RankSnapshot{
		RunID:        &in.RunID,
		PUUID:        puuid,
		Region:       in.Region,
		Source:       "phase1",
		LeagueID:     leagueIDPtr,
		Queue:        in.Queue,
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
	return a.rt.Store.InsertSnapshot(ctx, snap)
}
