-- Queries on game_versions + the versions column of matches.

-- name: GetLatestGameVersion :one
SELECT version, patch_start_at
FROM game_versions
WHERE is_latest = TRUE
ORDER BY patch_start_at DESC NULLS LAST
LIMIT 1;

-- name: ListGameVersions :many
SELECT version, patch_start_at, is_latest
FROM game_versions
ORDER BY patch_start_at DESC NULLS LAST, version DESC
LIMIT $1;

-- name: ListVersionsWithData :many
-- Distinct match-processing versions for matches that have completed
-- the fetch pipeline. Mirrors legacy VersionStore.GetVersionsWithData
-- exactly so /api/v1/versions stays byte-equal with /api/versions.
SELECT DISTINCT version
FROM matches
WHERE fetch_status = 'done'
  AND version IS NOT NULL
  AND version <> ''
ORDER BY version DESC;
