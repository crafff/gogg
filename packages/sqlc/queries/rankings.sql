-- Champion rankings: overall (aggregated across all positions) and
-- by-position. Mirrors legacy internal/server/rankings.go verbatim so
-- /api/v1/rankings/champions stays byte-equal with /api/rankings/champions
-- through the Phase D cutover.
--
-- The filtered_matches CTE is duplicated between the two queries
-- because sqlc emits one Go function per query and has no cross-query
-- CTE sharing. Folding it into a postgres view is a future optimisation
-- (was already a code smell in legacy per ADR-0002); doing it during
-- Phase B is out of scope — we ship parity first, refactor later.
--
-- Parameter semantics:
--   queue_id            : exact match on matches.queue_id (e.g. 420 for ranked solo)
--   version_filter      : exact match on matches.version; '' means all
--   region_filter       : exact match on matches.region;  '' means all
--   avg_tiers           : whitelist of matches.avg_tier; empty slice means all
--   position_threshold  : minimum % of a champion's games to keep that position
--   position_filter     : ONLY for by-position; '' for overall (unused)
--   min_games           : drop champions with fewer than N games
--   row_limit           : -1 means unlimited (becomes LIMIT NULL via NULLIF)

-- name: ListOverallRankings :many
WITH filtered_matches AS (
    SELECT m.match_id
    FROM matches m
    WHERE m.queue_id = @queue_id::int
      AND (@version_filter::text = '' OR m.version = @version_filter::text)
      AND (@region_filter::text  = '' OR m.region  = @region_filter::text)
      AND m.fetch_status = 'done'
      AND (cardinality(@avg_tiers::text[]) = 0 OR m.avg_tier = ANY(@avg_tiers::text[]))
),
base AS (
    SELECT
        mp.match_id,
        mp.champion_id,
        mp.champion_name,
        mp.team_position,
        mp.win,
        mp.kills,
        mp.deaths,
        mp.assists
    FROM match_participants mp
    INNER JOIN filtered_matches fm ON fm.match_id = mp.match_id
    WHERE mp.champion_id > 0
),
totals AS (
    SELECT
        COUNT(*)::float8                 AS total_participants,
        COUNT(DISTINCT match_id)::float8 AS total_matches
    FROM base
),
champ_agg AS (
    SELECT
        -- The base CTE filters champion_id > 0, so NULLs are
        -- already excluded; COALESCE(..., 0) is a sqlc type-hint
        -- only so the generated row uses int32 instead of *int32.
        COALESCE(b.champion_id, 0)::int                                     AS champion_id,
        COALESCE(MAX(b.champion_name), '')::text                            AS champion_name,
        COUNT(*)::int                                                       AS games,
        SUM(CASE WHEN b.win THEN 1 ELSE 0 END)::int                         AS wins,
        AVG((b.kills + b.assists)::float8 / GREATEST(b.deaths, 1))::float8  AS kda
    FROM base b
    GROUP BY b.champion_id
),
pos_agg AS (
    SELECT b.champion_id, b.team_position, COUNT(*)::int AS pos_games
    FROM base b
    GROUP BY b.champion_id, b.team_position
),
valid_positions AS (
    SELECT
        pa.champion_id,
        ARRAY_AGG(pa.team_position ORDER BY pa.pos_games DESC) AS positions
    FROM pos_agg pa
    INNER JOIN champ_agg ca ON pa.champion_id = ca.champion_id
    WHERE ((pa.pos_games::float8 / NULLIF(ca.games, 0)) * 100.0) >= @position_threshold::float8
    GROUP BY pa.champion_id
),
ban_agg AS (
    SELECT b.champion_id, COUNT(DISTINCT b.match_id)::float8 AS ban_matches
    FROM match_bans b
    INNER JOIN filtered_matches fm ON fm.match_id = b.match_id
    GROUP BY b.champion_id
)
SELECT
    ca.champion_id,
    ca.champion_name,
    COALESCE(vp.positions, ARRAY[]::text[])::text[]                                                   AS team_position,
    ca.games,
    ca.wins,
    (ca.games - ca.wins)                                                                              AS losses,
    ROUND(((ca.wins::float8  / NULLIF(ca.games, 0))                  * 100.0)::numeric, 2)::float8    AS win_rate,
    ROUND(((ca.games::float8 / NULLIF(t.total_matches, 0))           * 100.0)::numeric, 2)::float8    AS pick_rate,
    ROUND(((COALESCE(ba.ban_matches, 0) / NULLIF(t.total_matches, 0)) * 100.0)::numeric, 2)::float8   AS ban_rate,
    ROUND(ca.kda::numeric, 2)::float8                                                                 AS kda,
    t.total_matches::int                                                                              AS total_matches
