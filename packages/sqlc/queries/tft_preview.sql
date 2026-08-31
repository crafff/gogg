-- name: GetLatestCompletedTFTObservedRun :one
SELECT *
FROM tft_crawl_runs runs
WHERE runs.status IN ('completed', 'completed_with_errors')
  AND EXISTS (
      SELECT matches.match_id
      FROM tft_match_discoveries discoveries
      JOIN tft_matches matches ON matches.match_id = discoveries.match_id
      JOIN tft_match_participants participants ON participants.match_id = matches.match_id
      WHERE discoveries.run_id = runs.id
        AND (@platform::text = 'GLOBAL' OR matches.platform = @platform)
        AND matches.queue_id = 1100
        AND matches.participant_count = 8
        AND matches.set_number IS NOT NULL
        AND matches.patch = ''
        AND matches.exclusion_reason = 'invalid_game_version'
      GROUP BY matches.match_id
      HAVING COUNT(DISTINCT participants.puuid) = 8
         AND COUNT(DISTINCT participants.placement) = 8
         AND MIN(participants.placement) = 1
         AND MAX(participants.placement) = 8
  )
ORDER BY COALESCE(runs.ended_at, runs.updated_at) DESC, runs.id DESC
LIMIT 1;

-- name: GetTFTObservedLineupPreview :one
WITH run_match_ids AS (
    SELECT DISTINCT discoveries.match_id
    FROM tft_match_discoveries discoveries
    WHERE discoveries.run_id = @run_id
),
structurally_valid_matches AS (
    SELECT matches.match_id, matches.platform, matches.game_version,
           matches.set_number, matches.game_datetime
    FROM run_match_ids
    JOIN tft_matches matches USING (match_id)
    JOIN tft_match_participants participants USING (match_id)
    WHERE matches.queue_id = 1100
      AND matches.participant_count = 8
      AND matches.set_number IS NOT NULL
      AND matches.patch = ''
      AND matches.exclusion_reason = 'invalid_game_version'
    GROUP BY matches.match_id, matches.platform, matches.game_version,
             matches.set_number, matches.game_datetime
    HAVING COUNT(*) = 8
       AND COUNT(DISTINCT participants.placement) = 8
       AND MIN(participants.placement) = 1
       AND MAX(participants.placement) = 8
),
available_platforms AS (
    SELECT ARRAY_AGG(DISTINCT platform ORDER BY platform)::text[] AS platforms
    FROM structurally_valid_matches
),
target_set AS (
    SELECT matches.set_number
    FROM structurally_valid_matches matches
    WHERE @platform::text = 'GLOBAL' OR matches.platform = @platform
    GROUP BY matches.set_number
    ORDER BY COUNT(*) DESC, matches.set_number DESC
    LIMIT 1
),
selected_matches AS (
    SELECT matches.*
    FROM structurally_valid_matches matches
    JOIN target_set ON target_set.set_number = matches.set_number
    WHERE @platform::text = 'GLOBAL' OR matches.platform = @platform
),
source_stats AS (
    SELECT COUNT(*)::bigint AS source_matches,
           (COUNT(*) * 8)::bigint AS source_participants,
           MIN(game_datetime)::timestamptz AS window_start,
           MAX(game_datetime)::timestamptz AS window_end,
           ARRAY_AGG(DISTINCT game_version ORDER BY game_version)::text[] AS raw_game_versions
    FROM selected_matches
),
catalog_snapshot AS (
    SELECT id, patch, revision
    FROM tft_static_snapshots
    WHERE source = 'ddragon' AND locale = 'en_us' AND status = 'published'
    ORDER BY published_at DESC, id DESC
    LIMIT 1
),
purchasable_units AS (
    SELECT DISTINCT COALESCE(NULLIF(objects.payload ->> 'id', ''), objects.object_id) AS character_id
    FROM tft_static_objects objects
    JOIN catalog_snapshot snapshot ON snapshot.id = objects.snapshot_id
    WHERE objects.object_kind = 'unit'
      AND objects.purchasable IS TRUE
      AND objects.cost > 0
),
boards AS (
    SELECT participants.match_id, participants.puuid, participants.placement,
           ARRAY_AGG(DISTINCT units.character_id ORDER BY units.character_id)
               FILTER (WHERE purchasable_units.character_id IS NOT NULL)::text[] AS unit_ids
    FROM selected_matches matches
    JOIN tft_match_participants participants USING (match_id)
    JOIN tft_match_units units
      ON units.match_id = participants.match_id
     AND units.puuid = participants.puuid
    LEFT JOIN purchasable_units ON purchasable_units.character_id = units.character_id
    GROUP BY participants.match_id, participants.puuid, participants.placement
    HAVING COUNT(DISTINCT units.character_id)
               FILTER (WHERE purchasable_units.character_id IS NOT NULL) >= 5
),
observations AS (
    SELECT boards.*,
           ARRAY_TO_STRING(boards.unit_ids, '|') AS signature
    FROM boards
),
lobby_signature_counts AS (
    SELECT match_id, signature, COUNT(*)::bigint AS signature_count
    FROM observations
    GROUP BY match_id, signature
),
lineup_rollups AS (
    SELECT observations.signature, observations.unit_ids,
           COUNT(*)::bigint AS sample_size,
           COUNT(DISTINCT observations.match_id)::bigint AS lobby_count,
           AVG(observations.placement)::double precision AS avg_placement,
           (COUNT(*) FILTER (WHERE observations.placement = 1))::double precision
               / COUNT(*) AS first_rate,
           (COUNT(*) FILTER (WHERE observations.placement <= 4))::double precision
               / COUNT(*) AS top4_rate,
           (COUNT(*) FILTER (WHERE lobby_signature_counts.signature_count > 1))::double precision
               / COUNT(*) AS contested_rate
    FROM observations
    JOIN lobby_signature_counts USING (match_id, signature)
    GROUP BY observations.signature, observations.unit_ids
    HAVING COUNT(*) >= 20
),
preview_lineups AS (
    SELECT lineup_rollups.*,
           lineup_rollups.sample_size::double precision
               / NULLIF((SELECT COUNT(*) FROM observations), 0) AS pick_rate
    FROM lineup_rollups
    ORDER BY lineup_rollups.avg_placement ASC,
             lineup_rollups.top4_rate DESC,
             lineup_rollups.sample_size DESC,
             lineup_rollups.signature ASC
)
SELECT @run_id::bigint AS run_id,
       @platform::text AS platform,
       available_platforms.platforms,
       1100::int AS queue_id,
       target_set.set_number,
       source_stats.raw_game_versions,
       source_stats.source_matches,
       source_stats.source_participants,
       (SELECT COUNT(*) FROM observations)::bigint AS usable_participants,
       source_stats.window_start,
       source_stats.window_end,
       (SELECT COUNT(DISTINCT signature) FROM observations)::bigint AS exact_lineups,
       catalog_snapshot.id AS catalog_snapshot_id,
       catalog_snapshot.patch AS catalog_patch,
       catalog_snapshot.revision AS catalog_revision,
       COALESCE(
           (
               SELECT JSONB_AGG(
                   JSONB_BUILD_OBJECT(
                       'id', preview_lineups.signature,
                       'unit_ids', preview_lineups.unit_ids,
                       'sample_size', preview_lineups.sample_size,
                       'lobby_count', preview_lineups.lobby_count,
                       'pick_rate', preview_lineups.pick_rate,
                       'avg_placement', preview_lineups.avg_placement,
                       'first_rate', preview_lineups.first_rate,
                       'top4_rate', preview_lineups.top4_rate,
                       'contested_rate', preview_lineups.contested_rate
                   )
                   ORDER BY preview_lineups.avg_placement ASC,
                            preview_lineups.top4_rate DESC,
                            preview_lineups.sample_size DESC,
                            preview_lineups.signature ASC
               )
               FROM preview_lineups
           ),
           '[]'::jsonb
       ) AS lineups
