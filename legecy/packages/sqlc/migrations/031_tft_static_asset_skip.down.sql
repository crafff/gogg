UPDATE tft_static_asset_jobs
SET status = 'completed',
    last_error = COALESCE(last_error, 'skipped static asset retained as completed during rollback'),
    updated_at = now()
WHERE status = 'skipped';

DROP INDEX IF EXISTS tft_static_asset_jobs_snapshot_ready;

ALTER TABLE tft_static_asset_jobs
    DROP CONSTRAINT tft_static_asset_jobs_status_check;

ALTER TABLE tft_static_asset_jobs
    ADD CONSTRAINT tft_static_asset_jobs_status_check
    CHECK (status IN ('pending','leased','completed','failed'));
