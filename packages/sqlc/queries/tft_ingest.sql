-- name: UpsertTFTMatch :exec
INSERT INTO tft_matches (
    match_id, data_version, platform, routing_region, queue_id, game_version,
    patch, game_datetime, game_length_seconds, map_id, tft_game_type,
    set_core_name, set_number, end_of_game_result, participant_count,
    eligible, exclusion_reason, static_revision_id
) VALUES (
    @match_id, sqlc.narg(data_version), @platform, @routing_region, @queue_id, @game_version,
    @patch, @game_datetime, sqlc.narg(game_length_seconds), sqlc.narg(map_id), sqlc.narg(tft_game_type),
    sqlc.narg(set_core_name), sqlc.narg(set_number), sqlc.narg(end_of_game_result), @participant_count,
    @eligible, sqlc.narg(exclusion_reason), sqlc.narg(static_revision_id)
)
ON CONFLICT (match_id) DO UPDATE
SET data_version = EXCLUDED.data_version,
    platform = EXCLUDED.platform,
    routing_region = EXCLUDED.routing_region,
    queue_id = EXCLUDED.queue_id,
    game_version = EXCLUDED.game_version,
    patch = EXCLUDED.patch,
    game_datetime = EXCLUDED.game_datetime,
    game_length_seconds = EXCLUDED.game_length_seconds,
    map_id = EXCLUDED.map_id,
    tft_game_type = EXCLUDED.tft_game_type,
    set_core_name = EXCLUDED.set_core_name,
    set_number = EXCLUDED.set_number,
    end_of_game_result = EXCLUDED.end_of_game_result,
    participant_count = EXCLUDED.participant_count,
    eligible = EXCLUDED.eligible,
    exclusion_reason = EXCLUDED.exclusion_reason,
    static_revision_id = EXCLUDED.static_revision_id,
    updated_at = now();

-- name: DeleteTFTMatchParticipants :exec
DELETE FROM tft_match_participants WHERE match_id = $1;

-- name: InsertTFTMatchParticipant :exec
INSERT INTO tft_match_participants (
    match_id, puuid, placement, level, gold_left, last_round,
    players_eliminated, time_eliminated_seconds, total_damage_to_players,
    companion_content_id, companion_item_id, companion_skin_id, companion_species,
    mapped_unit_count, lineup_signature, abnormal
) VALUES (
    @match_id, @puuid, @placement, sqlc.narg(level), sqlc.narg(gold_left), sqlc.narg(last_round),
    sqlc.narg(players_eliminated), sqlc.narg(time_eliminated_seconds), sqlc.narg(total_damage_to_players),
    sqlc.narg(companion_content_id), sqlc.narg(companion_item_id), sqlc.narg(companion_skin_id), sqlc.narg(companion_species),
    @mapped_unit_count, sqlc.narg(lineup_signature), @abnormal
);

-- name: InsertTFTMatchAugment :exec
INSERT INTO tft_match_augments (match_id, puuid, slot, augment_id)
VALUES (@match_id, @puuid, @slot, @augment_id);

-- name: InsertTFTMatchTrait :exec
INSERT INTO tft_match_traits (
    match_id, puuid, trait_index, trait_id, num_units, style, tier_current, tier_total
) VALUES (
    @match_id, @puuid, @trait_index, @trait_id, sqlc.narg(num_units), sqlc.narg(style),
    sqlc.narg(tier_current), sqlc.narg(tier_total)
);

-- name: InsertTFTMatchUnit :exec
INSERT INTO tft_match_units (
    match_id, puuid, unit_index, character_id, name, rarity, tier, mapped
) VALUES (
    @match_id, @puuid, @unit_index, @character_id, sqlc.narg(name),
    sqlc.narg(rarity), sqlc.narg(tier), @mapped
);

-- name: InsertTFTMatchUnitItem :exec
INSERT INTO tft_match_unit_items (match_id, puuid, unit_index, item_slot, item_id)
VALUES (@match_id, @puuid, @unit_index, @item_slot, @item_id);
