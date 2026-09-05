-- Atomic rankings aggregates built only from fully validated ranked-solo
-- matches. The refresh job owns eligibility; table constraints keep corrupt
-- aggregate rows from being published even if the builder regresses.

CREATE TABLE rankings_champion_position_rollup (
    queue_id                 int              NOT NULL CHECK (queue_id = 420),
    version                  text             NOT NULL CHECK (btrim(version) <> ''),
    region                   text             NOT NULL CHECK (btrim(region) <> ''),
    tier_bucket              text             NOT NULL CHECK (tier_bucket IN (
        'IRON', 'BRONZE', 'SILVER', 'GOLD', 'PLATINUM', 'EMERALD',
        'DIAMOND', 'MASTER', 'GRANDMASTER', 'CHALLENGER'
    )),
    champion_id              int              NOT NULL CHECK (champion_id > 0),
    champion_name            text             NOT NULL CHECK (btrim(champion_name) <> ''),
    team_position            text             NOT NULL CHECK (team_position IN (
        'TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY'
    )),
    games                    bigint           NOT NULL CHECK (games > 0),
    wins                     bigint           NOT NULL CHECK (wins >= 0 AND wins <= games),
    kda_contribution_sum     double precision NOT NULL CHECK (kda_contribution_sum >= 0),
    PRIMARY KEY (queue_id, version, region, tier_bucket, champion_id, team_position)
);

CREATE TABLE rankings_champion_ban_rollup (
    queue_id                 int    NOT NULL CHECK (queue_id = 420),
    version                  text   NOT NULL CHECK (btrim(version) <> ''),
    region                   text   NOT NULL CHECK (btrim(region) <> ''),
    tier_bucket              text   NOT NULL CHECK (tier_bucket IN (
        'IRON', 'BRONZE', 'SILVER', 'GOLD', 'PLATINUM', 'EMERALD',
        'DIAMOND', 'MASTER', 'GRANDMASTER', 'CHALLENGER'
    )),
    champion_id              int    NOT NULL CHECK (champion_id > 0),
    ban_matches              bigint NOT NULL CHECK (ban_matches > 0),
    PRIMARY KEY (queue_id, version, region, tier_bucket, champion_id)
);

CREATE TABLE rankings_match_count_rollup (
    queue_id                 int    NOT NULL CHECK (queue_id = 420),
    version                  text   NOT NULL CHECK (btrim(version) <> ''),
    region                   text   NOT NULL CHECK (btrim(region) <> ''),
    tier_bucket              text   NOT NULL CHECK (tier_bucket IN (
        'IRON', 'BRONZE', 'SILVER', 'GOLD', 'PLATINUM', 'EMERALD',
        'DIAMOND', 'MASTER', 'GRANDMASTER', 'CHALLENGER'
    )),
    total_matches            bigint NOT NULL CHECK (total_matches > 0),
    PRIMARY KEY (queue_id, version, region, tier_bucket)
);

CREATE TABLE rankings_rollup_state (
    singleton                           boolean     PRIMARY KEY DEFAULT true CHECK (singleton),
    refreshed_at                        timestamptz NOT NULL,
    data_through                        timestamptz,
    source_completed_matches            bigint      NOT NULL CHECK (source_completed_matches >= 0),
    eligible_matches                    bigint      NOT NULL CHECK (eligible_matches >= 0),
    excluded_metadata_matches           bigint      NOT NULL CHECK (excluded_metadata_matches >= 0),
    excluded_tier_matches               bigint      NOT NULL CHECK (excluded_tier_matches >= 0),
    excluded_duration_matches           bigint      NOT NULL CHECK (excluded_duration_matches >= 0),
    excluded_participant_shape_matches  bigint      NOT NULL CHECK (excluded_participant_shape_matches >= 0),
    excluded_participant_facts_matches  bigint      NOT NULL CHECK (excluded_participant_facts_matches >= 0),
    excluded_ban_shape_matches          bigint      NOT NULL CHECK (excluded_ban_shape_matches >= 0),
    champion_position_rows              bigint      NOT NULL CHECK (champion_position_rows >= 0),
    ban_rows                            bigint      NOT NULL CHECK (ban_rows >= 0),
    match_count_rows                    bigint      NOT NULL CHECK (match_count_rows >= 0),
    CHECK (
        source_completed_matches = eligible_matches
            + excluded_metadata_matches
            + excluded_tier_matches
            + excluded_duration_matches
            + excluded_participant_shape_matches
            + excluded_participant_facts_matches
            + excluded_ban_shape_matches
    )
);
