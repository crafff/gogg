-- TFT player lookup is deliberately separate from the LoL summoner lookup
-- tables: TFT supports fifteen platforms and runs on its own Temporal worker.

CREATE TABLE tft_player_identities (
    platform              text        NOT NULL,
    puuid                 text        NOT NULL,
    game_name             text        NOT NULL,
    tag_line              text        NOT NULL,
    identity_refreshed_at timestamptz NOT NULL,
    matches_refreshed_at  timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (platform, puuid)
);

CREATE UNIQUE INDEX tft_player_identities_riot_id
    ON tft_player_identities (platform, lower(game_name), upper(tag_line));

CREATE TABLE tft_player_lookup_jobs (
    id                       text        PRIMARY KEY,
    platform                 text        NOT NULL,
    requested_game_name      text        NOT NULL,
    requested_tag_line       text        NOT NULL,
    requested_game_name_norm text        NOT NULL,
    requested_tag_line_norm  text        NOT NULL,
    puuid                    text,
    status                   text        NOT NULL CHECK (status IN ('QUEUED','RUNNING','COMPLETED','PARTIAL','FAILED')),
    stage                    text        NOT NULL CHECK (stage IN ('QUEUED','RESOLVE_ACCOUNT','FETCH_MATCH_IDS','FETCH_MATCHES','FINALIZE')),
    scanned_count            int         NOT NULL DEFAULT 0 CHECK (scanned_count BETWEEN 0 AND 100),
    fetched_count            int         NOT NULL DEFAULT 0 CHECK (fetched_count BETWEEN 0 AND 100),
    failed_count             int         NOT NULL DEFAULT 0 CHECK (failed_count BETWEEN 0 AND 100),
    error_code               text,
    error_message            text,
    created_at               timestamptz NOT NULL DEFAULT now(),
    started_at               timestamptz,
    completed_at             timestamptz,
    updated_at               timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX tft_player_lookup_jobs_active_identity
    ON tft_player_lookup_jobs (platform, requested_game_name_norm, requested_tag_line_norm)
    WHERE status IN ('QUEUED','RUNNING');

CREATE INDEX tft_player_lookup_jobs_created
    ON tft_player_lookup_jobs (created_at DESC);

CREATE INDEX tft_player_lookup_jobs_identity_history
    ON tft_player_lookup_jobs (
        platform, requested_game_name_norm, requested_tag_line_norm, created_at DESC
    );

-- The existing tft_match_participants_history index makes the participant
-- join selective; this companion index supports the match-side time order.
CREATE INDEX tft_matches_platform_recent
    ON tft_matches (platform, game_datetime DESC, match_id DESC);
