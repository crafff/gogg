DROP INDEX IF EXISTS idx_matches_timeline_retry_ready;
DROP INDEX IF EXISTS idx_matches_fetch_retry_ready;

ALTER TABLE matches DROP COLUMN IF EXISTS timeline_last_error_message;
ALTER TABLE matches DROP COLUMN IF EXISTS timeline_last_error_code;
ALTER TABLE matches DROP COLUMN IF EXISTS timeline_last_error_kind;
ALTER TABLE matches DROP COLUMN IF EXISTS timeline_next_retry_at;

ALTER TABLE matches DROP COLUMN IF EXISTS fetch_last_error_message;
ALTER TABLE matches DROP COLUMN IF EXISTS fetch_last_error_code;
ALTER TABLE matches DROP COLUMN IF EXISTS fetch_last_error_kind;
ALTER TABLE matches DROP COLUMN IF EXISTS fetch_next_retry_at;
