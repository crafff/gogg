// Package phase4 computes avg_tier_score, avg_tier and avg_division for matches.
package phase4

import (
	"context"
	"log/slog"
	"strings"

	"github.com/crafff/gogg/internal/crawler"
	"github.com/crafff/gogg/internal/storage"
)

// Tier score encoding: tier_base + division_bonus + lp
var tierBase = map[string]int{
	"IRON": 0, "BRONZE": 400, "SILVER": 800, "GOLD": 1200,
	"PLATINUM": 1600, "EMERALD": 2000, "DIAMOND": 2400,
	"MASTER": 2800, "GRANDMASTER": 2800, "CHALLENGER": 2800,
}

var divisionBonus = map[string]int{
	"IV": 0, "III": 100, "II": 200, "I": 300,
}

// tierOrder is used for score→tier conversion (highest first).
var tierOrder = []struct {
	name string
	base int
}{
	{"DIAMOND", 2400},
	{"EMERALD", 2000},
	{"PLATINUM", 1600},
	{"GOLD", 1200},
	{"SILVER", 800},
	{"BRONZE", 400},
	{"IRON", 0},
}

const masterBase = 2800

const batchSize = 2000

type Phase struct {
	store *storage.Store
}

func New(store *storage.Store) *Phase {
	return &Phase{store: store}
}

func (p *Phase) ID() int      { return 4 }
func (p *Phase) Name() string { return "Phase4:AvgTierCalc" }

func (p *Phase) IsDone(ctx context.Context, state *crawler.RunState) (bool, error) {
	ids, err := p.store.GetMatchesNeedingTierCalc(ctx, state.Region(), 1)
	if err != nil {
		return false, err
	}
	return len(ids) == 0, nil
}

func (p *Phase) Run(ctx context.Context, state *crawler.RunState) error {
	thresholds, err := p.store.GetApexThresholds(ctx, state.ID, state.Region())
	if err != nil {
		return err
	}
	slog.Info("phase4: apex thresholds",
		"challenger_min_score", thresholds.ChallengerMinScore,
		"grandmaster_min_score", thresholds.GrandmasterMinScore,
	)

	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		ids, err := p.store.GetMatchesNeedingTierCalc(ctx, state.Region(), batchSize)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		for _, matchID := range ids {
			if err := p.computeAndStore(ctx, matchID, thresholds); err != nil {
				slog.Warn("phase4: failed to compute tier", "match_id", matchID, "err", err)
			}
			total++
		}
		slog.Info("phase4: computed avg_tier", "total", total)
	}
	return nil
}

func (p *Phase) computeAndStore(ctx context.Context, matchID string, thresholds storage.ApexThresholds) error {
	rows, err := p.store.GetParticipantTiersForMatch(ctx, matchID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return p.store.UpdateAvgTier(ctx, matchID, "", "", 0, 0)
	}

	sum := 0
	for _, r := range rows {
		score := tierBase[strings.ToUpper(r.Tier)]
		if r.Division != nil {
			score += divisionBonus[strings.ToUpper(*r.Division)]
		}
		if r.LeaguePoints != nil {
			score += *r.LeaguePoints
		}
		sum += score
	}
	avg := sum / len(rows)
	tier, division := scoreToTier(avg, thresholds)
	return p.store.UpdateAvgTier(ctx, matchID, tier, division, avg, len(rows))
}

// scoreToTier converts a numeric score to (tier, division).
// Division is empty for MASTER / GRANDMASTER / CHALLENGER.
//
// Apex tiers:
//   - CHALLENGER  : score >= ChallengerMinScore  (min LP of Challenger players in this run)
//   - GRANDMASTER : score >= GrandmasterMinScore (min LP of Grandmaster players in this run)
//   - MASTER      : score >= masterBase (Diamond I + 100 LP equivalent)
//
// Diamond and below follow the standard tier-base + division-bonus encoding.
func scoreToTier(score int, t storage.ApexThresholds) (tier, division string) {
	switch {
	case score >= t.ChallengerMinScore:
		return "CHALLENGER", "I"
	case score >= t.GrandmasterMinScore:
		return "GRANDMASTER", "I"
	case score >= masterBase:
		return "MASTER", "I"
	}

	divNames := [4]string{"IV", "III", "II", "I"}
	for _, entry := range tierOrder {
		if score >= entry.base {
			offset := score - entry.base
			idx := offset / 100
			if idx > 3 {
				idx = 3
			}
			return entry.name, divNames[idx]
		}
	}
	return "IRON", "IV"
}