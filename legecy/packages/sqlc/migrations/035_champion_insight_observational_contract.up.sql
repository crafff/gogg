-- Keep the first champion-insight publication explicitly observational. Bucket
-- privacy gates need both match and distinct-player counts.

ALTER TABLE champion_insight_cohorts
    DROP CONSTRAINT champion_insight_cohorts_model_scope_check;
ALTER TABLE champion_insight_cohorts
    DROP CONSTRAINT champion_insight_cohorts_version_check;
ALTER TABLE champion_insight_cohorts
    RENAME COLUMN model_scope TO cohort_scope;
UPDATE champion_insight_cohorts SET cohort_scope = 'CHAMPION_POSITION';
ALTER TABLE champion_insight_cohorts
    ADD CONSTRAINT champion_insight_cohorts_cohort_scope_check
    CHECK (cohort_scope = 'CHAMPION_POSITION');
ALTER TABLE champion_insight_cohorts
    ADD CONSTRAINT champion_insight_cohorts_version_check
    CHECK (version ~ '^[0-9]+\.[0-9]+$');

ALTER TABLE champion_insight_factors
    DROP CONSTRAINT champion_insight_factors_kind_check,
    DROP CONSTRAINT champion_insight_factors_direction_check;
ALTER TABLE champion_insight_factors
    RENAME COLUMN importance_rank TO display_order;
UPDATE champion_insight_factors
SET kind = 'BEHAVIOR_METRIC', direction = 'OBSERVED_TREND';
ALTER TABLE champion_insight_factors
    ADD CONSTRAINT champion_insight_factors_kind_check
        CHECK (kind = 'BEHAVIOR_METRIC'),
    ADD CONSTRAINT champion_insight_factors_direction_check
        CHECK (direction = 'OBSERVED_TREND');

ALTER TABLE champion_insight_factor_buckets
    ADD COLUMN sample_players bigint NOT NULL DEFAULT 0
        CHECK (sample_players >= 0 AND sample_players <= games);
ALTER TABLE champion_insight_factor_buckets
    ALTER COLUMN sample_players DROP DEFAULT;

-- Existing publications cannot be grandfathered into the stronger privacy
-- contract: their historical buckets have no recoverable player membership.
-- Fail closed until the revisioned builder creates a fully gated publication.
UPDATE champion_insight_publications p
SET status = 'superseded',
    coverage = p.coverage || '{"invalidatedBy":"035_bucket_player_gate"}'::jsonb
WHERE p.status = 'published'
  AND EXISTS (
      SELECT 1
      FROM champion_insight_factor_buckets b
      JOIN champion_insight_cohorts c ON c.id = b.cohort_id
      WHERE c.publication_id = p.id AND b.sample_players = 0
  );
