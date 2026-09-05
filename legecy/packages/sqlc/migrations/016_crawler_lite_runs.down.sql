DROP INDEX IF EXISTS idx_runs_runner_status_updated;

ALTER TABLE runs DROP COLUMN IF EXISTS updated_at;
ALTER TABLE runs DROP COLUMN IF EXISTS last_error;
ALTER TABLE runs DROP COLUMN IF EXISTS pause_requested;
ALTER TABLE runs DROP COLUMN IF EXISTS current_division;
ALTER TABLE runs DROP COLUMN IF EXISTS runner_type;
