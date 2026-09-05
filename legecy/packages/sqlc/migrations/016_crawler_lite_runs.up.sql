ALTER TABLE runs ADD COLUMN IF NOT EXISTS runner_type text NOT NULL DEFAULT 'temporal';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS current_division text;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS pause_requested boolean NOT NULL DEFAULT false;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS last_error text;
ALTER TABLE runs ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS idx_runs_runner_status_updated
    ON runs (runner_type, status, updated_at DESC);
