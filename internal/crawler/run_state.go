package crawler

import (
	"context"
	"log/slog"
	"time"

	"github.com/crafff/gogg/internal/config"
	"github.com/crafff/gogg/internal/storage"
)

// RunState holds the mutable state of an active run and provides checkpoint helpers.
type RunState struct {
	ID          int
	Profile     *config.RunProfile
	StartedAt   time.Time
	LastRunEnd  time.Time
	CurrentTier string       // set by PipelineStrategy before each per-tier phase group
	donePhases  map[int]bool // set on resume: phase IDs that completed before the checkpoint
	store       *storage.Store
}

// NewRunState creates a brand-new run in the DB and returns its state.
func NewRunState(ctx context.Context, store *storage.Store, profileName *string, profile *config.RunProfile, lastRunEnd time.Time) (*RunState, error) {
	var version *string
	if profile.Version != "" {
		version = &profile.Version
	}
	sp := storage.RunProfile{
		Mode:              string(profile.Mode),
		TargetTiers:       profile.TargetTiers,
		RankPrefetchTiers: profile.RankPrefetchTiers,
		Queue:             profile.Queue,
		Execution:         string(profile.Execution),
		Version:           version,
		Region:            profile.Region,
	}
	id, startedAt, err := store.CreateRun(ctx, profileName, sp, lastRunEnd)
	if err != nil {
		return nil, err
	}
	slog.Info("run created", "run_id", id, "mode", profile.Mode, "tiers", profile.TargetTiers)
	return &RunState{ID: id, Profile: profile, StartedAt: startedAt, LastRunEnd: lastRunEnd, store: store}, nil
}

// ResumeRunState wraps an existing run record into a RunState.
// executionOrder is the ordered list of phase IDs from the runner (e.g. [0,1,2,3,35,4]).
// Phases that appear before checkpointPhase in that order are marked as done and will be skipped.
func ResumeRunState(run *storage.Run, profile *config.RunProfile, store *storage.Store, executionOrder []int) *RunState {
	done := make(map[int]bool)
	for _, id := range executionOrder {
		if id == run.CurrentPhase {
			break // stop before the checkpoint phase — it needs to re-run
		}
		done[id] = true
	}
	return &RunState{
		ID:         run.ID,
		Profile:    profile,
		StartedAt:  run.StartedAt,
		LastRunEnd: run.LastRunEnd,
		donePhases: done,
		store:      store,
	}
}

// Region returns the region this run is crawling.
func (rs *RunState) Region() string { return rs.Profile.Region }

// SaveCheckpoint persists the current phase/tier to the DB.
func (rs *RunState) SaveCheckpoint(ctx context.Context, phase int, tier *string) error {
	return rs.store.UpdateCheckpoint(ctx, rs.ID, phase, tier)
}


// Complete marks the run as successfully finished.
func (rs *RunState) Complete(ctx context.Context) error {
	slog.Info("run completed", "run_id", rs.ID)
	return rs.store.CompleteRun(ctx, rs.ID)
}

// Fail marks the run as failed.
func (rs *RunState) Fail(ctx context.Context) error {
	return rs.store.FailRun(ctx, rs.ID)
}
