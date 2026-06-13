package crawler

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"github.com/crafff/gogg/internal/config"
	"github.com/crafff/gogg/internal/riotapi"
	"github.com/crafff/gogg/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Runner orchestrates a single crawler run.
type Runner struct {
	store    *storage.Store
	riot     *riotapi.Client
	phases   []Phase
	strategy ExecutionStrategy
}

func NewRunner(store *storage.Store, riot *riotapi.Client, phases []Phase, strategy ExecutionStrategy) *Runner {
	return &Runner{store: store, riot: riot, phases: phases, strategy: strategy}
}

// Run executes the crawler with the given profile.
// If resumeID > 0, it resumes that specific run.
func (r *Runner) Run(ctx context.Context, profileName *string, profile *config.RunProfile, resumeID int) error {
	// 1. Acquire per-region advisory lock – one run per region at a time.
	lockConn, err := acquireAdvisoryLock(ctx, r.store.Pool, profile.Region)
	if err != nil {
		return err
	}
	defer lockConn.Release()

	// 2. Set up graceful shutdown.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 3. Create or resume RunState.
	var state *RunState
	if resumeID > 0 {
		found, err := r.store.GetRunByID(ctx, resumeID)
		if err != nil {
			return err
		}
		if found == nil {
			return fmt.Errorf("run %d not found", resumeID)
		}
		if found.Status != "running" && found.Status != "failed" {
			return fmt.Errorf("run %d has status=%q; only 'running' or 'failed' runs can be resumed", resumeID, found.Status)
		}
		if found.Status == "failed" {
			if err := r.store.ReactivateRun(ctx, resumeID); err != nil {
				return err
			}
			found.Status = "running"
		}
		// Phase0/1 are fast; always restart from phase0 for a clean slate.
		if found.CurrentPhase <= 1 {
			if err := r.store.ResetRunToPhase0(ctx, resumeID); err != nil {
				return err
			}
			found.CurrentPhase = 0
			found.CurrentTier = nil
			slog.Info("resuming run: phase <= 1, restarting from phase0", "run_id", resumeID)
		}
		var version string
		if found.Version != nil {
			version = *found.Version
		}
		restored := &config.RunProfile{
			Region:            found.Region,
			Mode:              config.Mode(found.Mode),
			TargetTiers:       found.TargetTiers,
			RankPrefetchTiers: found.RankPrefetchTiers,
			Queue:             found.Queue,
			Execution:         config.Execution(found.Execution),
			Version:           version,
		}
		restored.MergeFlags(profile.TargetTiers, profile.Mode, profile.Version, profile.Execution, profile.Region)
		_ = restored.Validate()
		slog.Info("resuming run", "run_id", resumeID, "from_phase", found.CurrentPhase, "tiers", restored.TargetTiers, "region", restored.Region)
		state = ResumeRunState(found, restored, r.store, phaseIDs(r.phases))
	} else {
		active, err := r.store.GetActiveRun(ctx, profile.Region)
		if err != nil {
			return err
		}
		if active != nil {
			return fmt.Errorf("run %d is already running in region %s (use --resume=%d or gogg crawl cancel %d)",
				active.ID, profile.Region, active.ID, active.ID)
		}

		lastRunEnd := r.store.GetLastCompletedRunEnd(ctx, profile.Region)
		if profile.Mode == config.ModeHistorical {
			lastRunEnd = time.Unix(0, 0)
		}

		state, err = NewRunState(ctx, r.store, profileName, profile, lastRunEnd)
		if err != nil {
			return err
		}
	}

	// 4. Execute phases; save checkpoint and mark status on exit.
	runErr := r.strategy.Execute(ctx, r.phases, state)

	if runErr == nil {
		_ = state.Complete(context.Background())
	} else if errors.Is(runErr, context.Canceled) {
		slog.Info("run interrupted, checkpoint saved – use --resume to continue", "run_id", state.ID)
	} else {
		_ = state.Fail(context.Background())
	}

	return runErr
}

// acquireAdvisoryLock tries to get a per-region PostgreSQL advisory lock (non-blocking).
func acquireAdvisoryLock(ctx context.Context, pool *pgxpool.Pool, region string) (*pgxpool.Conn, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire db conn: %w", err)
	}
	var acquired bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", regionLockKey(region)).Scan(&acquired); err != nil {
		conn.Release()
		return nil, fmt.Errorf("advisory lock query: %w", err)
	}
	if !acquired {
		conn.Release()
		return nil, fmt.Errorf("another crawler run is already executing for region %s; use --resume or wait", region)
	}
	return conn, nil
}

// phaseIDs returns the phase IDs in execution order from the phases slice.
func phaseIDs(phases []Phase) []int {
	ids := make([]int, len(phases))
	for i, p := range phases {
		ids[i] = p.ID()
	}
	return ids
}

// regionLockKey derives a stable int64 advisory lock key from a region name.
func regionLockKey(region string) int64 {
	h := fnv.New32a()
	h.Write([]byte("gogg:" + region))
	return int64(h.Sum32())
}

// SignalContext returns a context that is cancelled when SIGINT or SIGTERM is received.
func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	return ctx, stop
}