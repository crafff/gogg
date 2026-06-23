package crawl

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/crafff/gogg/apps/worker/internal/crawler/phase4"
)

// Phase4Input drives the legacy phase4.Phase.Run. No Riot client
// needed — phase 4 is DB-only (apex threshold lookup + per-match
// avg_tier compute). Heartbeat budget can be shorter as a result.
type Phase4Input struct {
	RunID        int       `json:"run_id"`
	Region       string    `json:"region"`
	RunStartedAt time.Time `json:"run_started_at"`
}

// Phase4Output is empty for parity with the other phase activities.
type Phase4Output struct{}

// Phase4AvgTierCalc wraps internal/crawler/phase4.Phase.Run. The inner
// computeAndStore loop is purely SQL + arithmetic so it advances fast;
// even a cold KR daily-run finishes in seconds.
func (a *Activities) Phase4AvgTierCalc(ctx context.Context, in Phase4Input) (Phase4Output, error) {
	logger := activity.GetLogger(ctx)
	if in.Region == "" {
		return Phase4Output{}, fmt.Errorf("phase4: region required")
	}

	state := a.synthState(in.RunID, in.Region, "", "",
		nil, "", in.RunStartedAt, time.Time{})

	activity.RecordHeartbeat(ctx, "phase4_starting")
	logger.Info("phase4 starting", "region", in.Region)

	p := phase4.New(a.rt.Store)
	if err := p.Run(ctx, state); err != nil {
		return Phase4Output{}, fmt.Errorf("phase4 run: %w", err)
	}
	logger.Info("phase4 done")
	return Phase4Output{}, nil
}
