package crawl

import (
	"time"

	"github.com/crafff/gogg/apps/worker/internal/crawler"
	"github.com/crafff/gogg/apps/worker/internal/crawlerconfig"
	"github.com/crafff/gogg/apps/worker/internal/storage"
)

// synthState builds a synthetic *crawler.RunState that the legacy
// Phase types accept. Workflows own the run lifecycle in Phase C, so
// the state's bookkeeping (donePhases, current_phase) is intentionally
// empty — Temporal replaces both. The only writes the legacy code
// performs against it that we want to keep are the `runs.current_tier`
// stamps via state.SaveCheckpoint(); those are harmless audit writes.
//
// run.LastRunEnd is loaded from the runs row on demand because phase2
// uses it to skip already-synced players. We accept it as an input so
// the Workflow can thread the actual value through without a DB
// round-trip.
func (a *Activities) synthState(
	runID int,
	region, version, queue string,
	targetTiers []string,
	currentTier string,
	startedAt time.Time,
	lastRunEnd time.Time,
) *crawler.RunState {
	run := &storage.Run{
		ID:                runID,
		Region:            region,
		StartedAt:         startedAt,
		LastRunEnd:        lastRunEnd,
		TargetTiers:       targetTiers,
		RankPrefetchTiers: nil,
		Queue:             queue,
	}
	profile := &config.RunProfile{
		Region:      region,
		Version:     version,
		Queue:       queue,
		TargetTiers: targetTiers,
	}
	state := crawler.ResumeRunState(run, profile, a.rt.Store, nil)
	state.CurrentTier = currentTier
	return state
}
