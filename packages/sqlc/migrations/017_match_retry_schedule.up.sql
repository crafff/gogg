ALTER TABLE matches ADD COLUMN IF NOT EXISTS fetch_next_retry_at timestamptz;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS fetch_last_error_kind text;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS fetch_last_error_code int;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS fetch_last_error_message text;

ALTER TABLE matches ADD COLUMN IF NOT EXISTS timeline_next_retry_at timestamptz;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS timeline_last_error_kind text;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS timeline_last_error_code int;
ALTER TABLE matches ADD COLUMN IF NOT EXISTS timeline_last_error_message text;

CREATE INDEX IF NOT EXISTS idx_matches_fetch_retry_ready
    ON matches (region, version, fetch_next_retry_at)
    WHERE fetch_status = 'pending';

CREATE INDEX IF NOT EXISTS idx_matches_timeline_retry_ready
    ON matches (region, version, timeline_next_retry_at)
    WHERE fetch_status = 'done' AND timeline_status = 'pending';
