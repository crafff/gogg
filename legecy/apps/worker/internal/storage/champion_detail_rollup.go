package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type ChampionDetailRollupRefresh struct {
	RefreshedAt   time.Time
	SourceMatches int64
	AggregateRows int64
	Duration      time.Duration
}

// RebuildChampionDetailRollups atomically replaces the read-optimized build
// aggregates. Each category is a partition of only the participants for which
// that category is available, so its summed games form the UI denominator.
func (s *Store) RebuildChampionDetailRollups(ctx context.Context) (ChampionDetailRollupRefresh, error) {
	started := time.Now()
	result := ChampionDetailRollupRefresh{RefreshedAt: time.Now().UTC()}
	err := s.WithTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('gogg:champion_detail_rollup_refresh', 0))`); err != nil {
			return fmt.Errorf("lock champion detail rollup refresh: %w", err)
		}
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM matches m
			JOIN statistics_match_membership smm
			  ON smm.match_id=m.match_id AND smm.dataset_key='ranked-solo-v1'
			WHERE m.fetch_status='done' AND m.queue_id=420`).Scan(&result.SourceMatches); err != nil {
			return fmt.Errorf("count champion detail source matches: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			CREATE TEMP TABLE champion_detail_stage (
				LIKE champion_detail_rollup INCLUDING DEFAULTS INCLUDING CONSTRAINTS
			) ON COMMIT DROP;

			WITH eligible AS (
				SELECT m.match_id, m.queue_id, m.version, m.region, m.avg_tier AS tier_bucket,
				       mp.participant_id, mp.puuid, mp.champion_id, mp.champion_name,
				       mp.team_position, mp.win, mp.summoner1_id, mp.summoner2_id,
				       m.items_status
				FROM matches m
				JOIN statistics_match_membership smm
				  ON smm.match_id=m.match_id AND smm.dataset_key='ranked-solo-v1'
				JOIN match_participants mp ON mp.match_id=m.match_id
				WHERE m.fetch_status='done' AND m.queue_id=420 AND m.game_duration >= 600
				  AND m.version <> '' AND m.region <> ''
				  AND m.avg_tier IN ('IRON','BRONZE','SILVER','GOLD','PLATINUM','EMERALD','DIAMOND','MASTER','GRANDMASTER','CHALLENGER')
				  AND mp.champion_id > 0 AND mp.champion_name <> '' AND mp.win IS NOT NULL
				  AND mp.team_position IN ('TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY')
			), facts AS (
				SELECT e.queue_id,e.version,e.region,e.tier_bucket,e.champion_id,e.champion_name,e.team_position,
				       'RUNES'::text category,0::smallint stage,
				       ARRAY[p.style0,p.style1,p.perk0,p.perk1,p.perk2,p.perk3,p.perk4,p.perk5,p.stat_offense,p.stat_flex,p.stat_defense]::int[] signature,e.win
				FROM eligible e JOIN match_perks p ON p.match_id=e.match_id AND p.puuid=e.puuid
				WHERE p.style0 > 0 AND p.style1 > 0 AND p.perk0 > 0 AND p.perk1 > 0 AND p.perk2 > 0 AND p.perk3 > 0 AND p.perk4 > 0 AND p.perk5 > 0
				UNION ALL
				SELECT e.queue_id,e.version,e.region,e.tier_bucket,e.champion_id,e.champion_name,e.team_position,
				       'SPELLS',0,ARRAY[LEAST(e.summoner1_id,e.summoner2_id),GREATEST(e.summoner1_id,e.summoner2_id)]::int[],e.win
				FROM eligible e WHERE e.summoner1_id > 0 AND e.summoner2_id > 0
				UNION ALL
				SELECT e.queue_id,e.version,e.region,e.tier_bucket,e.champion_id,e.champion_name,e.team_position,
				       'STARTER',0,s.signature,e.win
				FROM eligible e JOIN LATERAL (
					SELECT ARRAY_AGG(si.item_id ORDER BY si.timestamp_ms,si.item_id)::int[] signature
					FROM match_starter_items si WHERE si.match_id=e.match_id AND si.participant_id=e.participant_id
				) s ON cardinality(s.signature)>0 WHERE e.items_status='done'
				UNION ALL
				SELECT e.queue_id,e.version,e.region,e.tier_bucket,e.champion_id,e.champion_name,e.team_position,
				       'BOOTS',0,ARRAY[b.item_id]::int[],e.win
				FROM eligible e JOIN match_boots b ON b.match_id=e.match_id AND b.participant_id=e.participant_id
				WHERE e.items_status='done'
			), ordered_items AS (
				SELECT e.*, ci.item_id,
				       ROW_NUMBER() OVER (PARTITION BY e.match_id,e.participant_id ORDER BY ci.timestamp_ms,ci.slot)::int rn
				FROM eligible e JOIN match_completed_items ci ON ci.match_id=e.match_id AND ci.participant_id=e.participant_id
				WHERE e.items_status='done' AND NOT ci.is_boots
			), item_sequences AS (
				SELECT queue_id,version,region,tier_bucket,champion_id,champion_name,team_position,
				       match_id,participant_id,win,ARRAY_AGG(item_id ORDER BY rn)::int[] items
				FROM ordered_items
				GROUP BY queue_id,version,region,tier_bucket,champion_id,champion_name,team_position,match_id,participant_id,win
			), item_facts AS (
				SELECT queue_id,version,region,tier_bucket,champion_id,champion_name,team_position,
				       'ITEMS'::text category,n.stage::smallint stage,items[1:n.stage]::int[] signature,win
				FROM item_sequences CROSS JOIN (VALUES (3),(4),(5),(6)) n(stage)
				WHERE cardinality(items) >= n.stage
			), all_facts AS (
				SELECT * FROM facts UNION ALL SELECT * FROM item_facts
			)
			INSERT INTO champion_detail_stage
			(queue_id,version,region,tier_bucket,champion_id,champion_name,team_position,category,stage,signature,games,wins)
			SELECT queue_id,version,region,tier_bucket,champion_id,MAX(champion_name),team_position,category,stage,signature,
			       COUNT(*)::bigint,COUNT(*) FILTER (WHERE win)::bigint
			FROM all_facts
			GROUP BY queue_id,version,region,tier_bucket,champion_id,team_position,category,stage,signature;
			ANALYZE champion_detail_stage`); err != nil {
			return fmt.Errorf("build champion detail staging rows: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM champion_detail_rollup`); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `INSERT INTO champion_detail_rollup SELECT * FROM champion_detail_stage`)
		if err != nil {
			return fmt.Errorf("publish champion detail rollups: %w", err)
		}
		result.AggregateRows = tag.RowsAffected()
		if _, err := tx.Exec(ctx, `
			INSERT INTO champion_detail_rollup_state(singleton,refreshed_at,source_matches,aggregate_rows)
			VALUES(true,$1,$2,$3)
			ON CONFLICT(singleton) DO UPDATE SET refreshed_at=EXCLUDED.refreshed_at,source_matches=EXCLUDED.source_matches,aggregate_rows=EXCLUDED.aggregate_rows`,
			result.RefreshedAt, result.SourceMatches, result.AggregateRows); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `ANALYZE champion_detail_rollup`)
		return err
	})
	result.Duration = time.Since(started)
	return result, err
}
