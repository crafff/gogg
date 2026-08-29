-- One deterministically selected jungle participant per ranked match.
-- Selecting one side prevents paired opponents from crossing data splits.
WITH ranked_candidates AS (
    SELECT
        m.match_id, m.region, m.game_version, m.game_start_ts,
        r.puuid, r.participant_id, r.team_id, r.win,
        r.champion_id, r.champion_name, r.tier_at_match,
        ROW_NUMBER() OVER (PARTITION BY m.match_id ORDER BY r.participant_id, r.id) AS jungle_rank
    FROM matches m
    JOIN match_participants r ON r.match_id = m.match_id
    WHERE UPPER(COALESCE(NULLIF(r.team_position, ''), r.individual_position)) = 'JUNGLE'
      AND r.team_id = CASE WHEN MOD(ABS(hashtext(m.match_id)), 2) = 0 THEN 100 ELSE 200 END
      AND m.queue_id = %(queue_id)s
      AND m.fetch_status = 'done'
      AND m.timeline_status = 'done'
      AND m.game_duration >= 900
      AND m.game_start_ts >= %(min_game_start)s::timestamptz
      AND m.game_start_ts < %(max_game_start)s::timestamptz
), candidates AS (
    SELECT * FROM ranked_candidates WHERE jungle_rank = 1
)
SELECT
    c.*,
    s10.total_gold AS gold_10, s15.total_gold AS gold_15,
    s10.current_gold AS current_gold_10, s15.current_gold AS current_gold_15,
    s10.jungle_cs AS jungle_cs_10, s15.jungle_cs AS jungle_cs_15,
    s10.cs AS lane_cs_10, s15.cs AS lane_cs_15,
    s10.level AS level_10, s15.level AS level_15,
    s10.xp AS xp_10, s15.xp AS xp_15,
    s10.time_enemy_cc AS time_enemy_cc_10, s15.time_enemy_cc AS time_enemy_cc_15,
    s10.wards_placed AS wards_placed_10, s15.wards_placed AS wards_placed_15,
    s10.wards_killed AS wards_killed_10, s15.wards_killed AS wards_killed_15,
    s10.dmg_to_champs AS dmg_to_champs_10, s15.dmg_to_champs AS dmg_to_champs_15,
    s10.dmg_taken AS dmg_taken_10, s15.dmg_taken AS dmg_taken_15
FROM candidates c
JOIN match_participant_snapshots s10
  ON s10.match_id = c.match_id AND s10.participant_id = c.participant_id AND s10.minute = 10
JOIN match_participant_snapshots s15
  ON s15.match_id = c.match_id AND s15.participant_id = c.participant_id AND s15.minute = 15
ORDER BY c.game_start_ts, c.match_id;
