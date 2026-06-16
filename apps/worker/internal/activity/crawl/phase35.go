package crawl

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/crafff/gogg/internal/crawler/phase35"
)

// Phase35Input drives the legacy phase35.Phase.Run. Queue must match
// the profile's queue — phase 3.5 only persists snapshots for entries
// whose queueType matches the profile.
type Phase35Input struct {
	RunID        int       `json:"run_id"`
	Region       string    `json:"region"`
	Queue        string    `json:"queue"`
	RunStartedAt time.Time `json:"run_started_at"`
}

// Phase35Output is intentionally empty — legacy logging owns counts.
type Phase35Output struct{}

// Phase35OnDemandRank wraps internal/crawler/phase35.Phase.Run. The
// inner loop pages 1000 puuids at a time so heartbeats once on entry
// keeps the workflow happy; an interrupted Activity gets retried from
// scratch which is safe because legacy MarkParticipantUnranked is
// idempotent.
func (a *Activities) Phase35OnDemandRank(ctx context.Context, in Phase35Input) (Phase35Output, error) {
	logger := activity.GetLogger(ctx)
	if in.Region == "" {
		return Phase35Output{}, fmt.Errorf("phase35: region required")
	}
	if in.Queue == "" {
		return Phase35Output{}, fmt.Errorf("phase35: queue required")
	}

	riot, err := a.rt.RiotForRegion(in.Region)
	if err != nil {
		return Phase35Output{}, err
	}

	state := a.synthState(in.RunID, in.Region, "", in.Queue,
		nil, "", in.RunStartedAt, time.Time{})

	activity.RecordHeartbeat(ctx, "phase35_starting")
	logger.Info("phase35 starting", "region", in.Region, "queue", in.Queue)

	p := phase35.New(riot, a.rt.Store)
	if err := p.Run(ctx, state); err != nil {
		return Phase35Output{}, fmt.Errorf("phase35 run: %w", err)
	}
	logger.Info("phase35 done")
	return Phase35Output{}, nil
}
