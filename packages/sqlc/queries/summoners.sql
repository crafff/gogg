-- Summoner search reads and asynchronous refresh bookkeeping.

-- name: GetSummonerByIdentity :one
SELECT p.puuid,
       p.game_name,
       p.tag_line,
       p.region,
       sp.profile_icon_id,
       sp.summoner_level,
       sp.identity_refreshed_at,
       sp.matches_refreshed_at
FROM players p
LEFT JOIN summoner_profiles sp ON sp.puuid = p.puuid AND sp.region = p.region
WHERE p.region = @region
  AND lower(p.game_name) = lower(@game_name)
  AND upper(p.tag_line) = upper(@tag_line::text)
ORDER BY COALESCE(sp.updated_at, p.updated_at) DESC
LIMIT 1;

-- name: GetSummonerByPUUID :one
SELECT p.puuid,
       p.game_name,
       p.tag_line,
       p.region,
       sp.profile_icon_id,
       sp.summoner_level,
       sp.identity_refreshed_at,
       sp.matches_refreshed_at
FROM players p
LEFT JOIN summoner_profiles sp ON sp.puuid = p.puuid AND sp.region = p.region
WHERE p.region = @region AND p.puuid = @puuid;

-- name: ListSummonerCurrentRanks :many
SELECT queue_type, tier, division, league_points, wins, losses, refreshed_at
FROM summoner_current_ranks
WHERE region = @region AND puuid = @puuid
ORDER BY queue_type;

-- name: ListSummonerMatches :many
WITH recent_matches AS (
    SELECT puuid, region, match_id, queue_id, game_start_ts
    FROM player_match_index
    WHERE player_match_index.puuid = @puuid
      AND player_match_index.region = @region
    ORDER BY game_start_ts DESC, match_id DESC
    LIMIT 100
)
SELECT pmi.match_id,
       pmi.queue_id,
       pmi.game_start_ts,
       m.game_end_ts,
       m.game_duration,
       m.version,
       m.game_version,
       m.end_of_game_result,
       m.avg_tier,
       m.avg_division,
       m.tier_coverage,
       mp.team_position,
       mp.individual_position,
       mp.win,
       mp.champion_id,
       mp.champion_name,
       mp.champ_level,
       mp.kills,
       mp.deaths,
       mp.assists,
       mp.total_minions_killed,
       mp.neutral_minions_killed,
       mp.gold_earned,
       mp.total_damage_dealt_to_champions,
       mp.vision_score,
       mp.item0, mp.item1, mp.item2, mp.item3, mp.item4, mp.item5, mp.item6,
       mp.summoner1_id, mp.summoner2_id,
       mp.game_ended_in_early_surrender,
       mp.game_ended_in_surrender,
       pk.style0, pk.style1,
       pk.perk0, pk.perk1, pk.perk2, pk.perk3, pk.perk4, pk.perk5,
       pk.stat_offense, pk.stat_flex, pk.stat_defense
FROM recent_matches pmi
JOIN matches m ON m.match_id = pmi.match_id
JOIN match_participants mp ON mp.match_id = pmi.match_id AND mp.puuid = pmi.puuid
LEFT JOIN match_perks pk ON pk.match_id = pmi.match_id AND pk.puuid = pmi.puuid
WHERE pmi.queue_id = ANY(@queue_ids::int[])
  AND (
       @before_time::timestamptz IS NULL
       OR (pmi.game_start_ts, pmi.match_id) < (@before_time::timestamptz, @before_match_id::text)
  )
ORDER BY pmi.game_start_ts DESC, pmi.match_id DESC
LIMIT @row_limit;

-- name: ListSummonerMatchParticipants :many
SELECT mp.match_id,
       mp.participant_id,
       mp.puuid,
       p.game_name,
       p.tag_line,
       mp.tier_at_match,
       mp.division_at_match,
       mp.lp_at_match,
       mp.tier_snapshot_delta_h,
       mp.team_id,
       mp.team_position,
       mp.individual_position,
       mp.win,
       mp.champion_id,
       mp.champion_name,
       mp.champ_level,
       mp.kills,
       mp.deaths,
       mp.assists,
       mp.total_minions_killed,
       mp.neutral_minions_killed,
       mp.gold_earned,
       mp.total_damage_dealt_to_champions,
       mp.vision_score,
       mp.item0, mp.item1, mp.item2, mp.item3, mp.item4, mp.item5, mp.item6,
       mp.summoner1_id, mp.summoner2_id,
       pk.style0, pk.style1,
       pk.perk0, pk.perk1, pk.perk2, pk.perk3, pk.perk4, pk.perk5
