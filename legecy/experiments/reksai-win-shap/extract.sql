-- One row per Rek'Sai jungle game. psql defaults can be overridden with -v.
\if :{?queue_id}
\else
\set queue_id 420
\endif
\if :{?cutoff_minute}
\else
\set cutoff_minute 15
\endif
\if :{?min_game_start}
\else
\set min_game_start '1970-01-01'
\endif
\if :{?max_game_start}
\else
\set max_game_start '2100-01-01'
\endif

WITH candidates AS (
    SELECT
        m.match_id, m.region, m.game_version, m.game_start_ts,
        r.puuid, r.participant_id, r.team_id, r.win,
        e.participant_id AS enemy_participant_id
    FROM matches m
    JOIN match_participants r ON r.match_id = m.match_id
    JOIN match_participants e
      ON e.match_id = r.match_id
     AND e.team_id <> r.team_id
     AND UPPER(COALESCE(NULLIF(e.team_position, ''), e.individual_position)) = 'JUNGLE'
    WHERE r.champion_id = 421
      AND UPPER(COALESCE(NULLIF(r.team_position, ''), r.individual_position)) = 'JUNGLE'
      AND m.queue_id = :queue_id
      AND m.fetch_status = 'done'
      AND m.timeline_status = 'done'
      AND m.game_duration >= (:cutoff_minute * 60)
      AND m.game_start_ts >= :'min_game_start'::timestamptz
      AND m.game_start_ts < :'max_game_start'::timestamptz
), item_features AS (
    SELECT
        c.match_id,
        COUNT(*) FILTER (WHERE i.removal_type IS NULL) AS item_purchases_15,
        COUNT(DISTINCT i.item_id) FILTER (WHERE i.removal_type IS NULL) AS distinct_items_15,
        COUNT(*) FILTER (WHERE i.removal_type = 'undo') AS item_undos_15,
        COUNT(*) FILTER (WHERE i.removal_type = 'sold') AS items_sold_15,
        MIN(i.timestamp_ms) FILTER (WHERE i.removal_type IS NULL) AS first_purchase_ms,
        MAX(i.timestamp_ms) FILTER (WHERE i.removal_type IS NULL) AS last_purchase_ms
    FROM candidates c
    LEFT JOIN match_item_events i
      ON i.match_id = c.match_id
     AND i.participant_id = c.participant_id
     AND i.timestamp_ms <= (:cutoff_minute * 60000)
    GROUP BY c.match_id
), skill_features AS (
    SELECT
        c.match_id,
        COUNT(*) FILTER (WHERE s.skill_slot = 1) AS q_points_15,
        COUNT(*) FILTER (WHERE s.skill_slot = 2) AS w_points_15,
        COUNT(*) FILTER (WHERE s.skill_slot = 3) AS e_points_15,
        COUNT(*) FILTER (WHERE s.skill_slot = 4) AS r_points_15
    FROM candidates c
    LEFT JOIN match_skill_events s
      ON s.match_id = c.match_id
     AND s.participant_id = c.participant_id
     AND s.timestamp_ms <= (:cutoff_minute * 60000)
    GROUP BY c.match_id
), snapshots AS (
    SELECT
        c.*,
        r10.total_gold AS gold_10, r15.total_gold AS gold_15,
        r10.current_gold AS current_gold_10, r15.current_gold AS current_gold_15,
        r10.jungle_cs AS jungle_cs_10, r15.jungle_cs AS jungle_cs_15,
        r10.cs AS lane_cs_10, r15.cs AS lane_cs_15,
        r10.level AS level_10, r15.level AS level_15,
        r10.xp AS xp_10, r15.xp AS xp_15,
        r10.time_enemy_cc AS time_enemy_cc_10, r15.time_enemy_cc AS time_enemy_cc_15,
        r10.wards_placed AS wards_placed_10, r15.wards_placed AS wards_placed_15,
        r10.wards_killed AS wards_killed_10, r15.wards_killed AS wards_killed_15,
        r10.dmg_total AS dmg_total_10, r15.dmg_total AS dmg_total_15,
        r10.dmg_to_champs AS dmg_to_champs_10, r15.dmg_to_champs AS dmg_to_champs_15,
        r10.dmg_magic_champs AS dmg_magic_champs_10, r15.dmg_magic_champs AS dmg_magic_champs_15,
        r10.dmg_phys_champs AS dmg_phys_champs_10, r15.dmg_phys_champs AS dmg_phys_champs_15,
        r10.dmg_true_champs AS dmg_true_champs_10, r15.dmg_true_champs AS dmg_true_champs_15,
        r10.dmg_taken AS dmg_taken_10, r15.dmg_taken AS dmg_taken_15,
        r10.pos_x AS pos_x_10, r10.pos_y AS pos_y_10,
        r15.pos_x AS pos_x_15, r15.pos_y AS pos_y_15,
        e15.total_gold AS enemy_gold_15,
        e15.current_gold AS enemy_current_gold_15,
        e15.jungle_cs AS enemy_jungle_cs_15,
        e15.cs AS enemy_lane_cs_15,
        e15.level AS enemy_level_15,
        e15.xp AS enemy_xp_15,
        e15.time_enemy_cc AS enemy_time_enemy_cc_15,
        e15.wards_placed AS enemy_wards_placed_15,
        e15.wards_killed AS enemy_wards_killed_15,
        e15.dmg_total AS enemy_dmg_total_15,
        e15.dmg_to_champs AS enemy_dmg_to_champs_15,
        e15.dmg_taken AS enemy_dmg_taken_15,
        COALESCE(i.item_purchases_15, 0) AS item_purchases_15,
        COALESCE(i.distinct_items_15, 0) AS distinct_items_15,
        COALESCE(i.item_undos_15, 0) AS item_undos_15,
        COALESCE(i.items_sold_15, 0) AS items_sold_15,
        COALESCE(i.first_purchase_ms, 0) AS first_purchase_ms,
        COALESCE(i.last_purchase_ms, 0) AS last_purchase_ms,
        COALESCE(sk.q_points_15, 0) AS q_points_15,
        COALESCE(sk.w_points_15, 0) AS w_points_15,
        COALESCE(sk.e_points_15, 0) AS e_points_15,
        COALESCE(sk.r_points_15, 0) AS r_points_15
    FROM candidates c
    JOIN match_participant_snapshots r10
      ON r10.match_id = c.match_id AND r10.participant_id = c.participant_id AND r10.minute = 10
    JOIN match_participant_snapshots r15
      ON r15.match_id = c.match_id AND r15.participant_id = c.participant_id AND r15.minute = :cutoff_minute
    JOIN match_participant_snapshots e15
      ON e15.match_id = c.match_id AND e15.participant_id = c.enemy_participant_id AND e15.minute = :cutoff_minute
    LEFT JOIN item_features i ON i.match_id = c.match_id
    LEFT JOIN skill_features sk ON sk.match_id = c.match_id
)
SELECT * FROM snapshots ORDER BY game_start_ts, match_id;
