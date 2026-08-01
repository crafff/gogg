CREATE TABLE raw_api_responses (
    match_id       text        NOT NULL,
    region         text        NOT NULL,
    response_kind  text        NOT NULL CHECK (response_kind IN ('match-detail', 'timeline')),
    relative_path  text        NOT NULL,
    sha256         text        NOT NULL CHECK (length(sha256) = 64),
    raw_size       bigint      NOT NULL CHECK (raw_size >= 0),
    compressed_size bigint     NOT NULL CHECK (compressed_size >= 0),
    captured_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (match_id, response_kind)
);

CREATE INDEX raw_api_responses_export_order
    ON raw_api_responses (captured_at, match_id, response_kind);

CREATE TABLE raw_export_batches (
    id             bigserial PRIMARY KEY,
    bundle_id      text        NOT NULL UNIQUE,
    output_name    text        NOT NULL,
    status         text        NOT NULL CHECK (status IN ('building', 'completed', 'failed')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    completed_at   timestamptz
);

CREATE TABLE raw_export_batch_items (
    batch_id       bigint NOT NULL REFERENCES raw_export_batches(id) ON DELETE CASCADE,
    match_id       text   NOT NULL,
    response_kind  text   NOT NULL,
    PRIMARY KEY (batch_id, match_id, response_kind),
    FOREIGN KEY (match_id, response_kind)
        REFERENCES raw_api_responses(match_id, response_kind)
);

CREATE TABLE raw_import_batches (
    bundle_id      text PRIMARY KEY,
    source_created_at timestamptz NOT NULL,
    imported_at    timestamptz NOT NULL DEFAULT now(),
    object_count   bigint NOT NULL CHECK (object_count >= 0)
);
