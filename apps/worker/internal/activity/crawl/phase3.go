package crawl

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/crafff/gogg/apps/worker/internal/crawler/phase3"
)

// Phase3Input drives the legacy phase3.Phase.Run. Global to the run
// — Phase 3 isn't tier-scoped because it iterates pending match IDs
// from any tier.
type Phase3Input struct {
	RunID        int       `json:"run_id"`
	Region       string    `json:"region"`
	Version      string    `json:"version"`
	RunStartedAt time.Time `json:"run_started_at"`
}

// Phase3Output mirrors what the legacy phase3 logs internally —
// counts surface nothing externally yet because the inner loop owns
// progress reporting via slog.
type Phase3Output struct{}

// Phase3MatchDetails wraps internal/crawler/phase3.Phase.Run. The
// inner code already paginates with batchSize=500, which fits within
// HeartbeatTimeout=2min comfortably for any reasonable Riot latency.
func (a *Activities) Phase3MatchDetails(ctx context.Context, in Phase3Input) (Phase3Output, error) {
	logger := activity.GetLogger(ctx)
	if in.Version == "" {
		return Phase3Output{}, fmt.Errorf("phase3: version required")
	}
	if in.Region == "" {
		return Phase3Output{}, fmt.Errorf("phase3: region required")
	}

	riot, err := a.rt.RiotForRegion(in.Region)
	if err != nil {
		return Phase3Output{}, err
	}

	state := a.synthState(in.RunID, in.Region, in.Version, "",
		nil, "", in.RunStartedAt, time.Time{})

	activity.RecordHeartbeat(ctx, "phase3_starting")
	logger.Info("phase3 starting", "region", in.Region, "version", in.Version)

	p := phase3.New(riot, a.rt.Store)
	if err := p.Run(ctx, state); err != nil {
		return Phase3Output{}, fmt.Errorf("phase3 run: %w", err)
	}
	logger.Info("phase3 done")
	return Phase3Output{}, nil
}
