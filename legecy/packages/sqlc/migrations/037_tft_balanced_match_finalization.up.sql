CREATE TABLE tft_run_match_sampling (
    run_id               bigint PRIMARY KEY REFERENCES tft_crawl_runs(id) ON DELETE CASCADE,
    phase                text NOT NULL DEFAULT 'open' CHECK (phase IN ('open','finalized')),
    target_per_region    int NOT NULL CHECK (target_per_region > 0),
    selection_revision   text NOT NULL,
    finalized_at         timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    CHECK ((phase = 'open' AND finalized_at IS NULL) OR
           (phase = 'finalized' AND finalized_at IS NOT NULL))
);

CREATE TABLE tft_run_platform_sampling (
    run_id                    bigint NOT NULL REFERENCES tft_crawl_runs(id) ON DELETE CASCADE,
    platform                  text NOT NULL,
    master_limit              int NOT NULL CHECK (master_limit >= 0),
    diamond_per_division      int NOT NULL CHECK (diamond_per_division >= 0),
    salt                      text NOT NULL,
    resolved_at               timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, platform)
);

CREATE TABLE tft_run_player_match_sync (
    run_id          bigint NOT NULL REFERENCES tft_crawl_runs(id) ON DELETE CASCADE,
    platform        text NOT NULL,
    puuid           text NOT NULL,
    queue_type      text NOT NULL,
    window_start    timestamptz NOT NULL,
    window_end      timestamptz NOT NULL,
    last_synced_at  timestamptz NOT NULL,
    last_match_id   text,
    updated_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, platform, puuid, queue_type)
);

ALTER TABLE tft_match_discoveries
    DROP CONSTRAINT tft_match_discoveries_pkey;

ALTER TABLE tft_match_discoveries
    ADD PRIMARY KEY (run_id, seed_puuid, match_id, cohort);
