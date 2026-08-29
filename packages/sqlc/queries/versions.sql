-- Queries on game_versions + published statistics rollups.

-- name: GetLatestGameVersion :one
-- "latest" on the statistics surface means the newest version with a
-- published, non-empty rollup. Collector bundles can contain newer raw match
-- versions before tier enrichment makes them eligible for statistics.
SELECT rollup.version, gv.patch_start_at
FROM (
    SELECT DISTINCT version
    FROM rankings_match_count_rollup
    WHERE total_matches > 0
) rollup
LEFT JOIN game_versions gv ON gv.version = rollup.version
ORDER BY string_to_array(rollup.version, '.')::int[] DESC
LIMIT 1;

-- name: ListGameVersions :many
SELECT version, patch_start_at, is_latest
FROM game_versions
ORDER BY patch_start_at DESC NULLS LAST, version DESC
LIMIT $1;

-- name: ListVersionsWithData :many
-- Only expose versions that can answer the statistics queries powered by this
-- catalog. Raw-only versions remain available to summoner match history but do
-- not produce an empty option in the rankings UI.
SELECT version
FROM rankings_match_count_rollup
WHERE total_matches > 0
GROUP BY version
ORDER BY string_to_array(version, '.')::int[] DESC;
