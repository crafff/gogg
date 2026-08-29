-- TFT player identity, refresh jobs, and read-optimised history assembly.

-- name: GetTFTPlayerIdentity :one
SELECT *
FROM tft_player_identities
WHERE platform = @platform
  AND lower(game_name) = lower(@game_name)
  AND upper(tag_line) = upper(@tag_line::text)
LIMIT 1;

-- name: UpsertTFTPlayerIdentity :one
INSERT INTO tft_player_identities (
    platform, puuid, game_name, tag_line, identity_refreshed_at, matches_refreshed_at
) VALUES (
    @platform, @puuid, @game_name, @tag_line, @identity_refreshed_at, sqlc.narg(matches_refreshed_at)
)
ON CONFLICT (platform, puuid) DO UPDATE
SET game_name = EXCLUDED.game_name,
    tag_line = EXCLUDED.tag_line,
    identity_refreshed_at = EXCLUDED.identity_refreshed_at,
    matches_refreshed_at = COALESCE(EXCLUDED.matches_refreshed_at, tft_player_identities.matches_refreshed_at),
    updated_at = now()
RETURNING *;

-- name: DeleteStaleTFTPlayerIdentityForRiotID :exec
DELETE FROM tft_player_identities
WHERE platform = @platform
  AND lower(game_name) = lower(@game_name)
  AND upper(tag_line) = upper(@tag_line::text)
  AND puuid <> @puuid;

-- name: MarkTFTPlayerMatchesRefreshed :exec
UPDATE tft_player_identities
SET matches_refreshed_at = @refreshed_at, updated_at = now()
WHERE platform = @platform AND puuid = @puuid;

-- name: GetTFTPlayerLookupJob :one
SELECT * FROM tft_player_lookup_jobs WHERE id = @id;

-- name: FindActiveTFTPlayerLookupJob :one
SELECT *
FROM tft_player_lookup_jobs
WHERE platform = @platform
  AND requested_game_name_norm = @game_name_norm
  AND requested_tag_line_norm = @tag_line_norm
  AND status IN ('QUEUED','RUNNING')
ORDER BY created_at DESC
LIMIT 1;

-- name: FindLatestTFTPlayerLookupJob :one
SELECT *
FROM tft_player_lookup_jobs
WHERE platform = @platform
  AND requested_game_name_norm = @game_name_norm
  AND requested_tag_line_norm = @tag_line_norm
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateTFTPlayerLookupJob :one
INSERT INTO tft_player_lookup_jobs (
    id, platform, requested_game_name, requested_tag_line,
    requested_game_name_norm, requested_tag_line_norm, status, stage
) VALUES (
    @id, @platform, @game_name, @tag_line,
    @game_name_norm, @tag_line_norm, 'QUEUED', 'QUEUED'
)
RETURNING *;

-- name: MarkTFTPlayerLookupJobRunning :exec
UPDATE tft_player_lookup_jobs
SET status = 'RUNNING', stage = @stage, puuid = COALESCE(sqlc.narg(puuid), puuid),
    started_at = COALESCE(started_at, now()), updated_at = now()
WHERE id = @id;

-- name: UpdateTFTPlayerLookupJobProgress :exec
UPDATE tft_player_lookup_jobs
SET stage = @stage,
    scanned_count = @scanned_count,
    fetched_count = @fetched_count,
    failed_count = @failed_count,
    updated_at = now()
WHERE id = @id;

-- name: FinishTFTPlayerLookupJob :exec
UPDATE tft_player_lookup_jobs
SET status = @status, stage = 'FINALIZE',
    error_code = sqlc.narg(error_code), error_message = sqlc.narg(error_message),
    completed_at = now(), updated_at = now()
WHERE id = @id;

-- name: DeleteExpiredTFTPlayerLookupJobs :execrows
DELETE FROM tft_player_lookup_jobs
WHERE created_at < now() - interval '7 days'
  AND status IN ('COMPLETED','PARTIAL','FAILED');

-- name: ListTFTPlayerMatches :many
SELECT m.match_id, m.platform, m.routing_region, m.queue_id, m.game_version,
       m.patch, m.game_datetime, m.game_length_seconds, m.map_id,
       m.tft_game_type, m.set_core_name, m.set_number, m.end_of_game_result,
       m.participant_count, m.eligible, m.exclusion_reason,
       p.puuid, p.placement, p.level, p.gold_left, p.last_round,
       p.players_eliminated, p.time_eliminated_seconds,
       p.total_damage_to_players, p.companion_content_id,
       p.companion_item_id, p.companion_skin_id, p.companion_species,
       p.abnormal
FROM tft_match_participants p
JOIN tft_matches m ON m.match_id = p.match_id
WHERE p.puuid = @puuid
  AND m.platform = @platform
  AND (@queue_id::int = 0 OR m.queue_id = @queue_id)
  AND (
      @before_time::timestamptz IS NULL
      OR (m.game_datetime, m.match_id) < (@before_time::timestamptz, @before_match_id::text)
  )
ORDER BY m.game_datetime DESC, m.match_id DESC
LIMIT @row_limit;

-- name: ListTFTMatchParticipantsForHistory :many
SELECT p.match_id, p.puuid, identities.game_name, identities.tag_line,
       p.placement, p.level, p.gold_left, p.last_round,
       p.players_eliminated, p.time_eliminated_seconds,
       p.total_damage_to_players, p.companion_content_id,
       p.companion_item_id, p.companion_skin_id, p.companion_species,
       p.abnormal
FROM tft_match_participants p
JOIN tft_matches m ON m.match_id = p.match_id
LEFT JOIN tft_player_identities identities
       ON identities.platform = m.platform AND identities.puuid = p.puuid
WHERE p.match_id = ANY(@match_ids::text[])
ORDER BY p.match_id, p.placement, p.puuid;

-- name: ListTFTMatchAugmentsForHistory :many
SELECT match_id, puuid, slot, augment_id
FROM tft_match_augments
WHERE match_id = ANY(@match_ids::text[])
ORDER BY match_id, puuid, slot;

-- name: ListTFTMatchTraitsForHistory :many
SELECT match_id, puuid, trait_index, trait_id, num_units, style, tier_current, tier_total
FROM tft_match_traits
WHERE match_id = ANY(@match_ids::text[])
ORDER BY match_id, puuid, trait_index;

-- name: ListTFTMatchUnitsForHistory :many
SELECT match_id, puuid, unit_index, character_id, name, rarity, tier, mapped
FROM tft_match_units
WHERE match_id = ANY(@match_ids::text[])
ORDER BY match_id, puuid, unit_index;

-- name: ListTFTMatchUnitItemsForHistory :many
SELECT match_id, puuid, unit_index, item_slot, item_id
FROM tft_match_unit_items
WHERE match_id = ANY(@match_ids::text[])
ORDER BY match_id, puuid, unit_index, item_slot;
