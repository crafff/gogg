-- name: CreateTFTLineupPublication :one
INSERT INTO tft_lineup_publications (
    platform, patch, set_number, queue_id, cohort, window_kind,
    algorithm_version, status, source_matches, source_participants, coverage
) VALUES (
    @platform, @patch, @set_number, @queue_id, @cohort, @window_kind,
    @algorithm_version, 'building', @source_matches, @source_participants, @coverage
)
RETURNING *;

-- name: DeleteTFTLineupPublicationFamilies :exec
DELETE FROM tft_lineup_families WHERE publication_id = $1;

-- name: DeleteTFTExactRollupSlice :exec
DELETE FROM tft_lineup_exact_rollups
WHERE platform = @platform AND patch = @patch AND set_number = @set_number
  AND queue_id = @queue_id AND cohort = @cohort AND window_kind = @window_kind;

-- name: UpsertTFTExactRollup :exec
INSERT INTO tft_lineup_exact_rollups (
    platform, patch, set_number, queue_id, cohort, window_kind, signature,
    sample_size, lobby_count, placement_sum, first_count, top4_count,
    contested_count, unit_ids, item_counts, augment_counts, trait_counts
) VALUES (
    @platform, @patch, @set_number, @queue_id, @cohort, @window_kind, @signature,
    @sample_size, @lobby_count, @placement_sum, @first_count, @top4_count,
    @contested_count, @unit_ids, @item_counts, @augment_counts, @trait_counts
)
ON CONFLICT (platform, patch, set_number, queue_id, cohort, window_kind, signature) DO UPDATE
SET sample_size = EXCLUDED.sample_size, lobby_count = EXCLUDED.lobby_count,
    placement_sum = EXCLUDED.placement_sum, first_count = EXCLUDED.first_count,
    top4_count = EXCLUDED.top4_count, contested_count = EXCLUDED.contested_count,
    unit_ids = EXCLUDED.unit_ids, item_counts = EXCLUDED.item_counts,
    augment_counts = EXCLUDED.augment_counts, trait_counts = EXCLUDED.trait_counts,
    refreshed_at = now();

-- name: ListTFTLineupObservations :many
WITH cohort_matches AS (
    SELECT DISTINCT discoveries.match_id
    FROM tft_match_discoveries discoveries
    JOIN tft_matches matches ON matches.match_id = discoveries.match_id
    WHERE discoveries.platform = @platform
      AND discoveries.cohort = @cohort
      AND matches.patch = @patch
      AND matches.set_number = @set_number
      AND matches.queue_id = @queue_id
      AND matches.game_datetime >= @window_start
      AND matches.game_datetime < @window_end
      AND matches.eligible
)
SELECT participants.match_id,
       participants.puuid,
       participants.placement,
       participants.lineup_signature,
       string_to_array(participants.lineup_signature, '|')::text[] AS unit_ids,
       ARRAY(SELECT items.item_id FROM tft_match_unit_items items
             WHERE items.match_id = participants.match_id AND items.puuid = participants.puuid
             ORDER BY items.unit_index, items.item_slot)::text[] AS item_ids,
       ARRAY(SELECT augments.augment_id FROM tft_match_augments augments
             WHERE augments.match_id = participants.match_id AND augments.puuid = participants.puuid
             ORDER BY augments.slot)::text[] AS augment_ids,
       ARRAY(SELECT traits.trait_id FROM tft_match_traits traits
             WHERE traits.match_id = participants.match_id AND traits.puuid = participants.puuid
               AND COALESCE(traits.tier_current, 0) > 0
             ORDER BY traits.trait_index)::text[] AS trait_ids
FROM cohort_matches
JOIN tft_match_participants participants USING (match_id)
WHERE NOT participants.abnormal
  AND participants.mapped_unit_count >= 5
  AND participants.lineup_signature IS NOT NULL
ORDER BY participants.match_id, participants.puuid;

-- name: ListTFTAnalysisTargets :many
WITH patch_activity AS (
    SELECT platform, patch, set_number, queue_id,
           MIN(game_datetime)::timestamptz AS first_match_at,
           MAX(game_datetime)::timestamptz AS last_match_at,
           COUNT(*)::bigint AS match_count
    FROM tft_matches
    WHERE eligible AND queue_id = 1100 AND game_datetime >= @not_before
      AND set_number IS NOT NULL
    GROUP BY platform, patch, set_number, queue_id
),
ranked AS (
    SELECT *, ROW_NUMBER() OVER (PARTITION BY platform ORDER BY last_match_at DESC, patch DESC) AS freshness_rank
    FROM patch_activity
)
SELECT platform, patch, set_number, queue_id, first_match_at, last_match_at, match_count
FROM ranked
WHERE freshness_rank = 1
ORDER BY platform;

-- name: CountTFTEligibleObservations :one
SELECT COUNT(*)::bigint
FROM tft_matches matches
JOIN tft_match_participants participants USING (match_id)
WHERE matches.platform = @platform AND matches.patch = @patch
  AND matches.queue_id = 1100 AND matches.eligible
  AND NOT participants.abnormal AND participants.mapped_unit_count >= 5;

