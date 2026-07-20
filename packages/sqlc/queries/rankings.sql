-- Champion rankings served from narrow additive rollups. The refresh job
-- scans the raw match facts; online API requests only combine these tables.
--
-- Atomic tier buckets remain disjoint, so overlapping API groups such as
-- master_plus are assembled by summing MASTER, GRANDMASTER, and CHALLENGER.
-- Pick rate means the percentage of eligible matches in which the champion
-- was picked. Overall and position views therefore share total_matches as the
-- denominator; their cross-champion totals are not expected to equal 100%.

-- name: ListOverallRankings :many
WITH filtered_stats AS (
    SELECT r.*
    FROM rankings_champion_position_rollup r
    WHERE r.queue_id = @queue_id::int
      AND (@version_filter::text = '' OR r.version = @version_filter::text)
      AND (@region_filter::text  = '' OR r.region  = @region_filter::text)
      AND (cardinality(@avg_tiers::text[]) = 0 OR r.tier_bucket = ANY(@avg_tiers::text[]))
),
champ_agg AS (
    SELECT
        champion_id,
        MAX(champion_name)::text AS champion_name,
        SUM(games)::int          AS games,
        SUM(wins)::int           AS wins,
        (
            SUM(kda_contribution_sum) /
            NULLIF(SUM(games), 0)
        )::float8 AS kda
    FROM filtered_stats
    GROUP BY champion_id
),
pos_agg AS (
    SELECT champion_id, team_position, SUM(games)::bigint AS pos_games
    FROM filtered_stats
    GROUP BY champion_id, team_position
),
valid_positions AS (
    SELECT
        pa.champion_id,
        ARRAY_AGG(pa.team_position ORDER BY pa.pos_games DESC) AS positions
    FROM pos_agg pa
    INNER JOIN champ_agg ca ON ca.champion_id = pa.champion_id
    WHERE ((pa.pos_games::float8 / NULLIF(ca.games, 0)) * 100.0) >= @position_threshold::float8
    GROUP BY pa.champion_id
),
totals AS (
    SELECT
        COALESCE(SUM(r.total_matches), 0)::float8 AS total_matches
    FROM rankings_match_count_rollup r
    WHERE r.queue_id = @queue_id::int
      AND (@version_filter::text = '' OR r.version = @version_filter::text)
      AND (@region_filter::text  = '' OR r.region  = @region_filter::text)
      AND (cardinality(@avg_tiers::text[]) = 0 OR r.tier_bucket = ANY(@avg_tiers::text[]))
),
ban_agg AS (
    SELECT r.champion_id, SUM(r.ban_matches)::float8 AS ban_matches
    FROM rankings_champion_ban_rollup r
    WHERE r.queue_id = @queue_id::int
      AND (@version_filter::text = '' OR r.version = @version_filter::text)
      AND (@region_filter::text  = '' OR r.region  = @region_filter::text)
      AND (cardinality(@avg_tiers::text[]) = 0 OR r.tier_bucket = ANY(@avg_tiers::text[]))
    GROUP BY r.champion_id
)
SELECT
    ca.champion_id::int AS champion_id,
    ca.champion_name,
    COALESCE(vp.positions, ARRAY[]::text[])::text[]                                                   AS team_position,
    ca.games,
    ca.wins,
    (ca.games - ca.wins)::int                                                                        AS losses,
    ROUND(((ca.wins::float8  / NULLIF(ca.games, 0))                  * 100.0)::numeric, 2)::float8    AS win_rate,
    ROUND(((ca.games::float8 / NULLIF(t.total_matches, 0))          * 100.0)::numeric, 2)::float8    AS pick_rate,
    ROUND(((COALESCE(ba.ban_matches, 0) / NULLIF(t.total_matches, 0)) * 100.0)::numeric, 2)::float8   AS ban_rate,
    ROUND(COALESCE(ca.kda, 0)::numeric, 2)::float8                                                    AS kda,
    t.total_matches::int                                                                              AS total_matches
FROM champ_agg ca
CROSS JOIN totals t
LEFT JOIN valid_positions vp ON vp.champion_id = ca.champion_id
LEFT JOIN ban_agg ba         ON ba.champion_id = ca.champion_id
WHERE ca.games >= @min_games::int
ORDER BY win_rate DESC, pick_rate DESC, games DESC
LIMIT @row_limit::int;


-- name: ListRankingsByPosition :many
WITH filtered_stats AS (
    SELECT r.*
    FROM rankings_champion_position_rollup r
    WHERE r.queue_id = @queue_id::int
      AND r.team_position = @position_filter::text
      AND (@version_filter::text = '' OR r.version = @version_filter::text)
      AND (@region_filter::text  = '' OR r.region  = @region_filter::text)
      AND (cardinality(@avg_tiers::text[]) = 0 OR r.tier_bucket = ANY(@avg_tiers::text[]))
),
champ_agg AS (
    SELECT
        champion_id,
        MAX(champion_name)::text AS champion_name,
        SUM(games)::int          AS games,
        SUM(wins)::int           AS wins,
        (
            SUM(kda_contribution_sum) /
            NULLIF(SUM(games), 0)
        )::float8 AS kda
    FROM filtered_stats
    GROUP BY champion_id
),
totals AS (
    SELECT
        COALESCE(SUM(r.total_matches), 0)::float8 AS total_matches
    FROM rankings_match_count_rollup r
    WHERE r.queue_id = @queue_id::int
      AND (@version_filter::text = '' OR r.version = @version_filter::text)
      AND (@region_filter::text  = '' OR r.region  = @region_filter::text)
      AND (cardinality(@avg_tiers::text[]) = 0 OR r.tier_bucket = ANY(@avg_tiers::text[]))
),
ban_agg AS (
    SELECT r.champion_id, SUM(r.ban_matches)::float8 AS ban_matches
    FROM rankings_champion_ban_rollup r
    WHERE r.queue_id = @queue_id::int
      AND (@version_filter::text = '' OR r.version = @version_filter::text)
      AND (@region_filter::text  = '' OR r.region  = @region_filter::text)
      AND (cardinality(@avg_tiers::text[]) = 0 OR r.tier_bucket = ANY(@avg_tiers::text[]))
    GROUP BY r.champion_id
)
SELECT
    ca.champion_id::int AS champion_id,
    ca.champion_name,
    ca.games,
    ca.wins,
    (ca.games - ca.wins)::int                                                                        AS losses,
    ROUND(((ca.wins::float8  / NULLIF(ca.games, 0))                  * 100.0)::numeric, 2)::float8    AS win_rate,
    ROUND(((ca.games::float8 / NULLIF(t.total_matches, 0))          * 100.0)::numeric, 2)::float8    AS pick_rate,
    ROUND(((COALESCE(ba.ban_matches, 0) / NULLIF(t.total_matches, 0)) * 100.0)::numeric, 2)::float8   AS ban_rate,
    ROUND(COALESCE(ca.kda, 0)::numeric, 2)::float8                                                    AS kda,
    t.total_matches::int                                                                              AS total_matches
FROM champ_agg ca
CROSS JOIN totals t
LEFT JOIN ban_agg ba ON ba.champion_id = ca.champion_id
WHERE ca.games >= @min_games::int
ORDER BY win_rate DESC, pick_rate DESC, games DESC
LIMIT @row_limit::int;
