ALTER TABLE tft_static_asset_jobs
    DROP CONSTRAINT tft_static_asset_jobs_status_check;

ALTER TABLE tft_static_asset_jobs
    ADD CONSTRAINT tft_static_asset_jobs_status_check
    CHECK (status IN ('pending','leased','completed','failed','skipped'));

-- Older workers could release an unfinished lease as pending without undoing
-- the claim attempt. Repair those poison rows so they cannot hot-loop forever.
UPDATE tft_static_asset_jobs
SET attempt = 0, lease_owner = NULL, lease_expires_at = NULL, updated_at = now()
WHERE status = 'pending' AND attempt >= 5;

CREATE INDEX tft_static_asset_jobs_snapshot_ready
    ON tft_static_asset_jobs (snapshot_id, status, lease_expires_at, updated_at)
    WHERE status IN ('pending','failed','leased');