FROM available_platforms
CROSS JOIN target_set
CROSS JOIN source_stats
CROSS JOIN catalog_snapshot;

-- name: GetTFTObservedLineupDetails :one
WITH run_match_ids AS (
    SELECT DISTINCT discoveries.match_id
    FROM tft_match_discoveries discoveries
    WHERE discoveries.run_id = @run_id
),
structurally_valid_matches AS (
    SELECT matches.match_id, matches.platform, matches.set_number
    FROM run_match_ids
    JOIN tft_matches matches USING (match_id)
    JOIN tft_match_participants participants USING (match_id)
    WHERE matches.queue_id = 1100
      AND matches.participant_count = 8
      AND matches.set_number IS NOT NULL
      AND matches.patch = ''
      AND matches.exclusion_reason = 'invalid_game_version'
    GROUP BY matches.match_id, matches.platform, matches.set_number
    HAVING COUNT(*) = 8
       AND COUNT(DISTINCT participants.placement) = 8
       AND MIN(participants.placement) = 1
       AND MAX(participants.placement) = 8
),
target_set AS (
    SELECT matches.set_number
    FROM structurally_valid_matches matches
    WHERE @platform::text = 'GLOBAL' OR matches.platform = @platform
    GROUP BY matches.set_number
    ORDER BY COUNT(*) DESC, matches.set_number DESC
    LIMIT 1
),
selected_matches AS (
    SELECT matches.match_id
    FROM structurally_valid_matches matches
    JOIN target_set ON target_set.set_number = matches.set_number
    WHERE @platform::text = 'GLOBAL' OR matches.platform = @platform
),
catalog_snapshot AS (
    SELECT id
    FROM tft_static_snapshots
    WHERE id = @catalog_snapshot_id
      AND source = 'ddragon'
      AND locale = 'en_us'
      AND status = 'published'
),
purchasable_units AS (
    SELECT DISTINCT COALESCE(NULLIF(objects.payload ->> 'id', ''), objects.object_id) AS character_id
    FROM tft_static_objects objects
    JOIN catalog_snapshot snapshot ON snapshot.id = objects.snapshot_id
    WHERE objects.object_kind = 'unit'
      AND objects.purchasable IS TRUE
      AND objects.cost > 0
),
unit_item_totals AS (
    SELECT items.match_id, items.puuid, items.unit_index,
           COUNT(*)::int AS item_count
    FROM selected_matches matches
    JOIN tft_match_unit_items items USING (match_id)
    GROUP BY items.match_id, items.puuid, items.unit_index
),
board_units AS (
    SELECT participants.match_id, participants.puuid, participants.placement,
           units.character_id,
           MAX(NULLIF(units.tier, 0))::int AS star_level,
           (ARRAY_AGG(
               units.unit_index
               ORDER BY COALESCE(unit_item_totals.item_count, 0) DESC,
                        units.tier DESC NULLS LAST,
                        units.unit_index
           ))[1] AS equipped_unit_index
    FROM selected_matches matches
    JOIN tft_match_participants participants USING (match_id)
    JOIN tft_match_units units
      ON units.match_id = participants.match_id
     AND units.puuid = participants.puuid
    JOIN purchasable_units ON purchasable_units.character_id = units.character_id
    LEFT JOIN unit_item_totals
      ON unit_item_totals.match_id = units.match_id
     AND unit_item_totals.puuid = units.puuid
     AND unit_item_totals.unit_index = units.unit_index
    GROUP BY participants.match_id, participants.puuid, participants.placement,
             units.character_id
),
board_unit_observations AS (
    SELECT board_units.*,
           ARRAY_AGG(character_id) OVER (
               PARTITION BY match_id, puuid
               ORDER BY character_id
               ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING
           )::text[] AS unit_ids,
           CASE
               WHEN COUNT(*) FILTER (WHERE star_level > 0)
                        OVER (PARTITION BY match_id, puuid)
                    = COUNT(*) OVER (PARTITION BY match_id, puuid)
               THEN SUM(star_level) OVER (PARTITION BY match_id, puuid)::int
               ELSE NULL
           END AS total_star_level,
           COUNT(*) OVER (PARTITION BY match_id, puuid) AS unit_count
    FROM board_units
),
target_board_units AS (
    SELECT observations.*,
           ARRAY_TO_STRING(observations.unit_ids, '|') AS signature,
           COALESCE(items.item_count, 0)::int AS item_count
    FROM board_unit_observations observations
    LEFT JOIN unit_item_totals items
      ON items.match_id = observations.match_id
     AND items.puuid = observations.puuid
     AND items.unit_index = observations.equipped_unit_index
    WHERE observations.unit_count >= 5
      AND ARRAY_TO_STRING(observations.unit_ids, '|') = ANY(@signatures::text[])
),
target_observations AS (
    SELECT match_id, puuid, placement, signature, total_star_level,
           CASE
               WHEN COUNT(*) FILTER (WHERE star_level > 0) = COUNT(*)
               THEN ARRAY_AGG(star_level ORDER BY star_level)::int[]
               ELSE NULL
           END AS star_levels
    FROM target_board_units
    GROUP BY match_id, puuid, placement, signature, total_star_level
),
signature_stats AS (
    SELECT signature, COUNT(*)::bigint AS sample_size
    FROM target_observations
    GROUP BY signature
),
star_level_rollups AS (
    SELECT observations.signature, observations.total_star_level,
           COUNT(*)::bigint AS sample_size,
           COUNT(DISTINCT observations.match_id)::bigint AS lobby_count,
           COUNT(*)::double precision / MIN(stats.sample_size) AS rate,
           AVG(observations.placement)::double precision AS avg_placement,
           (COUNT(*) FILTER (WHERE observations.placement = 1))::double precision
               / COUNT(*) AS first_rate,
           (COUNT(*) FILTER (WHERE observations.placement <= 4))::double precision
               / COUNT(*) AS top4_rate
    FROM target_observations observations
    JOIN signature_stats stats USING (signature)
    WHERE observations.total_star_level IS NOT NULL
    GROUP BY observations.signature, observations.total_star_level
    HAVING COUNT(*) >= 5
),
lineup_star_levels AS (
    SELECT signature,
           JSONB_AGG(
               JSONB_BUILD_OBJECT(
                   'total_stars', total_star_level,
                   'sample_size', sample_size,
                   'lobby_count', lobby_count,
                   'rate', rate,
                   'avg_placement', avg_placement,
                   'first_rate', first_rate,
                   'top4_rate', top4_rate
               )
               ORDER BY total_star_level
           ) AS star_levels
    FROM star_level_rollups
    GROUP BY signature
),
unit_observation_rollups AS (
    SELECT units.signature, units.character_id,
           COUNT(*)::bigint AS sample_size,
           (COUNT(*) FILTER (WHERE units.star_level > 0))::bigint
               AS known_star_samples,
           (COUNT(*) FILTER (WHERE units.star_level IS NULL
                                  OR units.star_level <= 0))::bigint
               AS unknown_star_samples,
           (COUNT(*) FILTER (WHERE units.star_level > 0))::double precision
               / COUNT(*) AS star_coverage,
           AVG(LEAST(units.item_count, 3))::double precision AS average_items,
           AVG(LEAST(units.item_count, 3)::double precision / 3) AS item_investment_rate,
           (COUNT(*) FILTER (WHERE units.item_count > 0))::double precision
               / COUNT(*) AS equipped_rate,
           (COUNT(*) FILTER (WHERE units.item_count >= 3))::double precision
               / COUNT(*) AS three_item_rate
    FROM target_board_units units
    GROUP BY units.signature, units.character_id
),
ranked_unit_investment AS (
    SELECT rollups.*,
           DENSE_RANK() OVER (
               PARTITION BY rollups.signature
               ORDER BY rollups.item_investment_rate DESC,
                        rollups.three_item_rate DESC,
                        rollups.average_items DESC
           )::int AS investment_rank,
           MAX(rollups.item_investment_rate) OVER (
               PARTITION BY rollups.signature
           ) AS maximum_investment_rate
    FROM unit_observation_rollups rollups
),
unit_summaries AS (
    SELECT investment.*,
           CASE
               WHEN investment.item_investment_rate >= 0.60
                AND investment.item_investment_rate
                    >= investment.maximum_investment_rate * 0.75
               THEN investment.investment_rank
               ELSE NULL
           END AS core_rank
    FROM ranked_unit_investment investment
),
unit_star_rollups AS (
    SELECT units.signature, units.character_id, units.star_level,
           COUNT(*)::bigint AS sample_size,
           COUNT(*)::double precision / MIN(summaries.sample_size) AS rate,
           COUNT(*)::double precision / NULLIF(MIN(summaries.known_star_samples), 0)
               AS known_rate,
           CASE WHEN COUNT(*) >= 5
                THEN AVG(units.placement)::double precision END AS avg_placement,
           CASE WHEN COUNT(*) >= 5
                THEN (COUNT(*) FILTER (WHERE units.placement = 1))::double precision
                     / COUNT(*) END AS first_rate,
           CASE WHEN COUNT(*) >= 5
                THEN (COUNT(*) FILTER (WHERE units.placement <= 4))::double precision
                     / COUNT(*) END AS top4_rate
    FROM target_board_units units
    JOIN unit_summaries summaries
      ON summaries.signature = units.signature
     AND summaries.character_id = units.character_id
    WHERE units.star_level > 0
    GROUP BY units.signature, units.character_id, units.star_level
),
unit_star_lists AS (
    SELECT signature, character_id,
           JSONB_AGG(
               JSONB_BUILD_OBJECT(
                   'stars', star_level,
                   'sample_size', sample_size,
                   'rate', rate,
                   'known_rate', known_rate,
                   'avg_placement', avg_placement,
                   'first_rate', first_rate,
                   'top4_rate', top4_rate
               )
               ORDER BY star_level
           ) AS star_distribution
    FROM unit_star_rollups
    GROUP BY signature, character_id
),
star_composition_rollups AS (
    SELECT observations.signature, observations.star_levels,
           observations.total_star_level,
           COUNT(*)::bigint AS sample_size,
           COUNT(*)::double precision / MIN(stats.sample_size) AS rate,
           CASE WHEN COUNT(*) >= 5
                THEN AVG(observations.placement)::double precision END AS avg_placement,
           CASE WHEN COUNT(*) >= 5
                THEN (COUNT(*) FILTER (WHERE observations.placement = 1))::double precision
                     / COUNT(*) END AS first_rate,
           CASE WHEN COUNT(*) >= 5
                THEN (COUNT(*) FILTER (WHERE observations.placement <= 4))::double precision
                     / COUNT(*) END AS top4_rate
    FROM target_observations observations
    JOIN signature_stats stats USING (signature)
    WHERE observations.star_levels IS NOT NULL
    GROUP BY observations.signature, observations.star_levels,
             observations.total_star_level
),
lineup_star_compositions AS (
    SELECT signature,
           SUM(sample_size)::bigint AS known_samples,
           JSONB_AGG(
               JSONB_BUILD_OBJECT(
                   'star_levels', star_levels,
                   'total_stars', total_star_level,
                   'sample_size', sample_size,
                   'rate', rate,
                   'avg_placement', avg_placement,
                   'first_rate', first_rate,
                   'top4_rate', top4_rate
               )
               ORDER BY sample_size DESC, star_levels DESC
           ) AS star_compositions
    FROM star_composition_rollups
    GROUP BY signature
),
unit_item_counts AS (
    SELECT units.signature, units.character_id, items.item_id,
           COUNT(DISTINCT (units.match_id, units.puuid))::bigint AS use_count
    FROM target_board_units units
    JOIN tft_match_unit_items items
      ON items.match_id = units.match_id
     AND items.puuid = units.puuid
     AND items.unit_index = units.equipped_unit_index
    GROUP BY units.signature, units.character_id, items.item_id
),
ranked_unit_items AS (
    SELECT counts.signature, counts.character_id, counts.item_id, counts.use_count,
           counts.use_count::double precision / stats.sample_size AS rate,
           ROW_NUMBER() OVER (
               PARTITION BY counts.signature, counts.character_id
               ORDER BY counts.use_count DESC, counts.item_id
           ) AS item_rank
    FROM unit_item_counts counts
    JOIN signature_stats stats USING (signature)
    WHERE counts.use_count >= GREATEST(
        3::numeric,
        CEIL(stats.sample_size::numeric * 0.05)
    )
),
unit_item_lists AS (
    SELECT signature, character_id,
           JSONB_AGG(
               JSONB_BUILD_OBJECT(
                   'id', item_id,
                   'count', use_count,
                   'rate', rate
               )
               ORDER BY use_count DESC, item_id
           ) AS items
    FROM ranked_unit_items
    WHERE item_rank <= 3
    GROUP BY signature, character_id
),
lineup_unit_items AS (
    SELECT summaries.signature,
           JSONB_AGG(
               JSONB_BUILD_OBJECT(
                   'unit_id', summaries.character_id,
                   'items', COALESCE(item_lists.items, '[]'::jsonb),
                   'core_rank', summaries.core_rank,
                   'average_items', summaries.average_items,
                   'item_investment_rate', summaries.item_investment_rate,
                   'equipped_rate', summaries.equipped_rate,
                   'three_item_rate', summaries.three_item_rate,
                   'known_star_samples', summaries.known_star_samples,
                   'unknown_star_samples', summaries.unknown_star_samples,
                   'star_coverage', summaries.star_coverage,
                   'star_distribution', COALESCE(star_lists.star_distribution, '[]'::jsonb)
               )
               ORDER BY summaries.character_id
           ) AS unit_items
    FROM unit_summaries summaries
    LEFT JOIN unit_item_lists item_lists
      ON item_lists.signature = summaries.signature
     AND item_lists.character_id = summaries.character_id
    LEFT JOIN unit_star_lists star_lists
      ON star_lists.signature = summaries.signature
     AND star_lists.character_id = summaries.character_id
    GROUP BY summaries.signature
),
requested_signatures AS (
    SELECT signature, ordinality
    FROM UNNEST(@signatures::text[]) WITH ORDINALITY AS requested(signature, ordinality)
)
SELECT COALESCE(
    JSONB_AGG(
        JSONB_BUILD_OBJECT(
            'id', requested_signatures.signature,
            'unit_items', COALESCE(lineup_unit_items.unit_items, '[]'::jsonb),
            'star_levels', COALESCE(lineup_star_levels.star_levels, '[]'::jsonb),
            'star_composition_known_samples', COALESCE(lineup_star_compositions.known_samples, 0),
            'star_compositions', COALESCE(lineup_star_compositions.star_compositions, '[]'::jsonb)
        )
        ORDER BY requested_signatures.ordinality
    ),
    '[]'::jsonb
) AS details
FROM requested_signatures
LEFT JOIN lineup_unit_items USING (signature)
LEFT JOIN lineup_star_levels USING (signature)
LEFT JOIN lineup_star_compositions USING (signature);

-- name: ListLatestTFTLocalizedStaticObjects :many
WITH snapshot AS (
    SELECT id, patch, revision
    FROM tft_static_snapshots
    WHERE source = 'cdragon' AND locale = @locale AND status = 'published'
    ORDER BY published_at DESC, id DESC
    LIMIT 1
)
SELECT objects.object_kind, objects.object_id, objects.name, objects.payload,
       snapshot.patch, snapshot.revision
FROM tft_static_objects objects
JOIN snapshot ON snapshot.id = objects.snapshot_id
WHERE objects.object_kind = ANY(@object_kinds::text[])
ORDER BY objects.object_kind, objects.object_id;
