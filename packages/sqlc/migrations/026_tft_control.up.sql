CREATE TABLE tft_crawl_runs (
    id                    bigserial PRIMARY KEY,
    workflow_id           text NOT NULL,
    workflow_run_id       text NOT NULL UNIQUE,
    schedule_id           text NOT NULL,
    profile_name          text NOT NULL,
    platform              text NOT NULL,
    routing_region        text NOT NULL,
    queue_type            text NOT NULL DEFAULT 'RANKED_TFT',
    queue_id              int NOT NULL DEFAULT 1100,
    status                text NOT NULL CHECK (status IN ('queued','running','pausing','paused','completed','completed_with_errors','cancelled','failed','paused_needs_auth')),
    desired_state         text NOT NULL DEFAULT 'running' CHECK (desired_state IN ('running','paused','cancelled')),
    stage                 text NOT NULL DEFAULT 'seed',
    target_patch          text,
    target_set            text,
    window_start          timestamptz NOT NULL,
    window_end            timestamptz NOT NULL,
    config                jsonb NOT NULL DEFAULT '{}'::jsonb,
    discovered_seeds      bigint NOT NULL DEFAULT 0,
    discovered_matches    bigint NOT NULL DEFAULT 0,
    completed_matches     bigint NOT NULL DEFAULT 0,
    terminal_matches      bigint NOT NULL DEFAULT 0,
    last_error            text,
    started_at            timestamptz,
    ended_at              timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CHECK (window_end > window_start)
);

CREATE UNIQUE INDEX tft_crawl_runs_one_active_scope
    ON tft_crawl_runs (platform, profile_name)
    WHERE status IN ('queued','running','pausing','paused','paused_needs_auth');

CREATE INDEX tft_crawl_runs_status_updated
    ON tft_crawl_runs (status, updated_at DESC);

CREATE TABLE tft_crawl_checkpoints (
    run_id             bigint NOT NULL REFERENCES tft_crawl_runs(id) ON DELETE CASCADE,
    stage              text NOT NULL,
    scope_key          text NOT NULL,
    cursor             jsonb NOT NULL DEFAULT '{}'::jsonb,
    processed          bigint NOT NULL DEFAULT 0,
    next_eligible_at   timestamptz NOT NULL DEFAULT now(),
    lease_owner        text,
    lease_expires_at   timestamptz,
    attempt            int NOT NULL DEFAULT 0,
    updated_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, stage, scope_key)
);

CREATE INDEX tft_crawl_checkpoints_ready
    ON tft_crawl_checkpoints (stage, next_eligible_at)
    WHERE lease_owner IS NULL;

CREATE TABLE tft_seed_snapshots (
    id             bigserial PRIMARY KEY,
    run_id         bigint NOT NULL REFERENCES tft_crawl_runs(id) ON DELETE CASCADE,
    platform       text NOT NULL,
    queue_type     text NOT NULL,
    cohort         text NOT NULL CHECK (cohort IN ('MASTER_PLUS','DIAMOND')),
    tier           text NOT NULL,
    division       text,
    puuid          text NOT NULL,
    league_id      text,
    league_points  int,
    wins           int,
    losses         int,
    sample_bucket  int,
    selected       boolean NOT NULL DEFAULT false,
    captured_at    timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX tft_seed_snapshots_identity
    ON tft_seed_snapshots (run_id, platform, queue_type, puuid, tier, COALESCE(division, ''));

CREATE INDEX tft_seed_snapshots_selected
    ON tft_seed_snapshots (run_id, cohort, selected, puuid);

CREATE TABLE tft_player_match_sync (
    platform          text NOT NULL,
    puuid             text NOT NULL,
    queue_type        text NOT NULL,
    window_start      timestamptz NOT NULL,
    window_end        timestamptz NOT NULL,
    last_synced_at    timestamptz NOT NULL,
    last_match_id     text,
    updated_at        timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (platform, puuid, queue_type)
);

CREATE TABLE tft_match_discoveries (
    run_id          bigint NOT NULL REFERENCES tft_crawl_runs(id) ON DELETE CASCADE,
    platform        text NOT NULL,
    routing_region  text NOT NULL,
    seed_puuid      text NOT NULL,
    match_id        text NOT NULL,
    cohort          text NOT NULL CHECK (cohort IN ('MASTER_PLUS','DIAMOND')),
    discovered_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, seed_puuid, match_id)
);

CREATE INDEX tft_match_discoveries_match
    ON tft_match_discoveries (routing_region, match_id);

CREATE TABLE tft_match_jobs (
    routing_region   text NOT NULL,
    match_id         text NOT NULL,
    platform         text NOT NULL,
    status           text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','leased','completed','terminal','retry')),
    attempt          int NOT NULL DEFAULT 0,
    next_eligible_at timestamptz NOT NULL DEFAULT now(),
    lease_owner      text,
    lease_expires_at timestamptz,
    last_status_code int,
    last_error       text,
    discovered_at   timestamptz NOT NULL DEFAULT now(),
    completed_at     timestamptz,
    updated_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (routing_region, match_id)
);

CREATE INDEX tft_match_jobs_ready
    ON tft_match_jobs (routing_region, next_eligible_at, discovered_at)
    WHERE status IN ('pending','retry');
