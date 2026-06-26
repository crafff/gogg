// Package phase2 collects match IDs for all active players.
package phase2

import (
	"context"
	"strings"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/crawler"
	"github.com/crafff/gogg/apps/worker/internal/crawler/heartbeat"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phaselog"
	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
)

const pageSize = 100

type Phase struct {
	riot  *riotapi.Client
	store *storage.Store
}

func New(riot *riotapi.Client, store *storage.Store) *Phase {
	return &Phase{riot: riot, store: store}
}

func (p *Phase) ID() int      { return 2 }
func (p *Phase) Name() string { return "Phase2:MatchIDCollection" }

func (p *Phase) IsDone(_ context.Context, _ *crawler.RunState) (bool, error) {
	return false, nil
}

func (p *Phase) Run(ctx context.Context, state *crawler.RunState) error {
	// In pipeline mode CurrentTier is set to the single tier being processed.
	// In sequential mode it is empty, so we process all target tiers at once.
	tiers := state.Profile.TargetTiers
	if state.CurrentTier != "" {
		tiers = []string{state.CurrentTier}
	}
	puuids, err := p.store.GetActivePUUIDsByTiers(ctx, state.ID, upperAll(tiers))
	if err != nil {
		return err
	}
	if len(puuids) == 0 {
		phaselog.Warn(phaseMeta(state, p, tiers), "no_active_puuids")
		return nil
	}

	// Resolve the time window for the target patch version.
	bounds, err := p.store.GetVersionBoundaries(ctx, state.Profile.Version)
	if err != nil {
		return err
	}
	// Upper bound: next version's patch start, or this run's started_at if latest.
	// runs.version is always pinned by phase0, so this window is stable across resumes.
	endTime := state.StartedAt
	if bounds.PatchEnd != nil {
		endTime = *bounds.PatchEnd
	}

	meta := phaseMeta(state, p, tiers)
	phaselog.Step(meta,
		"match_collection_started",
		"puuids", len(puuids),
		"patch_start", bounds.PatchStart.Format(time.RFC3339),
		"end_time", endTime.Format(time.RFC3339),
	)

	start := time.Now()
	for i, puuid := range puuids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if i%25 == 0 {
			heartbeat.Record(ctx, map[string]any{
				"run_id":    state.ID,
				"region":    state.Region(),
				"version":   state.Profile.Version,
				"tiers":     tiers,
				"processed": i,
				"total":     len(puuids),
			})
		}
		if i%100 == 0 {
			phaselog.Progress(meta, i, len(puuids), 0, start)
		}
		if err := p.collectForPlayer(ctx, state.Region(), state, puuid, bounds.PatchStart, endTime); err != nil {
			phaselog.Warn(meta, "player_failed", "puuid", puuid, "err", err)
		}
	}
	return nil
}

func phaseMeta(state *crawler.RunState, p *Phase, tiers []string) phaselog.Meta {
	tier := ""
	if len(tiers) == 1 {
		tier = tiers[0]
	}
	return phaselog.Meta{
		RunID:   state.ID,
		Region:  state.Region(),
		Phase:   p.Name(),
		PhaseID: p.ID(),
		Version: state.Profile.Version,
		Tier:    tier,
		Queue:   state.Profile.Queue,
	}
}

func (p *Phase) collectForPlayer(ctx context.Context, region string, state *crawler.RunState, puuid string, startTime, endTime time.Time) error {
	lastSync, err := p.store.GetPlayerSyncTime(ctx, puuid, region)
	if err != nil {
		return err
	}

	// Skip players already synced during this run (resume safety).
	if !lastSync.IsZero() && !lastSync.Before(state.StartedAt) {
		return nil
	}

	var collected []string

	for start := 0; ; start += pageSize {
		if err := ctx.Err(); err != nil {
			return err
		}
		heartbeat.Record(ctx, map[string]any{
			"run_id":       state.ID,
			"region":       region,
			"version":      state.Profile.Version,
			"puuid_prefix": puuid[:8],
			"start":        start,
			"page_size":    pageSize,
		})

		ids, err := p.riot.GetMatchIDsByPUUID(ctx, puuid, 420, startTime.Unix(), endTime.Unix(), start, pageSize)
		if err != nil {
			return err
		}

		collected = append(collected, ids...)

		if len(ids) < pageSize {
			break
		}
	}

	// Update sync time only after all IDs are written in the same transaction.
	if err := p.store.SavePlayerMatchIDs(ctx, puuid, region, state.Profile.Version, collected, time.Now()); err != nil {
		return err
	}

	if len(collected) > 0 {
		phaselog.DebugStep(phaseMeta(state, p, []string{state.CurrentTier}), "player_completed",
			"puuid_prefix", puuid[:8],
			"match_count", len(collected),
		)
	}
	return nil
}

func upperAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToUpper(s)
	}
	return out
}
