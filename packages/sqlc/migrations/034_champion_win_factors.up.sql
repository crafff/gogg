-- Immutable, revisioned publications for champion win-factor statistics.
-- The first schema version publishes descriptive JUNGLE metrics only. Model-
-- adjusted and causal estimates must use a later algorithm/evidence version.

CREATE TABLE champion_insight_publications (
    id                     bigserial   PRIMARY KEY,
    revision               text        NOT NULL UNIQUE CHECK (btrim(revision) <> ''),
    dataset_key            text        NOT NULL CHECK (dataset_key = 'ranked-solo-v1'),
    algorithm_version      text        NOT NULL CHECK (algorithm_version = 'OBSERVED_PERCENTILES_V1'),
    feature_schema_version int         NOT NULL CHECK (feature_schema_version = 1),
    status                 text        NOT NULL CHECK (status IN ('building','published','superseded')),
    source_matches         bigint      NOT NULL CHECK (source_matches >= 0),
    eligible_matches       bigint      NOT NULL CHECK (eligible_matches >= 0),
    source_participants    bigint      NOT NULL CHECK (source_participants >= 0),
    data_through           timestamptz,
    coverage               jsonb       NOT NULL DEFAULT '{}'::jsonb,
    created_at             timestamptz NOT NULL DEFAULT now(),
    published_at           timestamptz,
    CHECK (eligible_matches <= source_matches),
    CHECK ((status = 'building' AND published_at IS NULL) OR
           (status <> 'building' AND published_at IS NOT NULL))
);

CREATE UNIQUE INDEX champion_insight_one_published_dataset
    ON champion_insight_publications(dataset_key)
    WHERE status = 'published';

CREATE TABLE champion_insight_cohorts (
    id                bigserial PRIMARY KEY,
    publication_id    bigint    NOT NULL REFERENCES champion_insight_publications(id) ON DELETE CASCADE,
    queue_id           int       NOT NULL CHECK (queue_id = 420),
    version            text      NOT NULL CHECK (btrim(version) <> ''),
    region_scope       text      NOT NULL CHECK (region_scope IN ('ALL','KR','NA1')),
    tier_group         text      NOT NULL CHECK (tier_group IN (
        'ALL','MASTER','MASTER_PLUS','GRANDMASTER','GRANDMASTER_PLUS','CHALLENGER'
    )),
    champion_id        int       NOT NULL CHECK (champion_id > 0),
    champion_name      text      NOT NULL CHECK (btrim(champion_name) <> ''),
    team_position      text      NOT NULL CHECK (team_position IN ('TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY')),
    sample_games       bigint    NOT NULL CHECK (sample_games > 0),
    sample_players     bigint    NOT NULL CHECK (sample_players >= 0 AND sample_players <= sample_games),
    availability       text      NOT NULL CHECK (availability IN ('AVAILABLE','INSUFFICIENT_SAMPLE')),
    reason_code        text,
    model_scope        text      NOT NULL CHECK (model_scope IN ('CHAMPION_POSITION','POSITION_FALLBACK')),
    CHECK ((availability = 'AVAILABLE' AND reason_code IS NULL) OR
           (availability = 'INSUFFICIENT_SAMPLE' AND reason_code IS NOT NULL)),
    UNIQUE (publication_id, queue_id, version, region_scope, tier_group, champion_id, team_position)
);

CREATE INDEX champion_insight_cohort_lookup
    ON champion_insight_cohorts (
        publication_id, champion_id, queue_id, version,
        region_scope, tier_group, team_position
    );

CREATE TABLE champion_insight_factors (
    cohort_id       bigint   NOT NULL REFERENCES champion_insight_cohorts(id) ON DELETE CASCADE,
    metric_key      text     NOT NULL CHECK (metric_key IN (
        'JUNGLE_CS_10', 'JUNGLE_CS_GAIN_10_15',
        'DAMAGE_TO_CHAMPIONS_10', 'DAMAGE_TO_CHAMPIONS_GAIN_10_15'
    )),
    kind            text     NOT NULL CHECK (kind = 'TRAINABLE'),
    start_minute    smallint NOT NULL CHECK (start_minute IN (0, 10)),
    end_minute      smallint NOT NULL CHECK (end_minute IN (10, 15) AND end_minute > start_minute),
    unit            text     NOT NULL CHECK (unit IN ('COUNT','DAMAGE')),
    direction       text     NOT NULL CHECK (direction = 'HIGHER'),
    p50             numeric  NOT NULL,
    p70             numeric  NOT NULL,
    p90             numeric  NOT NULL,
    evidence_grade  text     NOT NULL CHECK (evidence_grade = 'OBSERVED'),
    importance_rank smallint NOT NULL CHECK (importance_rank BETWEEN 1 AND 4),
    PRIMARY KEY (cohort_id, metric_key),
    CHECK (p50 <= p70 AND p70 <= p90)
);

CREATE TABLE champion_insight_factor_buckets (
    cohort_id               bigint   NOT NULL,
    metric_key              text     NOT NULL,
    ordinal                 smallint NOT NULL CHECK (ordinal BETWEEN 1 AND 10),
    lower_bound             numeric  NOT NULL,
    upper_bound             numeric  NOT NULL CHECK (upper_bound >= lower_bound),
    games                   bigint   NOT NULL CHECK (games > 0),
    wins                    bigint   NOT NULL CHECK (wins >= 0 AND wins <= games),
    adjusted_win_rate_delta numeric,
    ci_low                  numeric,
    ci_high                 numeric,
    PRIMARY KEY (cohort_id, metric_key, ordinal),
    FOREIGN KEY (cohort_id, metric_key)
        REFERENCES champion_insight_factors(cohort_id, metric_key) ON DELETE CASCADE,
    CHECK ((ci_low IS NULL AND ci_high IS NULL) OR
           (ci_low IS NOT NULL AND ci_high IS NOT NULL AND ci_low <= ci_high))
);
