-- name: GetChampionIdentity :one
SELECT champion_id::int AS champion_id, MAX(champion_name)::text AS champion_name
FROM champion_detail_rollup
WHERE champion_id = @champion_id::int
GROUP BY champion_id;

-- name: ListChampionDetailBuilds :many
WITH filtered AS (
    SELECT category, stage, signature, SUM(games)::bigint AS games, SUM(wins)::bigint AS wins
    FROM champion_detail_rollup
    WHERE champion_id = @champion_id::int
      AND queue_id = @queue_id::int
      AND (@version_filter::text = '' OR version = @version_filter::text)
      AND (@region_filter::text = '' OR region = @region_filter::text)
      AND (cardinality(@avg_tiers::text[]) = 0 OR tier_bucket = ANY(@avg_tiers::text[]))
      AND (@position_filter::text = '' OR team_position = @position_filter::text)
    GROUP BY category, stage, signature
), ranked AS (
    SELECT *, SUM(games) OVER (PARTITION BY category, stage)::bigint AS eligible_games,
           ROW_NUMBER() OVER (
               PARTITION BY category, stage
               ORDER BY games DESC, (wins::float8 / games) DESC, signature
           ) AS rank
    FROM filtered
)
SELECT category, stage::int AS stage, signature, games::int AS games, wins::int AS wins,
       eligible_games::int AS eligible_games
FROM ranked
WHERE rank <= @row_limit::int
ORDER BY category, stage, rank;
