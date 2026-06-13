// Package phase0 syncs all game versions from CommunityDragon.
package phase0

import (
	"context"
	"log/slog"
	"time"

	"github.com/crafff/gogg/internal/crawler"
	"github.com/crafff/gogg/internal/riotapi"
	"github.com/crafff/gogg/internal/storage"
)

type Phase struct {
	riot  *riotapi.Client
	store *storage.Store
}

func New(riot *riotapi.Client, store *storage.Store) *Phase {
	return &Phase{riot: riot, store: store}
}

func (p *Phase) ID() int       { return 0 }
func (p *Phase) Name() string  { return "Phase0:VersionSync" }

func (p *Phase) IsDone(_ context.Context, state *crawler.RunState) (bool, error) {
	return state.Profile == nil, nil
}

func (p *Phase) Run(ctx context.Context, state *crawler.RunState) error {
	entries, err := p.riot.GetAllVersions(ctx)
	if err != nil {
		return err
	}
	slog.Info("fetched versions from CommunityDragon", "count", len(entries))

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

	slog.Info("latest game version", "version", latest.Version, "patch_start_at", latest.PatchStartAt)

	if state.Profile.Version == "" {
		// First execution: pin the resolved version so phase2 uses a fixed window.
		if err := p.store.UpdateRunVersion(ctx, state.ID, latest.Version); err != nil {
			return err
		}
		state.Profile.Version = latest.Version
		slog.Info("phase0: pinned version for this run", "version", latest.Version)
	} else if latest.Version != state.Profile.Version {
		// Resume after a new patch dropped: warn but keep the stored version.
		slog.Warn("phase0: new game version available; continuing this run with original version — start a new run to collect data for the new patch",
			"run_version", state.Profile.Version, "latest", latest.Version)
	}
	return nil
}
