// Package phase0 syncs all game versions from CommunityDragon.
package phase0

import (
	"context"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/crawler"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phaselog"
	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
)

type Phase struct {
	riot  *riotapi.Client
	store *storage.Store
}

func New(riot *riotapi.Client, store *storage.Store) *Phase {
	return &Phase{riot: riot, store: store}
}

func (p *Phase) ID() int      { return 0 }
func (p *Phase) Name() string { return "Phase0:VersionSync" }

func (p *Phase) IsDone(_ context.Context, state *crawler.RunState) (bool, error) {
	return state.Profile == nil, nil
}

func (p *Phase) Run(ctx context.Context, state *crawler.RunState) error {
	meta := phaselog.Meta{RunID: state.ID, Region: state.Region(), Phase: p.Name(), PhaseID: p.ID()}
	entries, err := p.riot.GetAllVersions(ctx)
	if err != nil {
		return err
	}
	phaselog.Step(meta, "versions_fetched", "count", len(entries))

	// Find latest: entry with the most recent patch_start_at.
	var latest riotapi.VersionEntry
	for _, e := range entries {
		if e.PatchStartAt.After(latest.PatchStartAt) {
			latest = e
		}
	}

	// Insert new versions; skip ones already present.
	now := time.Now()
	for _, e := range entries {
		_, err := p.store.Pool.Exec(ctx, `
			INSERT INTO game_versions (version, fetched_at, patch_start_at, is_latest)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (version) DO NOTHING`,
			e.Version, now, e.PatchStartAt, e.Version == latest.Version)
		if err != nil {
			return err
		}
	}

	// Sync is_latest flag: clear all, then set the current latest.
	if _, err := p.store.Pool.Exec(ctx, `UPDATE game_versions SET is_latest = false`); err != nil {
		return err
	}
	_, err = p.store.Pool.Exec(ctx,
		`UPDATE game_versions SET is_latest = true WHERE version = $1`, latest.Version)
	if err != nil {
		return err
	}

	phaselog.Step(meta, "latest_version_resolved", "latest_version", latest.Version, "patch_start_at", latest.PatchStartAt)

	if state.Profile.Version == "" {
		// First execution: pin the resolved version so phase2 uses a fixed window.
		if err := p.store.UpdateRunVersion(ctx, state.ID, latest.Version); err != nil {
			return err
		}
		state.Profile.Version = latest.Version
		phaselog.Step(phaselog.Meta{RunID: state.ID, Region: state.Region(), Phase: p.Name(), PhaseID: p.ID(), Version: latest.Version}, "version_pinned")
	} else if latest.Version != state.Profile.Version {
		// Resume after a new patch dropped: warn but keep the stored version.
		phaselog.Warn(phaselog.Meta{RunID: state.ID, Region: state.Region(), Phase: p.Name(), PhaseID: p.ID(), Version: state.Profile.Version}, "newer_version_available", "latest_version", latest.Version)
	}
	return nil
}