FROM champ_agg ca
CROSS JOIN totals t
LEFT JOIN valid_positions vp ON vp.champion_id = ca.champion_id
LEFT JOIN ban_agg ba         ON ba.champion_id = ca.champion_id
WHERE ca.games >= @min_games::int
ORDER BY win_rate DESC, pick_rate DESC, games DESC
LIMIT NULLIF(@row_limit::int, -1);


-- name: ListRankingsByPosition :many
WITH filtered_matches AS (
    SELECT m.match_id
    FROM matches m
    WHERE m.queue_id = @queue_id::int
      AND (@version_filter::text = '' OR m.version = @version_filter::text)
      AND (@region_filter::text  = '' OR m.region  = @region_filter::text)
      AND m.fetch_status = 'done'
      AND (cardinality(@avg_tiers::text[]) = 0 OR m.avg_tier = ANY(@avg_tiers::text[]))
),
base AS (
    SELECT
        mp.match_id,
        mp.champion_id,
        mp.champion_name,
        mp.win,
        mp.kills,
        mp.deaths,
        mp.assists
    FROM match_participants mp
    INNER JOIN filtered_matches fm ON fm.match_id = mp.match_id
    WHERE mp.champion_id > 0
      AND mp.team_position = @position_filter::text
),
totals AS (
    SELECT
        COUNT(*)::float8                 AS total_participants,
        COUNT(DISTINCT match_id)::float8 AS total_matches
    FROM base
),
champ_agg AS (
    SELECT
        -- The base CTE filters champion_id > 0, so NULLs are
        -- already excluded; COALESCE(..., 0) is a sqlc type-hint
        -- only so the generated row uses int32 instead of *int32.
        COALESCE(b.champion_id, 0)::int                                     AS champion_id,
        COALESCE(MAX(b.champion_name), '')::text                            AS champion_name,
        COUNT(*)::int                                                       AS games,
        SUM(CASE WHEN b.win THEN 1 ELSE 0 END)::int                         AS wins,
        AVG((b.kills + b.assists)::float8 / GREATEST(b.deaths, 1))::float8  AS kda
    FROM base b
    GROUP BY b.champion_id
),
ban_agg AS (
    SELECT b.champion_id, COUNT(DISTINCT b.match_id)::float8 AS ban_matches
    FROM match_bans b
    INNER JOIN filtered_matches fm ON fm.match_id = b.match_id
    GROUP BY b.champion_id
)
SELECT
    ca.champion_id,
    ca.champion_name,
    ca.games,
    ca.wins,
    (ca.games - ca.wins)                                                                              AS losses,
    ROUND(((ca.wins::float8  / NULLIF(ca.games, 0))                  * 100.0)::numeric, 2)::float8    AS win_rate,
    ROUND(((ca.games::float8 / NULLIF(t.total_matches, 0))           * 100.0)::numeric, 2)::float8    AS pick_rate,
    ROUND(((COALESCE(ba.ban_matches, 0) / NULLIF(t.total_matches, 0)) * 100.0)::numeric, 2)::float8   AS ban_rate,
    ROUND(ca.kda::numeric, 2)::float8                                                                 AS kda,
    t.total_matches::int                                                                              AS total_matches
FROM champ_agg ca
CROSS JOIN totals t
LEFT JOIN ban_agg ba ON ba.champion_id = ca.champion_id
WHERE ca.games >= @min_games::int
ORDER BY win_rate DESC, pick_rate DESC, games DESC
LIMIT NULLIF(@row_limit::int, -1);
