-- Hello-world query to verify sqlc codegen wiring. Real query
-- packages (rankings.sql, summoner.sql, user.sql, etc.) land in
-- Phase B alongside the service layer.

-- name: GetLatestGameVersion :one
SELECT version, patch_start_at
FROM game_versions
WHERE is_latest = TRUE
LIMIT 1;

-- name: ListGameVersions :many
SELECT version, patch_start_at, is_latest
FROM game_versions
ORDER BY patch_start_at DESC NULLS LAST, version DESC
LIMIT $1;
