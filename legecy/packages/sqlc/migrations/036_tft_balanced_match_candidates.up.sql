CREATE TABLE tft_run_match_candidates (
    run_id           bigint NOT NULL REFERENCES tft_crawl_runs(id) ON DELETE CASCADE,
    routing_region   text NOT NULL CHECK (routing_region IN ('AMERICAS','ASIA','EUROPE','SEA')),
    match_id         text NOT NULL,
    platform         text NOT NULL,
    selection_key    text NOT NULL,
    selected         boolean NOT NULL DEFAULT false,
    discovered_at    timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, routing_region, match_id)
);

CREATE INDEX tft_run_match_candidates_selection
    ON tft_run_match_candidates (run_id, routing_region, selection_key, match_id);

CREATE TABLE tft_run_match_candidate_sources (
    run_id           bigint NOT NULL,
    routing_region   text NOT NULL,
    match_id         text NOT NULL,
    platform         text NOT NULL,
    seed_puuid       text NOT NULL,
    cohort           text NOT NULL CHECK (cohort IN ('MASTER_PLUS','DIAMOND')),
    discovered_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, routing_region, match_id, seed_puuid, cohort),
    FOREIGN KEY (run_id, routing_region, match_id)
        REFERENCES tft_run_match_candidates(run_id, routing_region, match_id)
        ON DELETE CASCADE
);

CREATE INDEX tft_run_match_candidate_sources_projection
    ON tft_run_match_candidate_sources (run_id, routing_region, match_id);