-- name: InsertTFTLineupFamily :exec
INSERT INTO tft_lineup_families (
    publication_id, family_id, display_name, sample_size, lobby_count,
    pick_rate, avg_placement, first_rate, top4_rate, contested_rate,
    core_units, common_items, common_augments, common_traits
) VALUES (
    @publication_id, @family_id, sqlc.narg(display_name), @sample_size, @lobby_count,
    @pick_rate, @avg_placement, @first_rate, @top4_rate, @contested_rate,
    @core_units, @common_items, @common_augments, @common_traits
);

-- name: InsertTFTLineupFamilyMember :exec
INSERT INTO tft_lineup_family_members (
    publication_id, family_id, signature, sample_size, jaccard
) VALUES (@publication_id, @family_id, @signature, @sample_size, @jaccard);

-- name: PublishTFTLineupPublication :exec
WITH superseded AS (
    UPDATE tft_lineup_publications previous
    SET status = 'superseded'
    WHERE previous.platform = (SELECT target.platform FROM tft_lineup_publications target WHERE target.id = sqlc.arg(publication_id))
      AND previous.patch = (SELECT target.patch FROM tft_lineup_publications target WHERE target.id = sqlc.arg(publication_id))
      AND previous.set_number = (SELECT target.set_number FROM tft_lineup_publications target WHERE target.id = sqlc.arg(publication_id))
      AND previous.queue_id = (SELECT target.queue_id FROM tft_lineup_publications target WHERE target.id = sqlc.arg(publication_id))
      AND previous.cohort = (SELECT target.cohort FROM tft_lineup_publications target WHERE target.id = sqlc.arg(publication_id))
      AND previous.window_kind = (SELECT target.window_kind FROM tft_lineup_publications target WHERE target.id = sqlc.arg(publication_id))
      AND previous.status = 'published'
      AND previous.id <> sqlc.arg(publication_id)
)
UPDATE tft_lineup_publications target
SET status = 'published', published_at = now()
WHERE target.id = sqlc.arg(publication_id);

-- name: ListTFTAnalysisCatalog :many
SELECT platform, patch, set_number, queue_id, cohort, window_kind,
       published_at, source_matches, source_participants, coverage
FROM tft_lineup_publications
WHERE status = 'published'
ORDER BY published_at DESC, platform, cohort, window_kind;

-- name: ListTFTLocalizedStaticObjects :many
WITH snapshot AS (
    SELECT snapshots.id, snapshots.patch, snapshots.revision
    FROM tft_static_snapshots snapshots
    WHERE snapshots.source = 'cdragon'
      AND snapshots.patch = sqlc.arg(patch)
      AND snapshots.locale = @locale
      AND snapshots.status = 'published'
    ORDER BY snapshots.published_at DESC, snapshots.id DESC
    LIMIT 1
), catalog_snapshot AS (
    SELECT snapshots.id
    FROM tft_static_snapshots snapshots
    WHERE snapshots.source = 'ddragon'
      AND snapshots.patch = sqlc.arg(patch)
      AND snapshots.locale = @locale
      AND snapshots.status = 'published'
    ORDER BY snapshots.published_at DESC, snapshots.id DESC
    LIMIT 1
)
SELECT objects.object_kind, objects.object_id, objects.name,
       COALESCE(NULLIF(objects.cost, 0), catalog_unit.cost) AS cost,
       objects.payload,
       snapshot.patch, snapshot.revision
FROM tft_static_objects objects
JOIN snapshot ON snapshot.id = objects.snapshot_id
LEFT JOIN LATERAL (
    SELECT catalog.cost
    FROM tft_static_objects catalog
    JOIN catalog_snapshot ON catalog_snapshot.id = catalog.snapshot_id
    WHERE catalog.object_kind = 'unit'
      AND COALESCE(NULLIF(catalog.payload ->> 'id', ''), catalog.object_id) = objects.object_id
      AND catalog.cost > 0
    ORDER BY catalog.cost
    LIMIT 1
) catalog_unit ON objects.object_kind = 'unit'
WHERE objects.object_kind = ANY(@object_kinds::text[])
ORDER BY object_kind, object_id;

-- name: ResolveLatestTFTAnalysisPatch :one
SELECT patch
FROM tft_lineup_publications
WHERE status = 'published'
  AND platform = @platform
  AND set_number = @set_number
  AND queue_id = @queue_id
  AND cohort = @cohort
  AND window_kind = @window_kind
ORDER BY published_at DESC, id DESC
LIMIT 1;

-- name: ListTFTLineupFamilies :many
SELECT f.*, p.platform, p.patch, p.set_number, p.queue_id, p.cohort,
       p.window_kind, p.algorithm_version, p.published_at,
       p.source_matches, p.source_participants, p.coverage
FROM tft_lineup_publications p
JOIN tft_lineup_families f ON f.publication_id = p.id
WHERE p.status = 'published'
  AND p.platform = @platform
  AND p.patch = @patch
  AND p.set_number = @set_number
  AND p.queue_id = @queue_id
  AND p.cohort = @cohort
  AND p.window_kind = @window_kind
  AND f.sample_size >= @min_samples
ORDER BY f.avg_placement ASC, f.top4_rate DESC, f.sample_size DESC
LIMIT @row_limit;

-- name: GetTFTLineupPublication :one
SELECT * FROM tft_lineup_publications
WHERE status = 'published'
  AND platform = @platform AND patch = @patch AND set_number = @set_number
  AND queue_id = @queue_id AND cohort = @cohort AND window_kind = @window_kind
LIMIT 1;
