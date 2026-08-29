CREATE TABLE tft_static_snapshots (
    id              bigserial PRIMARY KEY,
    source          text NOT NULL CHECK (source IN ('ddragon','cdragon')),
    patch           text NOT NULL,
    build           text NOT NULL,
    revision        text NOT NULL,
    locale          text NOT NULL,
    status          text NOT NULL DEFAULT 'building' CHECK (status IN ('building','published','failed')),
    etag            text,
    last_modified   text,
    source_url      text NOT NULL,
    parser_version  text NOT NULL,
    fetched_at      timestamptz NOT NULL DEFAULT now(),
    published_at    timestamptz,
    UNIQUE (source, patch, build, revision, locale)
);

CREATE TABLE tft_static_objects (
    snapshot_id    bigint NOT NULL REFERENCES tft_static_snapshots(id) ON DELETE CASCADE,
    object_kind    text NOT NULL,
    object_id      text NOT NULL,
    name           text,
    purchasable    boolean,
    cost           int,
    payload         jsonb NOT NULL,
    PRIMARY KEY (snapshot_id, object_kind, object_id)
);

CREATE TABLE tft_static_assets (
    snapshot_id    bigint NOT NULL REFERENCES tft_static_snapshots(id) ON DELETE CASCADE,
    asset_key      text NOT NULL,
    relative_path  text NOT NULL,
    sha256         text NOT NULL CHECK (length(sha256) = 64),
    PRIMARY KEY (snapshot_id, asset_key)
);

CREATE TABLE tft_static_asset_jobs (
    snapshot_id      bigint NOT NULL REFERENCES tft_static_snapshots(id) ON DELETE CASCADE,
    asset_key        text NOT NULL,
    source_url       text NOT NULL,
    relative_path    text NOT NULL,
    status           text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','leased','completed','failed')),
    attempt          int NOT NULL DEFAULT 0,
    lease_owner      text,
    lease_expires_at timestamptz,
    last_error       text,
    updated_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (snapshot_id, asset_key)
);

CREATE INDEX tft_static_asset_jobs_ready
    ON tft_static_asset_jobs (status, updated_at)
    WHERE status IN ('pending','failed');

ALTER TABLE tft_matches
    ADD CONSTRAINT tft_matches_static_revision_fk
    FOREIGN KEY (static_revision_id) REFERENCES tft_static_snapshots(id);

CREATE TABLE tft_lineup_exact_rollups (
    platform          text NOT NULL,
    patch             text NOT NULL,
    set_number        int NOT NULL,
    queue_id          int NOT NULL,
    cohort            text NOT NULL CHECK (cohort IN ('MASTER_PLUS','DIAMOND')),
    window_kind       text NOT NULL CHECK (window_kind IN ('THREE_DAYS','PATCH')),
    signature         text NOT NULL,
    sample_size       bigint NOT NULL,
    lobby_count       bigint NOT NULL,
    placement_sum     bigint NOT NULL,
    first_count       bigint NOT NULL,
    top4_count        bigint NOT NULL,
    contested_count   bigint NOT NULL,
    unit_ids          text[] NOT NULL,
    item_counts       jsonb NOT NULL DEFAULT '{}'::jsonb,
    augment_counts    jsonb NOT NULL DEFAULT '{}'::jsonb,
    trait_counts      jsonb NOT NULL DEFAULT '{}'::jsonb,
    refreshed_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (platform, patch, set_number, queue_id, cohort, window_kind, signature)
);

CREATE TABLE tft_lineup_publications (
    id                bigserial PRIMARY KEY,
    platform          text NOT NULL,
    patch             text NOT NULL,
    set_number        int NOT NULL,
    queue_id          int NOT NULL,
    cohort            text NOT NULL CHECK (cohort IN ('MASTER_PLUS','DIAMOND')),
    window_kind       text NOT NULL CHECK (window_kind IN ('THREE_DAYS','PATCH')),
    algorithm_version text NOT NULL,
    status            text NOT NULL CHECK (status IN ('building','published','superseded','failed')),
    source_matches    bigint NOT NULL,
    source_participants bigint NOT NULL,
    coverage          jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at        timestamptz NOT NULL DEFAULT now(),
    published_at      timestamptz
);

CREATE INDEX tft_lineup_publications_history
    ON tft_lineup_publications (platform, patch, set_number, queue_id, cohort, window_kind, created_at DESC);

CREATE UNIQUE INDEX tft_lineup_publications_one_published
    ON tft_lineup_publications (platform, patch, set_number, queue_id, cohort, window_kind)
    WHERE status = 'published';

CREATE TABLE tft_lineup_families (
    publication_id      bigint NOT NULL REFERENCES tft_lineup_publications(id) ON DELETE CASCADE,
    family_id           text NOT NULL,
    display_name        text,
    sample_size         bigint NOT NULL,
    lobby_count         bigint NOT NULL,
    pick_rate           double precision NOT NULL,
    avg_placement       double precision NOT NULL,
    first_rate          double precision NOT NULL,
    top4_rate           double precision NOT NULL,
    contested_rate      double precision NOT NULL,
    core_units          text[] NOT NULL,
    common_items        jsonb NOT NULL DEFAULT '[]'::jsonb,
    common_augments     jsonb NOT NULL DEFAULT '[]'::jsonb,
    common_traits       jsonb NOT NULL DEFAULT '[]'::jsonb,
    PRIMARY KEY (publication_id, family_id)
);

CREATE TABLE tft_lineup_family_members (
    publication_id bigint NOT NULL,
    family_id      text NOT NULL,
    signature      text NOT NULL,
    sample_size    bigint NOT NULL,
    jaccard        double precision NOT NULL,
    PRIMARY KEY (publication_id, family_id, signature),
    FOREIGN KEY (publication_id, family_id) REFERENCES tft_lineup_families(publication_id, family_id) ON DELETE CASCADE
);
