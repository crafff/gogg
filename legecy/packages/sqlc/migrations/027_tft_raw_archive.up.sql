CREATE TABLE tft_raw_objects (
    sha256          text PRIMARY KEY CHECK (length(sha256) = 64),
    relative_path   text NOT NULL UNIQUE,
    content_type    text NOT NULL DEFAULT 'application/json',
    compression     text NOT NULL DEFAULT 'gzip' CHECK (compression IN ('gzip')),
    raw_size        bigint NOT NULL CHECK (raw_size >= 0),
    compressed_size bigint NOT NULL CHECK (compressed_size >= 0),
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tft_raw_captures (
    id                  bigserial PRIMARY KEY,
    run_id              bigint REFERENCES tft_crawl_runs(id) ON DELETE SET NULL,
    platform            text NOT NULL,
    routing_region      text NOT NULL,
    endpoint_kind       text NOT NULL,
    resource_key        text NOT NULL,
    request_fingerprint text NOT NULL,
    request_url         text NOT NULL,
    object_sha256       text NOT NULL REFERENCES tft_raw_objects(sha256),
    parse_status        text NOT NULL DEFAULT 'pending' CHECK (parse_status IN ('pending','parsed','failed','ignored')),
    parser_version      text,
    parse_error         text,
    captured_at         timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX tft_raw_captures_run_object
    ON tft_raw_captures (COALESCE(run_id, 0), request_fingerprint, object_sha256);

CREATE INDEX tft_raw_captures_replay
    ON tft_raw_captures (parse_status, captured_at, id);

CREATE INDEX tft_raw_captures_resource
    ON tft_raw_captures (endpoint_kind, resource_key, captured_at DESC);

CREATE TABLE tft_export_batches (
    id            bigserial PRIMARY KEY,
    bundle_id     text NOT NULL UNIQUE,
    output_name   text NOT NULL,
    status        text NOT NULL CHECK (status IN ('building','completed','failed')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    completed_at  timestamptz
);

CREATE TABLE tft_export_batch_items (
    batch_id       bigint NOT NULL REFERENCES tft_export_batches(id) ON DELETE CASCADE,
    capture_id     bigint NOT NULL REFERENCES tft_raw_captures(id),
    PRIMARY KEY (batch_id, capture_id)
);

CREATE TABLE tft_import_batches (
    bundle_id          text PRIMARY KEY,
    schema_version     text NOT NULL DEFAULT 'gogg.tft.raw/v1',
    source_created_at  timestamptz NOT NULL,
    imported_at        timestamptz NOT NULL DEFAULT now(),
    object_count       bigint NOT NULL CHECK (object_count >= 0)
);
