-- Queries on the region column of matches.

-- name: ListRegionsWithData :many
-- Distinct regions that have completed matches. Mirrors legacy
-- RankingStore.GetRegionsWithData exactly so /api/v1/regions stays
-- byte-equal with /api/regions.
SELECT DISTINCT region
FROM matches
WHERE fetch_status = 'done'
  AND region IS NOT NULL
  AND region <> ''
ORDER BY region;
