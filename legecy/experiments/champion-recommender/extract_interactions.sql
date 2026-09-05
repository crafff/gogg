SELECT
    m.match_id,
    m.game_start_ts,
    m.region,
    m.queue_id,
    split_part(m.game_version, '.', 1) || '.' || split_part(m.game_version, '.', 2) AS patch,
    p.puuid,
    p.champion_id,
    p.champion_name,
    UPPER(COALESCE(NULLIF(p.team_position, ''), p.individual_position)) AS position,
    p.tier_at_match,
    p.win,
    p.kills,
    p.deaths,
    p.assists,
    p.gold_earned,
    p.total_damage_dealt_to_champions,
    p.vision_score,
    p.time_played
FROM matches m
JOIN match_participants p ON p.match_id = m.match_id
WHERE p.puuid IS NOT NULL
  AND UPPER(COALESCE(NULLIF(p.team_position, ''), p.individual_position)) = %(position)s
  AND m.queue_id = %(queue_id)s
  AND m.fetch_status = 'done'
  AND m.game_start_ts >= %(min_game_start)s::timestamptz
  AND m.game_start_ts < %(max_game_start)s::timestamptz
ORDER BY m.game_start_ts, m.match_id, p.participant_id;
