package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type Run struct {
	ID                int
	Status            string
	Profile           *string
	Mode              string
	TargetTiers       []string
	RankPrefetchTiers []string
	Queue             string
	Execution         string
	Version           *string
	Region            string
	CurrentPhase      int
	CurrentTier       *string
	StartedAt         time.Time
	EndedAt           *time.Time
	LastRunEnd        time.Time
}

// RunProfile carries the config fields stored alongside a run.
type RunProfile struct {
	Mode              string
	TargetTiers       []string
	RankPrefetchTiers []string
	Queue             string
	Execution         string
	Version           *string
	Region            string
}

// CreateRun inserts a new run and returns its ID and started_at.
func (s *Store) CreateRun(ctx context.Context, profile *string, p RunProfile, lastRunEnd time.Time) (int, time.Time, error) {
	var id int
	var startedAt time.Time
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO runs (status, profile, mode, target_tiers, rank_prefetch_tiers, queue, execution, version, region, last_run_end)
		VALUES ('running', $1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, started_at`,
		profile, p.Mode, p.TargetTiers, p.RankPrefetchTiers, p.Queue, p.Execution, p.Version, p.Region, lastRunEnd,
	).Scan(&id, &startedAt)
	return id, startedAt, err
}

// GetActiveRun returns the most recent running run for the given region, if any.
func (s *Store) GetActiveRun(ctx context.Context, region string) (*Run, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, status, profile, mode, target_tiers, rank_prefetch_tiers, queue, execution, version, region,
		       current_phase, current_tier, started_at, ended_at, last_run_end
		FROM runs
		WHERE status = 'running' AND region = $1
		ORDER BY id DESC
		LIMIT 1`, region)
	r, err := scanRun(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return r, err
}

// GetRunByID returns a single run by ID, or nil if not found.
func (s *Store) GetRunByID(ctx context.Context, id int) (*Run, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, status, profile, mode, target_tiers, rank_prefetch_tiers, queue, execution, version, region,
		       current_phase, current_tier, started_at, ended_at, last_run_end
		FROM runs WHERE id = $1`, id)
	r, err := scanRun(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return r, err
}

// ReactivateRun resets a failed/interrupted run back to 'running' for resume.
func (s *Store) ReactivateRun(ctx context.Context, runID int) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE runs SET status = 'running', ended_at = NULL WHERE id = $1`, runID)
	return err
}

// UpdateRunVersion stores the resolved game version on a run.
func (s *Store) UpdateRunVersion(ctx context.Context, runID int, version string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE runs SET version = $1 WHERE id = $2`, version, runID)
	return err
}

// ResetRunToPhase0 resets a run's checkpoint to phase 0 and deletes phase1 snapshots.
func (s *Store) ResetRunToPhase0(ctx context.Context, runID int) error {
	if _, err := s.Pool.Exec(ctx,
		`UPDATE runs SET current_phase = 0, current_tier = NULL WHERE id = $1`, runID); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM player_rank_snapshots WHERE run_id = $1`, runID)
	return err
}

// UpdateCheckpoint saves the current phase/tier progress for a run.
func (s *Store) UpdateCheckpoint(ctx context.Context, runID, phase int, tier *string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE runs SET current_phase = $1, current_tier = $2 WHERE id = $3`,
		phase, tier, runID)
	return err
}


// CompleteRun marks a run as completed.
func (s *Store) CompleteRun(ctx context.Context, runID int) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE runs SET status = 'completed', ended_at = now() WHERE id = $1`, runID)
	return err
}

// FailRun marks a run as failed.
func (s *Store) FailRun(ctx context.Context, runID int) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE runs SET status = 'failed', ended_at = now() WHERE id = $1`, runID)
	return err
}

// CancelRun marks a run as failed (used by the cancel CLI command).
func (s *Store) CancelRun(ctx context.Context, runID int) error {
	return s.FailRun(ctx, runID)
}

// GetLastCompletedRunEnd returns the ended_at of the most recent completed run
// for the given region, or the Unix epoch if none exists.
func (s *Store) GetLastCompletedRunEnd(ctx context.Context, region string) time.Time {
	var t time.Time
	s.Pool.QueryRow(ctx, `
		SELECT ended_at FROM runs
		WHERE status = 'completed' AND region = $1
		ORDER BY id DESC
		LIMIT 1`, region).Scan(&t)
	if t.IsZero() {
		return time.Unix(0, 0)
	}
	return t
}

// ListRuns returns the N most recent runs across all regions.
func (s *Store) ListRuns(ctx context.Context, limit int) ([]Run, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, status, profile, mode, target_tiers, rank_prefetch_tiers, queue, execution, version, region,
		       current_phase, current_tier, started_at, ended_at, last_run_end
		FROM runs
		ORDER BY id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *r)
	}
	return runs, rows.Err()
}

func scanRun(row pgx.Row) (*Run, error) {
	var r Run
	err := row.Scan(
		&r.ID, &r.Status, &r.Profile, &r.Mode, &r.TargetTiers, &r.RankPrefetchTiers,
		&r.Queue, &r.Execution, &r.Version, &r.Region,
		&r.CurrentPhase, &r.CurrentTier, &r.StartedAt, &r.EndedAt, &r.LastRunEnd,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}