package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// RankingsRollupRefresh describes one committed rebuild of the rankings
// rollups. Row counts are aggregate-table rows, not source facts.
type RankingsRollupRefresh struct {
	RefreshedAt                     time.Time
	DataThrough                     *time.Time
	SourceCompletedMatches          int64
	EligibleMatches                 int64
	ExcludedMetadataMatches         int64
	ExcludedTierMatches             int64
	ExcludedDurationMatches         int64
	ExcludedParticipantShapeMatches int64
	ExcludedParticipantFactsMatches int64
	ExcludedBanShapeMatches         int64
	ChampionPositionRows            int64
	BanRows                         int64
	MatchCountRows                  int64
	Duration                        time.Duration
}

// RebuildRankingsRollups replaces all rankings rollups in one transaction.
// Eligibility is classified once and shared by every aggregate so champion,
// ban, and denominator data can never be built from different match sets.
func (s *Store) RebuildRankingsRollups(ctx context.Context) (RankingsRollupRefresh, error) {
	started := time.Now()
	var result RankingsRollupRefresh

	err := s.WithTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('gogg:rankings_rollup_refresh', 0))`); err != nil {
			return fmt.Errorf("lock rankings rollup refresh: %w", err)
		}

		// Keep every completed ranked-solo match in this table, including
		// matches with no participant rows, so exclusion counts reconcile to
		// the source count. CASE order defines the mutually exclusive reason.
		if _, err := tx.Exec(ctx, `
			CREATE TEMP TABLE rankings_match_quality ON COMMIT DROP AS
			WITH participant_quality AS (
				SELECT
					m.match_id,
					COUNT(mp.*)::int AS participant_rows,
					COUNT(DISTINCT mp.participant_id) FILTER (
						WHERE mp.participant_id BETWEEN 1 AND 10
					)::int AS participant_ids,
					COUNT(*) FILTER (WHERE mp.team_id = 100)::int AS team_100_rows,
					COUNT(*) FILTER (WHERE mp.team_id = 200)::int AS team_200_rows,
					COUNT(*) FILTER (
						WHERE mp.team_position IN ('TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY')
					)::int AS canonical_position_rows,
					COUNT(DISTINCT (mp.team_id, mp.team_position)) FILTER (
						WHERE mp.team_id IN (100, 200)
						  AND mp.team_position IN ('TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY')
					)::int AS team_positions,
					COUNT(*) FILTER (
						WHERE mp.champion_id IS NULL OR mp.champion_id <= 0
						   OR NULLIF(btrim(mp.champion_name), '') IS NULL
						   OR mp.win IS NULL
						   OR mp.kills IS NULL OR mp.kills < 0
						   OR mp.deaths IS NULL OR mp.deaths < 0
						   OR mp.assists IS NULL OR mp.assists < 0
					)::int AS invalid_fact_rows,
					COUNT(DISTINCT mp.champion_id) FILTER (WHERE mp.champion_id > 0)::int AS distinct_champions,
					COUNT(*) FILTER (WHERE mp.team_id = 100 AND mp.win)::int AS team_100_wins,
					COUNT(*) FILTER (WHERE mp.team_id = 200 AND mp.win)::int AS team_200_wins
				FROM matches m
				LEFT JOIN match_participants mp ON mp.match_id = m.match_id
				WHERE m.fetch_status = 'done' AND m.queue_id = 420
				GROUP BY m.match_id
			), ban_quality AS (
				SELECT
					m.match_id,
					COUNT(mb.*)::int AS ban_rows,
					COUNT(*) FILTER (WHERE mb.team_id = 100)::int AS team_100_bans,
					COUNT(*) FILTER (WHERE mb.team_id = 200)::int AS team_200_bans,
					COUNT(*) FILTER (
						WHERE (mb.team_id = 100 AND mb.pick_turn BETWEEN 1 AND 5)
						   OR (mb.team_id = 200 AND mb.pick_turn BETWEEN 6 AND 10)
					)::int AS canonical_ban_slots,
					COUNT(DISTINCT (mb.team_id, mb.pick_turn)) FILTER (
						WHERE (mb.team_id = 100 AND mb.pick_turn BETWEEN 1 AND 5)
						   OR (mb.team_id = 200 AND mb.pick_turn BETWEEN 6 AND 10)
					)::int AS team_pick_turns,
					COUNT(*) FILTER (
						WHERE mb.match_id IS NOT NULL AND mb.champion_id IS NULL
					)::int AS null_champion_ids
				FROM matches m
				LEFT JOIN match_bans mb ON mb.match_id = m.match_id
				WHERE m.fetch_status = 'done' AND m.queue_id = 420
				GROUP BY m.match_id
			)
			SELECT
				m.match_id, m.queue_id, m.version, m.region, m.avg_tier,
				m.game_end_ts,
				CASE
					WHEN NULLIF(btrim(m.version), '') IS NULL
					  OR NULLIF(btrim(m.region), '') IS NULL
						THEN 'metadata'
					WHEN m.avg_tier IS NULL OR m.avg_tier NOT IN (
						'IRON', 'BRONZE', 'SILVER', 'GOLD', 'PLATINUM',
						'EMERALD', 'DIAMOND', 'MASTER', 'GRANDMASTER', 'CHALLENGER'
					) THEN 'tier'
					WHEN m.game_duration IS NULL OR m.game_duration < 600
						THEN 'duration'
					WHEN pq.participant_rows <> 10
					  OR pq.participant_ids <> 10
					  OR pq.team_100_rows <> 5
					  OR pq.team_200_rows <> 5
					  OR pq.canonical_position_rows <> 10
					  OR pq.team_positions <> 10
						THEN 'participant_shape'
					WHEN pq.invalid_fact_rows <> 0
					  OR pq.distinct_champions <> 10
					  OR NOT (
						(pq.team_100_wins = 5 AND pq.team_200_wins = 0)
						OR (pq.team_100_wins = 0 AND pq.team_200_wins = 5)
					  ) THEN 'participant_facts'
					WHEN bq.ban_rows <> 10
					  OR bq.team_100_bans <> 5
					  OR bq.team_200_bans <> 5
					  OR bq.canonical_ban_slots <> 10
					  OR bq.team_pick_turns <> 10
					  OR bq.null_champion_ids <> 0
						THEN 'ban_shape'
					ELSE 'eligible'
				END::text AS quality
			FROM matches m
			INNER JOIN participant_quality pq ON pq.match_id = m.match_id
			INNER JOIN ban_quality bq ON bq.match_id = m.match_id
			WHERE m.fetch_status = 'done' AND m.queue_id = 420`); err != nil {
			return fmt.Errorf("classify rankings matches: %w", err)
		}
		if _, err := tx.Exec(ctx, `CREATE UNIQUE INDEX ON rankings_match_quality (match_id); ANALYZE rankings_match_quality`); err != nil {
			return fmt.Errorf("index rankings match quality: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			CREATE TEMP TABLE rankings_position_stage ON COMMIT DROP AS
			SELECT
				q.queue_id,
				q.version,
				q.region,
				q.avg_tier AS tier_bucket,
				mp.champion_id,
				MAX(mp.champion_name)::text AS champion_name,
				mp.team_position,
				COUNT(*)::bigint AS games,
				COUNT(*) FILTER (WHERE mp.win)::bigint AS wins,
				SUM((mp.kills + mp.assists)::float8 / GREATEST(mp.deaths, 1))::float8
					AS kda_contribution_sum
			FROM rankings_match_quality q
			INNER JOIN match_participants mp ON mp.match_id = q.match_id
			WHERE q.quality = 'eligible'
			GROUP BY q.queue_id, q.version, q.region, q.avg_tier,
			         mp.champion_id, mp.team_position;

			CREATE TEMP TABLE rankings_ban_stage ON COMMIT DROP AS
			SELECT
				q.queue_id,
				q.version,
				q.region,
				q.avg_tier AS tier_bucket,
				mb.champion_id,
				COUNT(DISTINCT mb.match_id)::bigint AS ban_matches
			FROM rankings_match_quality q
			INNER JOIN match_bans mb ON mb.match_id = q.match_id
			WHERE q.quality = 'eligible' AND mb.champion_id > 0
			GROUP BY q.queue_id, q.version, q.region, q.avg_tier, mb.champion_id;

			CREATE TEMP TABLE rankings_match_count_stage ON COMMIT DROP AS
			SELECT
				queue_id,
				version,
				region,
				avg_tier AS tier_bucket,
				COUNT(*)::bigint AS total_matches
			FROM rankings_match_quality
			WHERE quality = 'eligible'
			GROUP BY queue_id, version, region, avg_tier`); err != nil {
			return fmt.Errorf("build rankings staging tables: %w", err)
		}

		var invariantViolations int64
		if err := tx.QueryRow(ctx, `
			WITH dimension_totals AS (
				SELECT
					mc.queue_id, mc.version, mc.region, mc.tier_bucket,
					mc.total_matches,
					COALESCE(SUM(p.games), 0)::bigint AS games,
					COALESCE(SUM(p.wins), 0)::bigint AS wins
				FROM rankings_match_count_stage mc
				LEFT JOIN rankings_position_stage p USING (queue_id, version, region, tier_bucket)
				GROUP BY mc.queue_id, mc.version, mc.region, mc.tier_bucket, mc.total_matches
			), position_totals AS (
				SELECT
					mc.queue_id, mc.version, mc.region, mc.tier_bucket,
					pos.team_position,
					mc.total_matches,
					COALESCE(SUM(p.games), 0)::bigint AS games
				FROM rankings_match_count_stage mc
				CROSS JOIN (VALUES ('TOP'), ('JUNGLE'), ('MIDDLE'), ('BOTTOM'), ('UTILITY')) pos(team_position)
				LEFT JOIN rankings_position_stage p
				  ON p.queue_id = mc.queue_id
				 AND p.version = mc.version
				 AND p.region = mc.region
				 AND p.tier_bucket = mc.tier_bucket
				 AND p.team_position = pos.team_position
				GROUP BY mc.queue_id, mc.version, mc.region, mc.tier_bucket,
				         pos.team_position, mc.total_matches
			), violations AS (
				SELECT 1 FROM dimension_totals
				WHERE games <> total_matches * 10 OR wins <> total_matches * 5
				UNION ALL
				SELECT 1 FROM position_totals WHERE games <> total_matches * 2
			)
			SELECT COUNT(*)::bigint FROM violations`).Scan(&invariantViolations); err != nil {
			return fmt.Errorf("validate rankings staging tables: %w", err)
		}
		if invariantViolations != 0 {
			return fmt.Errorf("validate rankings staging tables: %d aggregate invariant violations", invariantViolations)
		}

		for _, table := range []string{
			"rankings_champion_position_rollup",
			"rankings_champion_ban_rollup",
			"rankings_match_count_rollup",
			"rankings_rollup_state",
		} {
			if _, err := tx.Exec(ctx, "DELETE FROM "+table); err != nil {
				return fmt.Errorf("clear %s: %w", table, err)
			}
		}

		positionTag, err := tx.Exec(ctx, `
			INSERT INTO rankings_champion_position_rollup
			SELECT * FROM rankings_position_stage`)
		if err != nil {
			return fmt.Errorf("publish champion-position rankings rollup: %w", err)
		}
		result.ChampionPositionRows = positionTag.RowsAffected()

		banTag, err := tx.Exec(ctx, `
			INSERT INTO rankings_champion_ban_rollup
			SELECT * FROM rankings_ban_stage`)
		if err != nil {
			return fmt.Errorf("publish champion-ban rankings rollup: %w", err)
		}
		result.BanRows = banTag.RowsAffected()

		matchCountTag, err := tx.Exec(ctx, `
			INSERT INTO rankings_match_count_rollup
			SELECT * FROM rankings_match_count_stage`)
		if err != nil {
			return fmt.Errorf("publish match-count rankings rollup: %w", err)
		}
		result.MatchCountRows = matchCountTag.RowsAffected()

		if err := tx.QueryRow(ctx, `
			WITH summary AS (
				SELECT
					COUNT(*)::bigint AS source_completed_matches,
					COUNT(*) FILTER (WHERE quality = 'eligible')::bigint AS eligible_matches,
					COUNT(*) FILTER (WHERE quality = 'metadata')::bigint AS excluded_metadata_matches,
					COUNT(*) FILTER (WHERE quality = 'tier')::bigint AS excluded_tier_matches,
					COUNT(*) FILTER (WHERE quality = 'duration')::bigint AS excluded_duration_matches,
					COUNT(*) FILTER (WHERE quality = 'participant_shape')::bigint AS excluded_participant_shape_matches,
					COUNT(*) FILTER (WHERE quality = 'participant_facts')::bigint AS excluded_participant_facts_matches,
					COUNT(*) FILTER (WHERE quality = 'ban_shape')::bigint AS excluded_ban_shape_matches,
					MAX(game_end_ts) FILTER (WHERE quality = 'eligible') AS data_through
				FROM rankings_match_quality
			), inserted AS (
				INSERT INTO rankings_rollup_state (
					singleton, refreshed_at, data_through,
					source_completed_matches, eligible_matches,
					excluded_metadata_matches, excluded_tier_matches,
					excluded_duration_matches, excluded_participant_shape_matches,
					excluded_participant_facts_matches, excluded_ban_shape_matches,
					champion_position_rows, ban_rows, match_count_rows
				)
				SELECT true, now(), data_through,
				       source_completed_matches, eligible_matches,
				       excluded_metadata_matches, excluded_tier_matches,
				       excluded_duration_matches, excluded_participant_shape_matches,
				       excluded_participant_facts_matches, excluded_ban_shape_matches,
				       $1, $2, $3
				FROM summary
				RETURNING refreshed_at, data_through, source_completed_matches,
				          eligible_matches, excluded_metadata_matches,
				          excluded_tier_matches, excluded_duration_matches,
				          excluded_participant_shape_matches,
				          excluded_participant_facts_matches,
				          excluded_ban_shape_matches
			)
			SELECT * FROM inserted`,
			result.ChampionPositionRows, result.BanRows, result.MatchCountRows,
		).Scan(
			&result.RefreshedAt,
			&result.DataThrough,
			&result.SourceCompletedMatches,
			&result.EligibleMatches,
			&result.ExcludedMetadataMatches,
			&result.ExcludedTierMatches,
			&result.ExcludedDurationMatches,
			&result.ExcludedParticipantShapeMatches,
			&result.ExcludedParticipantFactsMatches,
			&result.ExcludedBanShapeMatches,
		); err != nil {
			return fmt.Errorf("record rankings rollup refresh: %w", err)
		}

		if _, err := tx.Exec(ctx, `ANALYZE rankings_champion_position_rollup, rankings_champion_ban_rollup, rankings_match_count_rollup`); err != nil {
			return fmt.Errorf("analyze rankings rollups: %w", err)
		}
		return nil
	})
	if err != nil {
		return RankingsRollupRefresh{}, err
	}

	result.Duration = time.Since(started)
	return result, nil
}
