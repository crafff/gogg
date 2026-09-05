-- name: GetLatestChampionInsightVersion :one
SELECT c.version
FROM champion_insight_cohorts c
JOIN champion_insight_publications p ON p.id=c.publication_id
WHERE p.dataset_key='ranked-solo-v1' AND p.status='published'
  AND c.availability='AVAILABLE'
GROUP BY c.version
ORDER BY split_part(c.version, '.', 1)::int DESC,
         split_part(c.version, '.', 2)::int DESC
LIMIT 1;

-- name: GetPublishedChampionInsightIdentity :one
SELECT c.champion_id, MAX(c.champion_name)::text AS champion_name
FROM champion_insight_cohorts c
JOIN champion_insight_publications p ON p.id=c.publication_id
WHERE p.dataset_key='ranked-solo-v1' AND p.status='published'
  AND c.champion_id=sqlc.arg(champion_id)::int
GROUP BY c.champion_id;

-- name: GetPublishedChampionInsightCohort :one
SELECT c.id AS cohort_id, c.champion_id, c.champion_name, c.team_position,
       c.sample_games, c.sample_players, c.availability, c.reason_code, c.cohort_scope,
       p.revision, p.algorithm_version, p.data_through, p.published_at
FROM champion_insight_cohorts c
JOIN champion_insight_publications p ON p.id=c.publication_id
WHERE p.dataset_key='ranked-solo-v1' AND p.status='published'
  AND c.champion_id=sqlc.arg(champion_id)::int
  AND c.queue_id=sqlc.arg(queue_id)::int
  AND c.version=sqlc.arg(version)::text
  AND c.region_scope=sqlc.arg(region_scope)::text
  AND c.tier_group=sqlc.arg(tier_group)::text
  AND c.team_position=sqlc.arg(team_position)::text;

-- name: ListPublishedChampionInsightFactors :many
SELECT f.metric_key, f.kind, f.start_minute::int AS start_minute,
       f.end_minute::int AS end_minute, f.unit, f.direction,
       f.p50::float8 AS p50, f.p70::float8 AS p70, f.p90::float8 AS p90,
       f.evidence_grade, f.display_order::int AS display_order,
       b.ordinal::int AS ordinal, b.lower_bound::float8 AS lower_bound,
       b.upper_bound::float8 AS upper_bound, b.games::bigint AS games,
       b.wins::bigint AS wins, b.sample_players::bigint AS sample_players,
       b.adjusted_win_rate_delta,
       b.ci_low, b.ci_high
FROM champion_insight_factors f
JOIN champion_insight_factor_buckets b
  ON b.cohort_id=f.cohort_id AND b.metric_key=f.metric_key
WHERE f.cohort_id=sqlc.arg(cohort_id)::bigint
ORDER BY f.display_order, f.metric_key, b.ordinal;
