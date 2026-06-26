package crawl

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/crafff/gogg/packages/riotapi"
)

// Phase0Input is the workflow → Activity payload. Region picks the
// Riot client even though CommunityDragon (the data source) is global;
// this keeps the call symmetric with later phases that actually need
// per-region routing.
type Phase0Input struct {
	Region string `json:"region"`
	// PinnedVersion mirrors RunProfile.Version: when non-empty the
	// activity still syncs game_versions but the workflow keeps its
	// pinned version untouched. Empty means "resolve latest and the
	// workflow will pin it".
	PinnedVersion string `json:"pinned_version,omitempty"`
}

// Phase0Output is what the workflow needs to thread to subsequent
// phases. ResolvedVersion equals PinnedVersion when that was supplied,
// otherwise the latest patch surfaced by CommunityDragon.
type Phase0Output struct {
	ResolvedVersion string    `json:"resolved_version"`
	LatestVersion   string    `json:"latest_version"`
	UpsertedCount   int       `json:"upserted_count"`
	FetchedAt       time.Time `json:"fetched_at"`
}

// Phase0VersionSync mirrors internal/crawler/phase0.Phase.Run: fetch
// every patch from CommunityDragon, upsert each into game_versions,
// then sync the is_latest flag. The legacy version mutated a
// RunState; here we return the resolved version and let the workflow
// hold that state.
func (a *Activities) Phase0VersionSync(ctx context.Context, in Phase0Input) (Phase0Output, error) {
	logger := activity.GetLogger(ctx)

	riot, err := a.rt.RiotForRegion(in.Region)
	if err != nil {
		return Phase0Output{}, err
	}

	entries, err := riot.GetAllVersions(ctx)
	if err != nil {
		return Phase0Output{}, fmt.Errorf("get all versions: %w", err)
	}
	logger.Info("phase0_versions_fetched",
		"region", in.Region,
		"count", len(entries),
	)

	var latest riotapi.VersionEntry
	for _, e := range entries {
		if e.PatchStartAt.After(latest.PatchStartAt) {
			latest = e
		}
	}

	pool := a.rt.Store.Pool
	now := time.Now()

	for i, e := range entries {
		// sqlc-skip: 1:1 with internal/crawler/phase0.Run; trivial
		// static INSERT, will move under sqlc in the same chunk that
		// retires internal/crawler entirely.
		if _, err := pool.Exec(ctx, `
			INSERT INTO game_versions (version, fetched_at, patch_start_at, is_latest)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (version) DO NOTHING`,
			e.Version, now, e.PatchStartAt, e.Version == latest.Version,
		); err != nil {
			return Phase0Output{}, fmt.Errorf("upsert version %s: %w", e.Version, err)
		}
		// Heartbeat every 25 versions so the UI shows progress on
		// the rare cold-start path where CDragon returns hundreds.
		if i%25 == 0 {
			activity.RecordHeartbeat(ctx, i)
		}
	}

	// sqlc-skip: same justification as above.
	if _, err := pool.Exec(ctx, `UPDATE game_versions SET is_latest = false`); err != nil {
		return Phase0Output{}, fmt.Errorf("reset is_latest: %w", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE game_versions SET is_latest = true WHERE version = $1`, latest.Version,
	); err != nil {
		return Phase0Output{}, fmt.Errorf("set is_latest=true on %s: %w", latest.Version, err)
	}

	resolved := in.PinnedVersion
	if resolved == "" {
		resolved = latest.Version
	}
	logger.Info("phase0_completed",
		"region", in.Region,
		"latest_version", latest.Version,
		"resolved_version", resolved,
		"upserted_count", len(entries),
	)
	return Phase0Output{
		ResolvedVersion: resolved,
		LatestVersion:   latest.Version,
		UpsertedCount:   len(entries),
		FetchedAt:       now,
	}, nil
}
