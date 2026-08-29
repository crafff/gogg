-- name: CreateTFTStaticSnapshot :one
INSERT INTO tft_static_snapshots (
    source, patch, build, revision, locale, status, etag, last_modified, source_url, parser_version
) VALUES (
    @source, @patch, @build, @revision, @locale, 'building', sqlc.narg(etag),
    sqlc.narg(last_modified), @source_url, @parser_version
)
ON CONFLICT (source, patch, build, revision, locale) DO UPDATE
SET etag = EXCLUDED.etag, last_modified = EXCLUDED.last_modified,
    parser_version = EXCLUDED.parser_version, fetched_at = now()
RETURNING *;

-- name: UpsertTFTStaticObject :exec
INSERT INTO tft_static_objects (
    snapshot_id, object_kind, object_id, name, purchasable, cost, payload
) VALUES (
    @snapshot_id, @object_kind, @object_id, sqlc.narg(name), sqlc.narg(purchasable), sqlc.narg(cost), @payload
)
ON CONFLICT (snapshot_id, object_kind, object_id) DO UPDATE
SET name = COALESCE(EXCLUDED.name, tft_static_objects.name),
    purchasable = COALESCE(EXCLUDED.purchasable, tft_static_objects.purchasable),
    cost = COALESCE(EXCLUDED.cost, tft_static_objects.cost),
    payload = EXCLUDED.payload;

-- name: PublishTFTStaticSnapshot :execrows
UPDATE tft_static_snapshots
SET status = 'published', published_at = now()
WHERE id = $1
  AND NOT EXISTS (
      SELECT 1 FROM tft_static_asset_jobs
      WHERE snapshot_id = tft_static_snapshots.id AND status NOT IN ('completed','skipped')
  );

-- name: EnqueueTFTStaticAsset :execrows
INSERT INTO tft_static_asset_jobs (snapshot_id, asset_key, source_url, relative_path)
VALUES (@snapshot_id, @asset_key, @source_url, @relative_path)
ON CONFLICT (snapshot_id, asset_key) DO UPDATE
SET source_url = EXCLUDED.source_url,
    relative_path = EXCLUDED.relative_path,
    status = CASE
      WHEN tft_static_asset_jobs.status IN ('pending','failed') AND tft_static_asset_jobs.attempt >= 5 THEN 'pending'
      ELSE tft_static_asset_jobs.status
    END,
    attempt = CASE
      WHEN tft_static_asset_jobs.status IN ('pending','failed') AND tft_static_asset_jobs.attempt >= 5 THEN 0
      ELSE tft_static_asset_jobs.attempt
    END,
    last_error = CASE
      WHEN tft_static_asset_jobs.status IN ('pending','failed') AND tft_static_asset_jobs.attempt >= 5 THEN NULL
      ELSE tft_static_asset_jobs.last_error
    END,
    lease_owner = CASE
      WHEN tft_static_asset_jobs.status IN ('pending','failed') AND tft_static_asset_jobs.attempt >= 5 THEN NULL
      ELSE tft_static_asset_jobs.lease_owner
    END,
    lease_expires_at = CASE
      WHEN tft_static_asset_jobs.status IN ('pending','failed') AND tft_static_asset_jobs.attempt >= 5 THEN NULL
      ELSE tft_static_asset_jobs.lease_expires_at
    END,
    updated_at = now();

-- name: ClaimTFTStaticAssetJobs :many
WITH candidates AS (
    SELECT snapshot_id, asset_key
    FROM tft_static_asset_jobs
    WHERE (((status = 'pending' OR (status = 'failed' AND attempt < 5))
              AND (lease_expires_at IS NULL OR lease_expires_at <= now()))
           OR (status = 'leased' AND lease_expires_at <= now()))
    ORDER BY updated_at, snapshot_id, asset_key
    LIMIT @row_limit
    FOR UPDATE SKIP LOCKED
)
UPDATE tft_static_asset_jobs jobs
SET status = 'leased', lease_owner = @lease_owner,
    lease_expires_at = now() + make_interval(secs => @lease_seconds::int),
    attempt = jobs.attempt + 1, updated_at = now()
FROM candidates
WHERE jobs.snapshot_id = candidates.snapshot_id AND jobs.asset_key = candidates.asset_key
RETURNING jobs.*;