FROM match_participants mp
LEFT JOIN players p ON p.puuid = mp.puuid
LEFT JOIN match_perks pk ON pk.match_id = mp.match_id AND pk.puuid = mp.puuid
WHERE mp.match_id = ANY(@match_ids::text[])
ORDER BY mp.match_id, mp.team_id, mp.participant_id;

-- name: GetSummonerLookupJob :one
SELECT * FROM summoner_lookup_jobs WHERE id = @id;

-- name: FindActiveSummonerLookupJob :one
SELECT *
FROM summoner_lookup_jobs
WHERE region = @region
  AND requested_game_name_norm = @game_name_norm
  AND requested_tag_line_norm = @tag_line_norm
  AND status IN ('QUEUED', 'RUNNING')
ORDER BY created_at DESC
LIMIT 1;

-- name: FindLatestSummonerLookupJob :one
SELECT *
FROM summoner_lookup_jobs
WHERE region = @region
  AND requested_game_name_norm = @game_name_norm
  AND requested_tag_line_norm = @tag_line_norm
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateSummonerLookupJob :one
INSERT INTO summoner_lookup_jobs (
    id, region,
    requested_game_name, requested_tag_line,
    requested_game_name_norm, requested_tag_line_norm,
    status, stage
) VALUES (
    @id, @region,
    @game_name, @tag_line,
    @game_name_norm, @tag_line_norm,
    'QUEUED', 'QUEUED'
)
RETURNING *;

-- name: UpsertSummonerProfile :exec
INSERT INTO summoner_profiles (
    region, puuid, profile_icon_id, summoner_level,
    identity_refreshed_at, matches_refreshed_at
) VALUES (
    @region, @puuid, @profile_icon_id, @summoner_level,
    @identity_refreshed_at, @matches_refreshed_at
)
ON CONFLICT (region, puuid) DO UPDATE
SET profile_icon_id = COALESCE(EXCLUDED.profile_icon_id, summoner_profiles.profile_icon_id),
    summoner_level = COALESCE(EXCLUDED.summoner_level, summoner_profiles.summoner_level),
    identity_refreshed_at = COALESCE(EXCLUDED.identity_refreshed_at, summoner_profiles.identity_refreshed_at),
    matches_refreshed_at = COALESCE(EXCLUDED.matches_refreshed_at, summoner_profiles.matches_refreshed_at),
    updated_at = now();

-- name: UpsertSummonerCurrentRank :exec
INSERT INTO summoner_current_ranks (
    region, puuid, queue_type, tier, division,
    league_points, wins, losses, refreshed_at
) VALUES (
    @region, @puuid, @queue_type, @tier, @division,
    @league_points, @wins, @losses, @refreshed_at
)
ON CONFLICT (region, puuid, queue_type) DO UPDATE
SET tier = EXCLUDED.tier,
    division = EXCLUDED.division,
    league_points = EXCLUDED.league_points,
    wins = EXCLUDED.wins,
    losses = EXCLUDED.losses,
    refreshed_at = EXCLUDED.refreshed_at;

-- name: DeleteMissingSummonerCurrentRanks :exec
DELETE FROM summoner_current_ranks
WHERE region = @region
  AND puuid = @puuid
  AND NOT (queue_type = ANY(@queue_types::text[]));

-- name: DeleteAllSummonerCurrentRanks :exec
DELETE FROM summoner_current_ranks WHERE region = @region AND puuid = @puuid;

-- name: MarkSummonerLookupJobRunning :exec
UPDATE summoner_lookup_jobs
SET status = 'RUNNING', stage = @stage, puuid = COALESCE(@puuid, puuid),
    started_at = COALESCE(started_at, now()), updated_at = now()
