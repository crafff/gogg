-- name: UpsertTFTRawObject :exec
INSERT INTO tft_raw_objects (
    sha256, relative_path, content_type, compression, raw_size, compressed_size
) VALUES (
    @sha256, @relative_path, @content_type, 'gzip', @raw_size, @compressed_size
)
ON CONFLICT (sha256) DO NOTHING;

-- name: InsertTFTRawCapture :one
INSERT INTO tft_raw_captures (
    run_id, platform, routing_region, endpoint_kind, resource_key,
    request_fingerprint, request_url, object_sha256
) VALUES (
    sqlc.narg(run_id), @platform, @routing_region, @endpoint_kind, @resource_key,
    @request_fingerprint, @request_url, @object_sha256
)
ON CONFLICT (COALESCE(run_id, 0), request_fingerprint, object_sha256) DO UPDATE
SET captured_at = tft_raw_captures.captured_at
RETURNING *;

-- name: MarkTFTRawCaptureParsed :exec
UPDATE tft_raw_captures target
SET parse_status = @parse_status,
    parser_version = @parser_version,
    parse_error = sqlc.narg(parse_error)
WHERE target.id = @id;

-- name: MarkLatestTFTRawCaptureParsed :exec
UPDATE tft_raw_captures target
SET parse_status = @parse_status,
    parser_version = @parser_version,
    parse_error = sqlc.narg(parse_error)
WHERE target.id = (
    SELECT capture.id FROM tft_raw_captures capture
    WHERE capture.platform = sqlc.arg(platform_filter)
      AND capture.endpoint_kind = sqlc.arg(kind_filter)
      AND capture.resource_key = sqlc.arg(resource_filter)
    ORDER BY capture.captured_at DESC, capture.id DESC
    LIMIT 1
);

-- name: ListPendingTFTRawCaptures :many
SELECT * FROM tft_raw_captures
WHERE parse_status IN ('pending','failed')
ORDER BY captured_at, id
LIMIT $1;