-- name: ClaimTFTStaticAssetGroups :many
WITH candidate_keys AS (
    SELECT jobs.asset_key, MIN(jobs.updated_at) AS oldest
    FROM tft_static_asset_jobs jobs
    WHERE jobs.snapshot_id = ANY(@snapshot_ids::bigint[])
      AND (((jobs.status = 'pending' OR (jobs.status = 'failed' AND jobs.attempt < 5))
              AND (jobs.lease_expires_at IS NULL OR jobs.lease_expires_at <= now()))
           OR (jobs.status = 'leased' AND jobs.lease_expires_at <= now()))
    GROUP BY jobs.asset_key
    ORDER BY oldest, jobs.asset_key
    LIMIT @row_limit
), claimed AS (
    UPDATE tft_static_asset_jobs jobs
    SET status = 'leased', lease_owner = @lease_owner,
        lease_expires_at = now() + make_interval(secs => @lease_seconds::int),
        attempt = jobs.attempt + 1, updated_at = now()
    FROM candidate_keys candidates
    WHERE jobs.snapshot_id = ANY(@snapshot_ids::bigint[])
      AND jobs.asset_key = candidates.asset_key
      AND (((jobs.status = 'pending' OR (jobs.status = 'failed' AND jobs.attempt < 5))
              AND (jobs.lease_expires_at IS NULL OR jobs.lease_expires_at <= now()))
           OR (jobs.status = 'leased' AND jobs.lease_expires_at <= now()))
    RETURNING jobs.*
)
SELECT * FROM claimed ORDER BY asset_key, snapshot_id;

-- name: CompleteTFTStaticAssetJob :execrows
WITH saved AS (
    INSERT INTO tft_static_assets (snapshot_id, asset_key, relative_path, sha256)
    SELECT jobs.snapshot_id, jobs.asset_key, jobs.relative_path, @sha256
    FROM tft_static_asset_jobs jobs
    WHERE jobs.snapshot_id = sqlc.arg(target_snapshot_id) AND jobs.asset_key = sqlc.arg(target_asset_key)
      AND jobs.lease_owner = sqlc.arg(lease_owner_filter)
      AND jobs.lease_expires_at > now()
    ON CONFLICT (snapshot_id, asset_key) DO UPDATE
    SET relative_path = EXCLUDED.relative_path, sha256 = EXCLUDED.sha256
)
UPDATE tft_static_asset_jobs jobs
SET status = 'completed', lease_owner = NULL, lease_expires_at = NULL, last_error = NULL, updated_at = now()
WHERE jobs.snapshot_id = sqlc.arg(target_snapshot_id) AND jobs.asset_key = sqlc.arg(target_asset_key)
  AND jobs.lease_owner = sqlc.arg(lease_owner_filter) AND jobs.lease_expires_at > now();

-- name: CompleteTFTStaticAssetGroup :execrows
WITH saved AS (
    INSERT INTO tft_static_assets (snapshot_id, asset_key, relative_path, sha256)
    SELECT jobs.snapshot_id, jobs.asset_key, jobs.relative_path, @sha256
    FROM tft_static_asset_jobs jobs
    WHERE jobs.snapshot_id = ANY(@snapshot_ids::bigint[])
      AND jobs.asset_key = @asset_key
      AND jobs.lease_owner = @lease_owner
      AND jobs.lease_expires_at > now()
    ON CONFLICT (snapshot_id, asset_key) DO UPDATE
    SET relative_path = EXCLUDED.relative_path, sha256 = EXCLUDED.sha256
)
UPDATE tft_static_asset_jobs jobs
SET status = 'completed', lease_owner = NULL, lease_expires_at = NULL,
    last_error = NULL, updated_at = now()
WHERE jobs.snapshot_id = ANY(@snapshot_ids::bigint[])
  AND jobs.asset_key = @asset_key
  AND jobs.lease_owner = @lease_owner
  AND jobs.lease_expires_at > now();

-- name: SkipTFTStaticAssetGroup :execrows
WITH saved AS (
    INSERT INTO tft_static_assets (snapshot_id, asset_key, relative_path, sha256)
    SELECT jobs.snapshot_id, jobs.asset_key, jobs.relative_path, @sha256
    FROM tft_static_asset_jobs jobs
    WHERE jobs.snapshot_id = ANY(@snapshot_ids::bigint[])
      AND jobs.asset_key = @asset_key
      AND jobs.lease_owner = @lease_owner
      AND jobs.lease_expires_at > now()
    ON CONFLICT (snapshot_id, asset_key) DO UPDATE
    SET relative_path = EXCLUDED.relative_path, sha256 = EXCLUDED.sha256
)
UPDATE tft_static_asset_jobs jobs
SET status = 'skipped', lease_owner = NULL, lease_expires_at = NULL,
    last_error = @last_error, updated_at = now()
WHERE jobs.snapshot_id = ANY(@snapshot_ids::bigint[])
  AND jobs.asset_key = @asset_key
  AND jobs.lease_owner = @lease_owner
  AND jobs.lease_expires_at > now();

-- name: SkipTFTStaticAssetJob :execrows
WITH saved AS (
    INSERT INTO tft_static_assets (snapshot_id, asset_key, relative_path, sha256)
    SELECT jobs.snapshot_id, jobs.asset_key, jobs.relative_path, @sha256
    FROM tft_static_asset_jobs jobs
    WHERE jobs.snapshot_id = sqlc.arg(target_snapshot_id)
      AND jobs.asset_key = sqlc.arg(target_asset_key)
      AND jobs.lease_owner = sqlc.arg(lease_owner_filter)
      AND jobs.lease_expires_at > now()
    ON CONFLICT (snapshot_id, asset_key) DO UPDATE
    SET relative_path = EXCLUDED.relative_path, sha256 = EXCLUDED.sha256
)
UPDATE tft_static_asset_jobs jobs
SET status = 'skipped', lease_owner = NULL, lease_expires_at = NULL,
    last_error = @last_error, updated_at = now()
