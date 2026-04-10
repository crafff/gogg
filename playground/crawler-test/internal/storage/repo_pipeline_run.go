package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const pipelineLockKey int64 = 782431987

type PipelineRunSeed struct {
	PipelineType   string
	StartStep      int
	CurrentStep    int
	Status         string
	ConfigSnapshot []byte
}

func (s *Store) AcquirePipelineLock(ctx context.Context) (*pgxpool.Conn, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("store is not initialized")
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire pipeline lock connection: %w", err)
	}

	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1);`, pipelineLockKey).Scan(&locked); err != nil {
		conn.Release()
		return nil, fmt.Errorf("try acquire pipeline lock: %w", err)
	}
	if !locked {
		conn.Release()
		return nil, fmt.Errorf("pipeline is already running")
	}

	return conn, nil
}

func (s *Store) ReleasePipelineLock(ctx context.Context, conn *pgxpool.Conn) error {
	if conn == nil {
		return nil
	}

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_unlock($1);`, pipelineLockKey); err != nil {
		conn.Release()
		return fmt.Errorf("release pipeline lock: %w", err)
	}

	conn.Release()
	return nil
}

func (s *Store) CreatePipelineRun(ctx context.Context, seed PipelineRunSeed) (int64, error) {
	if s == nil || s.pool == nil {
		return 0, fmt.Errorf("store is not initialized")
	}
	if seed.PipelineType == "" {
		return 0, fmt.Errorf("pipeline_type is required")
	}

	var runID int64
	if err := s.pool.QueryRow(ctx, `
INSERT INTO pipeline_runs (
	pipeline_type,
	start_step,
	current_step,
	status,
	heartbeat_at,
	config_snapshot,
	created_at,
	updated_at
)
VALUES ($1, $2, $3, $4, NOW(), COALESCE($5, '{}'::jsonb), NOW(), NOW())
RETURNING id;
`, seed.PipelineType, seed.StartStep, seed.CurrentStep, seed.Status, seed.ConfigSnapshot).Scan(&runID); err != nil {
		return 0, fmt.Errorf("create pipeline run: %w", err)
	}

	return runID, nil
}

func (s *Store) UpdatePipelineRunStep(ctx context.Context, runID int64, currentStep int) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}
	if runID == 0 {
		return fmt.Errorf("run_id is required")
	}

	_, err := s.pool.Exec(ctx, `
UPDATE pipeline_runs
SET current_step = $2,
	updated_at = NOW(),
	heartbeat_at = NOW()
WHERE id = $1;
`, runID, currentStep)
	if err != nil {
		return fmt.Errorf("update pipeline run step: %w", err)
	}

	return nil
}

func (s *Store) HeartbeatPipelineRun(ctx context.Context, runID int64) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}
	if runID == 0 {
		return fmt.Errorf("run_id is required")
	}

	_, err := s.pool.Exec(ctx, `
UPDATE pipeline_runs
SET heartbeat_at = NOW(),
	updated_at = NOW()
WHERE id = $1;
`, runID)
	if err != nil {
		return fmt.Errorf("heartbeat pipeline run: %w", err)
	}

	return nil
}

func (s *Store) FinishPipelineRun(ctx context.Context, runID int64, status, errorSummary string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}
	if runID == 0 {
		return fmt.Errorf("run_id is required")
	}
	if status == "" {
		return fmt.Errorf("status is required")
	}

	var finishedAt *time.Time
	if status == "success" || status == "failed" || status == "cancelled" {
		now := time.Now().UTC()
		finishedAt = &now
	}

	_, err := s.pool.Exec(ctx, `
UPDATE pipeline_runs
SET status = $2,
	finished_at = COALESCE($3, finished_at),
	error_summary = $4,
	updated_at = NOW()
WHERE id = $1;
`, runID, status, finishedAt, errorSummary)
	if err != nil {
		return fmt.Errorf("finish pipeline run: %w", err)
	}

	return nil
}

func (s *Store) SnapshotPipelineConfig(cfg interface{}) ([]byte, error) {
	if cfg == nil {
		return json.Marshal(map[string]any{})
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal pipeline config: %w", err)
	}
	return data, nil
}
