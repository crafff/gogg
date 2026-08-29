-- Queries on regions with published statistics rollups.

-- name: ListRegionsWithData :many
-- Keep the rankings filter catalog aligned with slices that can return data.
SELECT DISTINCT region
FROM rankings_match_count_rollup
WHERE total_matches > 0
  AND region IS NOT NULL
  AND region <> ''
ORDER BY region;
