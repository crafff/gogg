SELECT
    m.match_id, m.game_start_ts, m.region, m.queue_id,
    split_part(m.game_version, '.', 1) || '.' || split_part(m.game_version, '.', 2) AS patch,
    p.puuid, p.champion_id, p.champion_name,
    UPPER(COALESCE(NULLIF(p.team_position, ''), p.individual_position)) AS position,
    p.tier_at_match, p.win, p.kills, p.deaths, p.assists,
    p.gold_earned, p.total_damage_dealt_to_champions, p.vision_score, p.time_played,
    s10.jungle_cs AS jungle_cs_10, s15.jungle_cs - s10.jungle_cs AS jungle_cs_gain_10_15,
    s10.cs AS lane_cs_10, s15.cs - s10.cs AS lane_cs_gain_10_15,
    s10.dmg_to_champs AS damage_10, s15.dmg_to_champs - s10.dmg_to_champs AS damage_gain_10_15,
    s10.dmg_taken AS damage_taken_10, s15.dmg_taken - s10.dmg_taken AS damage_taken_gain_10_15,
    s10.time_enemy_cc AS cc_10, s15.time_enemy_cc - s10.time_enemy_cc AS cc_gain_10_15,
    -- wards_placed is exported for auditing but excluded from modelling: Riot's
    -- WARD_PLACED events include champion entities (notably Zyra plants).
    s10.wards_placed AS wards_10, s15.wards_placed - s10.wards_placed AS wards_gain_10_15,
    s10.wards_killed AS wards_killed_10, s15.wards_killed - s10.wards_killed AS wards_killed_gain_10_15
FROM matches m
JOIN match_participants p ON p.match_id = m.match_id
JOIN match_participant_snapshots s10
  ON s10.match_id = p.match_id AND s10.participant_id = p.participant_id AND s10.minute = 10
JOIN match_participant_snapshots s15
  ON s15.match_id = p.match_id AND s15.participant_id = p.participant_id AND s15.minute = 15
WHERE p.puuid IS NOT NULL
  AND UPPER(COALESCE(NULLIF(p.team_position, ''), p.individual_position)) = %(position)s
  AND m.queue_id = %(queue_id)s
  AND m.fetch_status = 'done' AND m.timeline_status = 'done'
  AND m.game_duration >= 900
  AND m.game_start_ts >= %(min_game_start)s::timestamptz
  AND m.game_start_ts < %(max_game_start)s::timestamptz
ORDER BY m.game_start_ts, m.match_id, p.participant_id;
