package crawl

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/crafff/gogg/apps/worker/internal/storage"
)

// CreateRunInput is the Workflow's bookkeeping seed for a CrawlRegion
// execution. Mirrors RunProfile fields the legacy code persists in the
// runs row; the workflow assembles this from its own input + the
// resolved version returned by Phase 0.
type CreateRunInput struct {
	ProfileName       string   `json:"profile_name,omitempty"`
	Region            string   `json:"region"`
	Mode              string   `json:"mode"`
	TargetTiers       []string `json:"target_tiers"`
	RankPrefetchTiers []string `json:"rank_prefetch_tiers"`
	Queue             string   `json:"queue"`
	Execution         string   `json:"execution"`
	Version           string   `json:"version,omitempty"`
	// LastRunEndUnix is seconds-since-epoch so JSON survives round-trip
	// without timezone fuzz. Zero means "no previous run".
	LastRunEndUnix int64 `json:"last_run_end_unix,omitempty"`
}

// CreateRunOutput is what the workflow threads through to subsequent
// activities.
type CreateRunOutput struct {
	RunID     int       `json:"run_id"`
	StartedAt time.Time `json:"started_at"`
}

// CreateRun inserts the runs row that subsequent phase activities
// stamp via current_phase/current_tier checkpoints. Keeps the legacy
// audit trail intact while Temporal owns recovery.
func (a *Activities) CreateRun(ctx context.Context, in CreateRunInput) (CreateRunOutput, error) {
	logger := activity.GetLogger(ctx)

	var profileName *string
	if in.ProfileName != "" {
		n := in.ProfileName
		profileName = &n
	}
	var version *string
	if in.Version != "" {
		v := in.Version
		version = &v
	}
	rp := storage.RunProfile{
		Mode:              in.Mode,
		TargetTiers:       in.TargetTiers,
		RankPrefetchTiers: in.RankPrefetchTiers,
		Queue:             in.Queue,
		Execution:         in.Execution,
		Version:           version,
		Region:            in.Region,
	}

	lastRunEnd := time.Time{}
	if in.LastRunEndUnix > 0 {
		lastRunEnd = time.Unix(in.LastRunEndUnix, 0)
	}

	id, startedAt, err := a.rt.Store.CreateRun(ctx, profileName, rp, lastRunEnd)
	if err != nil {
		return CreateRunOutput{}, fmt.Errorf("create run: %w", err)
	}
	logger.Info("run created", "run_id", id, "region", in.Region, "mode", in.Mode)
	return CreateRunOutput{RunID: id, StartedAt: startedAt}, nil
}

// PinRunVersionInput threads the Phase 0 result into the runs row so
// reporting tools that query runs.version keep showing the patch each
// run targeted.
type PinRunVersionInput struct {
	RunID   int    `json:"run_id"`
	Version string `json:"version"`
}

// PinRunVersion updates runs.version. Idempotent — safe to retry.
func (a *Activities) PinRunVersion(ctx context.Context, in PinRunVersionInput) error {
	if in.Version == "" {
		return fmt.Errorf("version must be set")
	}
	if err := a.rt.Store.UpdateRunVersion(ctx, in.RunID, in.Version); err != nil {
		return fmt.Errorf("pin run version: %w", err)
	}
	return nil
}

// CompleteRun marks the runs row done. Idempotent; safe to retry.
func (a *Activities) CompleteRun(ctx context.Context, runID int) error {
	if err := a.rt.Store.CompleteRun(ctx, runID); err != nil {
		return fmt.Errorf("complete run %d: %w", runID, err)
	}
	return nil
}

// FailRun is invoked from the Workflow's failure path. Best-effort —
// surfaces the error to the workflow so the failure event captures it,
// but we tolerate transient DB problems rather than masking the
// original Activity error.
func (a *Activities) FailRun(ctx context.Context, runID int) error {
	if err := a.rt.Store.FailRun(ctx, runID); err != nil {
		return fmt.Errorf("fail run %d: %w", runID, err)
	}
	return nil
}
