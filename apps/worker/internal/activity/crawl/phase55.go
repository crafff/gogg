package crawl

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/crafff/gogg/apps/worker/internal/crawler/phase55"
)

// Phase55Input drives the legacy phase55.Phase.Run. CDragon catalog
// fetch is a pure HTTP call (no Riot key needed) — phase 5.5 itself
// doesn't require a *riotapi.Client, only a Store.
type Phase55Input struct {
	RunID        int       `json:"run_id"`
	Region       string    `json:"region"`
	Version      string    `json:"version"`
	RunStartedAt time.Time `json:"run_started_at"`
}

// Phase55Output mirrors the empty-output convention.
type Phase55Output struct{}

// Phase55ItemClassify wraps internal/crawler/phase55.Phase.Run. The
// inner loop is DB-heavy (decodes ItemEvent rows, runs classify(),
// writes completed/starter/boots tables) and emits per-batch progress
// logs; we heartbeat once on entry as elsewhere in this chunk.
func (a *Activities) Phase55ItemClassify(ctx context.Context, in Phase55Input) (Phase55Output, error) {
	logger := activity.GetLogger(ctx)
	if in.Region == "" {
		return Phase55Output{}, fmt.Errorf("phase55: region required")
	}
	if in.Version == "" {
		return Phase55Output{}, fmt.Errorf("phase55: version required")
	}

	state := a.synthState(in.RunID, in.Region, in.Version, "",
		nil, "", in.RunStartedAt, time.Time{})

	activity.RecordHeartbeat(ctx, "phase55_starting")
	logger.Info("phase55_started",
		"run_id", in.RunID,
		"region", in.Region,
		"version", in.Version,
	)

	p := phase55.New(a.rt.Store)
	if err := p.Run(ctx, state); err != nil {
		return Phase55Output{}, fmt.Errorf("phase55 run: %w", err)
	}
	logger.Info("phase55_completed",
		"run_id", in.RunID,
		"region", in.Region,
		"version", in.Version,
	)
	return Phase55Output{}, nil
}
