// Package phase2 collects match IDs for all active players.
package phase2

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/crawler"
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
		slog.Warn("phase2: no active puuids found for this run")
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

	slog.Info("phase2: collecting match IDs",
		"puuids", len(puuids),
		"patch_start", bounds.PatchStart.Format(time.RFC3339),
		"end_time", endTime.Format(time.RFC3339),
	)

	for i, puuid := range puuids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if i%100 == 0 {
			slog.Info("phase2: progress", "processed", i, "total", len(puuids))
		}
		if err := p.collectForPlayer(ctx, state.Region(), state, puuid, bounds.PatchStart, endTime); err != nil {
			slog.Warn("phase2: failed to collect matches for player", "puuid", puuid, "err", err)
		}
	}
	return nil
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

		ids, err := p.riot.GetMatchIDsByPUUID(ctx, puuid, 420, startTime.Unix(), endTime.Unix(), start, pageSize)
		if err != nil {
			return err
		}

		collected = append(collected, ids...)

		if len(ids) < pageSize {
			break
		}
	}

	// Write match IDs (idempotent).
	for _, id := range collected {
		if err := p.store.UpsertMatchID(ctx, id, region, state.Profile.Version); err != nil {
			return err
		}
	}

	// Update sync time only after all IDs are written.
	if err := p.store.SetPlayerSyncTime(ctx, puuid, region, time.Now()); err != nil {
		return err
	}

	if len(collected) > 0 {
		slog.Debug("phase2: collected match IDs", "puuid", puuid[:8], "count", len(collected))
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