WHERE id = @id;

-- name: UpdateSummonerLookupJobProgress :exec
UPDATE summoner_lookup_jobs
SET stage = @stage,
    scanned_count = @scanned_count,
    supported_count = @supported_count,
    fetched_count = @fetched_count,
    failed_count = @failed_count,
    updated_at = now()
WHERE id = @id;

-- name: FinishSummonerLookupJob :exec
UPDATE summoner_lookup_jobs
SET status = @status,
    stage = 'FINALIZE',
    error_code = @error_code,
    error_message = @error_message,
    completed_at = now(),
    updated_at = now()
WHERE id = @id;

-- name: UpsertSummonerLookupJobMatch :exec
INSERT INTO summoner_lookup_job_matches (job_id, match_id, ordinal, supported)
VALUES (@job_id, @match_id, @ordinal, @supported)
ON CONFLICT (job_id, match_id) DO UPDATE
SET ordinal = EXCLUDED.ordinal, supported = EXCLUDED.supported;

-- name: ListSummonerLookupRankTargets :many
SELECT DISTINCT COALESCE(mp.puuid, '') AS puuid
FROM summoner_lookup_job_matches jm
JOIN match_participants mp ON mp.match_id = jm.match_id
WHERE jm.job_id = @job_id
  AND jm.supported
  AND NULLIF(mp.puuid, '') IS NOT NULL
  AND mp.tier_at_match IS NULL
ORDER BY puuid;

-- name: BackfillSummonerLookupParticipantRank :exec
UPDATE match_participants mp
SET tier_at_match = @tier,
    division_at_match = NULLIF(@division::text, ''),
    lp_at_match = @league_points,
    tier_snapshot_delta_h = ROUND(
        ABS(EXTRACT(EPOCH FROM (now() - m.game_start_ts)) / 3600)
    )::int
FROM matches m, summoner_lookup_job_matches jm
WHERE jm.job_id = @job_id
  AND jm.supported
  AND jm.match_id = mp.match_id
  AND m.match_id = mp.match_id
  AND mp.puuid = @puuid
  AND mp.tier_at_match IS NULL;

-- name: MarkSummonerLookupParticipantUnranked :exec
UPDATE match_participants mp
SET tier_at_match = 'UNRANKED',
    division_at_match = NULL,
    lp_at_match = NULL,
    tier_snapshot_delta_h = ROUND(
        ABS(EXTRACT(EPOCH FROM (now() - m.game_start_ts)) / 3600)
    )::int
FROM matches m, summoner_lookup_job_matches jm
WHERE jm.job_id = @job_id
  AND jm.supported
  AND jm.match_id = mp.match_id
  AND m.match_id = mp.match_id
  AND mp.puuid = @puuid
  AND mp.tier_at_match IS NULL;

-- name: ListSummonerLookupMatchIDs :many
SELECT jm.match_id
FROM summoner_lookup_job_matches jm
JOIN matches m ON m.match_id = jm.match_id
WHERE jm.job_id = @job_id
  AND jm.supported
  AND m.fetch_status = 'done'
ORDER BY jm.ordinal;

-- name: GetSummonerLookupApexThresholds :one
SELECT COALESCE(MIN(CASE
                      WHEN UPPER(mp.tier_at_match) = 'CHALLENGER'
                      THEN COALESCE(mp.lp_at_match, 0)
                    END), -1)::int AS challenger_min_lp,
       COALESCE(MIN(CASE
                      WHEN UPPER(mp.tier_at_match) = 'GRANDMASTER'
                      THEN COALESCE(mp.lp_at_match, 0)
                    END), -1)::int AS grandmaster_min_lp
FROM summoner_lookup_job_matches jm
JOIN match_participants mp ON mp.match_id = jm.match_id
WHERE jm.job_id = @job_id
  AND jm.supported;

-- name: DeleteExpiredSummonerLookupJobs :execrows
DELETE FROM summoner_lookup_jobs
WHERE created_at < now() - interval '7 days'
  AND status IN ('COMPLETED', 'PARTIAL', 'FAILED');
