package crawl

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/crafff/gogg/internal/crawler/phase2"
)

// Phase2Input drives the legacy phase2.Phase.Run. Tiers is the slice
// of TargetTiers to filter active PUUIDs on; the legacy code reads
// state.CurrentTier first (single-tier) and falls back to
// state.Profile.TargetTiers (all-tiers). To reuse that, the Activity
// sets CurrentTier when Tiers has length 1 and leaves it empty
// otherwise — mirrors PipelineStrategy vs SequentialStrategy.
type Phase2Input struct {
	RunID          int       `json:"run_id"`
	Region         string    `json:"region"`
	Version        string    `json:"version"`
	Queue          string    `json:"queue"`
	Tiers          []string  `json:"tiers"`
	RunStartedAt   time.Time `json:"run_started_at"`
	LastRunEndUnix int64     `json:"last_run_end_unix,omitempty"`
}

// Phase2Output is intentionally minimal — phase2 itself returns no
// counts, only writes. The activity infers nothing extra; the count is
// observable via the match_ids table.
type Phase2Output struct {
	Tiers []string `json:"tiers"`
}

// Phase2MatchIDCollection wraps internal/crawler/phase2.Phase.Run. The
// long pagination loops inside phase2 don't heartbeat themselves; the
// Activity heartbeats once on entry so the workflow's heartbeat
// timeout doesn't trip during the initial puuid fetch. Subsequent
// heartbeats would require legacy edits; chunk 4 will lift this
// limitation when we move puuid iteration into the Activity.
func (a *Activities) Phase2MatchIDCollection(ctx context.Context, in Phase2Input) (Phase2Output, error) {
	logger := activity.GetLogger(ctx)
	if in.Version == "" {
		return Phase2Output{}, fmt.Errorf("phase2: version required")
	}
	if in.Region == "" {
		return Phase2Output{}, fmt.Errorf("phase2: region required")
	}
	if len(in.Tiers) == 0 {
		return Phase2Output{}, fmt.Errorf("phase2: at least one tier required")
	}

	riot, err := a.rt.RiotForRegion(in.Region)
	if err != nil {
		return Phase2Output{}, err
	}

	currentTier := ""
	if len(in.Tiers) == 1 {
		currentTier = in.Tiers[0]
	}
	lastRunEnd := time.Time{}
	if in.LastRunEndUnix > 0 {
		lastRunEnd = time.Unix(in.LastRunEndUnix, 0)
	}
	state := a.synthState(in.RunID, in.Region, in.Version, in.Queue,
		in.Tiers, currentTier, in.RunStartedAt, lastRunEnd)

	activity.RecordHeartbeat(ctx, in.Tiers)
	logger.Info("phase2 starting",
		"region", in.Region, "version", in.Version,
		"tiers", in.Tiers, "current_tier", currentTier,
	)
	p := phase2.New(riot, a.rt.Store)
	if err := p.Run(ctx, state); err != nil {
		return Phase2Output{}, fmt.Errorf("phase2 run: %w", err)
	}
	logger.Info("phase2 done", "tiers", in.Tiers)
	return Phase2Output{Tiers: in.Tiers}, nil
}
