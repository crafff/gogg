ALTER TABLE champion_insight_factor_buckets
    DROP COLUMN sample_players;

ALTER TABLE champion_insight_factors
    DROP CONSTRAINT champion_insight_factors_kind_check,
    DROP CONSTRAINT champion_insight_factors_direction_check;
UPDATE champion_insight_factors
SET kind = 'TRAINABLE', direction = 'HIGHER';
ALTER TABLE champion_insight_factors
    RENAME COLUMN display_order TO importance_rank;
ALTER TABLE champion_insight_factors
    ADD CONSTRAINT champion_insight_factors_kind_check
        CHECK (kind = 'TRAINABLE'),
    ADD CONSTRAINT champion_insight_factors_direction_check
        CHECK (direction = 'HIGHER');

ALTER TABLE champion_insight_cohorts
    DROP CONSTRAINT champion_insight_cohorts_cohort_scope_check,
    DROP CONSTRAINT champion_insight_cohorts_version_check;
ALTER TABLE champion_insight_cohorts
    RENAME COLUMN cohort_scope TO model_scope;
ALTER TABLE champion_insight_cohorts
    ADD CONSTRAINT champion_insight_cohorts_model_scope_check
        CHECK (model_scope IN ('CHAMPION_POSITION','POSITION_FALLBACK'));
ALTER TABLE champion_insight_cohorts
    ADD CONSTRAINT champion_insight_cohorts_version_check
        CHECK (btrim(version) <> '');
