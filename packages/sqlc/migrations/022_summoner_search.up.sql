-- On-demand summoner search state and its read-optimised history index.
-- Canonical match facts remain in matches/match_participants; these tables
-- isolate refresh bookkeeping and statistics sample membership.

CREATE TABLE IF NOT EXISTS summoner_profiles (
    region               text        NOT NULL,
    puuid                text        NOT NULL REFERENCES players(puuid) ON DELETE CASCADE,
    profile_icon_id      int,
    summoner_level       bigint,
    identity_refreshed_at timestamptz,
    matches_refreshed_at  timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (region, puuid)
);

CREATE TABLE IF NOT EXISTS summoner_current_ranks (
    region         text        NOT NULL,
    puuid          text        NOT NULL REFERENCES players(puuid) ON DELETE CASCADE,
    queue_type     text        NOT NULL CHECK (queue_type IN ('RANKED_SOLO_5x5', 'RANKED_FLEX_SR')),
    tier           text        NOT NULL,
    division       text,
    league_points  int         NOT NULL DEFAULT 0,
    wins           int         NOT NULL DEFAULT 0,
    losses         int         NOT NULL DEFAULT 0,
    refreshed_at   timestamptz NOT NULL,
    PRIMARY KEY (region, puuid, queue_type)
);

CREATE TABLE IF NOT EXISTS summoner_lookup_jobs (
    id                       text        PRIMARY KEY,
    region                   text        NOT NULL CHECK (region IN ('KR', 'NA1')),
    requested_game_name      text        NOT NULL,
    requested_tag_line       text        NOT NULL,
    requested_game_name_norm text        NOT NULL,
    requested_tag_line_norm  text        NOT NULL,
    puuid                    text        REFERENCES players(puuid),
    status                   text        NOT NULL CHECK (status IN ('QUEUED', 'RUNNING', 'COMPLETED', 'PARTIAL', 'FAILED')),
    stage                    text        NOT NULL CHECK (stage IN ('QUEUED', 'RESOLVE_ACCOUNT', 'REFRESH_PROFILE', 'FETCH_MATCHES', 'FINALIZE')),
    scanned_count            int         NOT NULL DEFAULT 0 CHECK (scanned_count BETWEEN 0 AND 100),
    supported_count          int         NOT NULL DEFAULT 0 CHECK (supported_count BETWEEN 0 AND 100),
    fetched_count            int         NOT NULL DEFAULT 0 CHECK (fetched_count BETWEEN 0 AND 100),
    failed_count             int         NOT NULL DEFAULT 0 CHECK (failed_count BETWEEN 0 AND 100),
    error_code               text,
    error_message            text,
    created_at               timestamptz NOT NULL DEFAULT now(),
    started_at               timestamptz,
    completed_at             timestamptz,
    updated_at               timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS summoner_lookup_jobs_active_identity_idx
    ON summoner_lookup_jobs (region, requested_game_name_norm, requested_tag_line_norm)
    WHERE status IN ('QUEUED', 'RUNNING');

CREATE INDEX IF NOT EXISTS summoner_lookup_jobs_created_idx
    ON summoner_lookup_jobs (created_at DESC);

CREATE TABLE IF NOT EXISTS summoner_lookup_job_matches (
    job_id       text        NOT NULL REFERENCES summoner_lookup_jobs(id) ON DELETE CASCADE,
    match_id     text        NOT NULL REFERENCES matches(match_id) ON DELETE CASCADE,
    ordinal      int         NOT NULL CHECK (ordinal BETWEEN 0 AND 99),
    supported    boolean     NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, match_id),
    UNIQUE (job_id, ordinal)
);

CREATE TABLE IF NOT EXISTS player_match_index (
    puuid         text        NOT NULL REFERENCES players(puuid) ON DELETE CASCADE,
    region        text        NOT NULL,
    match_id      text        NOT NULL REFERENCES matches(match_id) ON DELETE CASCADE,
    queue_id      int         NOT NULL,
    game_start_ts timestamptz NOT NULL,
    PRIMARY KEY (puuid, match_id)
);

CREATE INDEX IF NOT EXISTS player_match_index_recent_idx
    ON player_match_index (puuid, region, game_start_ts DESC, match_id DESC);

CREATE INDEX IF NOT EXISTS player_match_index_queue_recent_idx
    ON player_match_index (puuid, region, queue_id, game_start_ts DESC, match_id DESC);

INSERT INTO player_match_index (puuid, region, match_id, queue_id, game_start_ts)
SELECT mp.puuid, m.region, m.match_id, m.queue_id, m.game_start_ts
FROM matches m
JOIN match_participants mp ON mp.match_id = m.match_id
WHERE m.fetch_status = 'done'
  AND mp.puuid IS NOT NULL
  AND m.queue_id IS NOT NULL
  AND m.game_start_ts IS NOT NULL
ON CONFLICT (puuid, match_id) DO UPDATE
SET region = EXCLUDED.region,
    queue_id = EXCLUDED.queue_id,
    game_start_ts = EXCLUDED.game_start_ts;

CREATE TABLE IF NOT EXISTS statistics_match_membership (
    dataset_key text        NOT NULL,
    match_id    text        NOT NULL REFERENCES matches(match_id) ON DELETE CASCADE,
    run_id      int         REFERENCES runs(id) ON DELETE SET NULL,
    admitted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (dataset_key, match_id)
);

CREATE INDEX IF NOT EXISTS statistics_match_membership_match_idx
    ON statistics_match_membership (match_id, dataset_key);

-- Preserve the exact population used by the existing rollups before this
-- migration. Future on-demand matches are excluded unless a scheduled crawl
-- explicitly admits them to the same dataset.
INSERT INTO statistics_match_membership (dataset_key, match_id)
SELECT 'ranked-solo-v1', match_id
FROM matches
WHERE fetch_status = 'done' AND queue_id = 420
ON CONFLICT DO NOTHING;