WHERE jobs.snapshot_id = sqlc.arg(target_snapshot_id)
  AND jobs.asset_key = sqlc.arg(target_asset_key)
  AND jobs.lease_owner = sqlc.arg(lease_owner_filter)
  AND jobs.lease_expires_at > now();

-- name: FailTFTStaticAssetJob :execrows
UPDATE tft_static_asset_jobs jobs
SET status = 'failed', lease_owner = NULL,
    lease_expires_at = now() + make_interval(
        secs => LEAST(300, 5 * (1 << LEAST(GREATEST(jobs.attempt - 1, 0), 6)))
    ),
    last_error = @last_error, updated_at = now()
WHERE jobs.snapshot_id = sqlc.arg(target_snapshot_id) AND jobs.asset_key = sqlc.arg(target_asset_key)
  AND jobs.lease_owner = sqlc.arg(lease_owner_filter) AND jobs.lease_expires_at > now();

-- name: FailTFTStaticAssetGroup :execrows
UPDATE tft_static_asset_jobs jobs
SET status = 'failed', lease_owner = NULL,
    lease_expires_at = now() + make_interval(
        secs => LEAST(300, 5 * (1 << LEAST(GREATEST(jobs.attempt - 1, 0), 6)))
    ),
    last_error = @last_error, updated_at = now()
WHERE jobs.snapshot_id = ANY(@snapshot_ids::bigint[])
  AND jobs.asset_key = @asset_key
  AND jobs.lease_owner = @lease_owner
  AND jobs.lease_expires_at > now();

-- name: ReleaseTFTStaticAssetLeases :exec
UPDATE tft_static_asset_jobs
SET status = 'pending', attempt = GREATEST(attempt - 1, 0),
    lease_owner = NULL, lease_expires_at = NULL, updated_at = now()
WHERE lease_owner = @lease_owner AND status = 'leased';

-- name: GetTFTStaticAssetQueueState :one
SELECT COUNT(*)::bigint AS remaining,
       MIN(
           CASE WHEN status IN ('leased','failed') THEN COALESCE(lease_expires_at, now())
                ELSE now()
           END
       )::timestamptz AS next_eligible_at
FROM tft_static_asset_jobs
WHERE status = 'pending'
   OR (status = 'failed' AND attempt < 5)
   OR status = 'leased';

-- name: GetTFTStaticAssetQueueStateForSnapshots :one
SELECT COUNT(*)::bigint AS total,
       COUNT(*) FILTER (WHERE status = 'completed')::bigint AS completed,
       COUNT(*) FILTER (WHERE status = 'skipped')::bigint AS skipped,
       COUNT(*) FILTER (WHERE status = 'failed')::bigint AS failed,
       COUNT(*) FILTER (WHERE status = 'failed' AND attempt >= 5)::bigint AS exhausted,
       COUNT(*) FILTER (
           WHERE status = 'pending'
              OR (status = 'failed' AND attempt < 5)
              OR status = 'leased'
       )::bigint AS remaining,
       MIN(
           CASE WHEN status IN ('leased','failed') THEN COALESCE(lease_expires_at, now())
                ELSE now()
           END
       )
           FILTER (
               WHERE status = 'pending'
                  OR (status = 'failed' AND attempt < 5)
                  OR status = 'leased'
           )::timestamptz AS next_eligible_at
FROM tft_static_asset_jobs
WHERE snapshot_id = ANY(@snapshot_ids::bigint[]);

-- name: ListTFTPurchasableUnits :many
SELECT object_id, name, cost
FROM tft_static_objects
WHERE snapshot_id = @snapshot_id
  AND object_kind = 'unit'
  AND purchasable IS TRUE
  AND cost > 0
ORDER BY object_id;

-- name: GetLatestPublishedTFTStaticSnapshot :one
SELECT * FROM tft_static_snapshots
WHERE source = @source AND patch = @patch AND locale = @locale AND status = 'published'
ORDER BY published_at DESC, id DESC
LIMIT 1;

-- name: GetLatestPublishedTFTStaticSnapshotAnyPatch :one
SELECT * FROM tft_static_snapshots
WHERE source = @source AND locale = @locale AND status = 'published'
ORDER BY published_at DESC, id DESC
LIMIT 1;

-- name: GetLatestTFTStaticSnapshotAnyStatus :one
SELECT * FROM tft_static_snapshots
WHERE source = @source AND locale = @locale
ORDER BY fetched_at DESC, id DESC
LIMIT 1;
