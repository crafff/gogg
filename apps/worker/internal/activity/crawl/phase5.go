package crawl

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/crafff/gogg/apps/worker/internal/crawler/phase5"
)

// Phase5Input drives the legacy phase5.Phase.Run. Like phase3 it's
// global — operates on every match in `matches` table that still needs
// a timeline for the run's pinned version.
type Phase5Input struct {
	RunID        int       `json:"run_id"`
	Region       string    `json:"region"`
	Version      string    `json:"version"`
	RunStartedAt time.Time `json:"run_started_at"`
}

// Phase5Output mirrors the empty-output convention of the other phase
// activities; legacy logging owns the count.
type Phase5Output struct{}

// Phase5Timeline wraps internal/crawler/phase5.Phase.Run. The Riot
// timeline endpoint returns multi-MB payloads so the legacy code keeps
// batch size at 200; the outer Activity timeout (4h) covers a full KR
// daily walk and the workflow's RetryPolicy bumps that for stuck
// runs.
func (a *Activities) Phase5Timeline(ctx context.Context, in Phase5Input) (Phase5Output, error) {
	logger := activity.GetLogger(ctx)
	if in.Version == "" {
		return Phase5Output{}, fmt.Errorf("phase5: version required")
	}
	if in.Region == "" {
		return Phase5Output{}, fmt.Errorf("phase5: region required")
	}

	riot, err := a.rt.RiotForRegion(in.Region)
	if err != nil {
		return Phase5Output{}, err
	}

	state := a.synthState(in.RunID, in.Region, in.Version, "",
		nil, "", in.RunStartedAt, time.Time{})

	activity.RecordHeartbeat(ctx, "phase5_starting")
	logger.Info("phase5 starting", "region", in.Region, "version", in.Version)

	p := phase5.New(riot, a.rt.Store)
	if err := p.Run(ctx, state); err != nil {
		return Phase5Output{}, fmt.Errorf("phase5 run: %w", err)
	}
	logger.Info("phase5 done")
	return Phase5Output{}, nil
}
