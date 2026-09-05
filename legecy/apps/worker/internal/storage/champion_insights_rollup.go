package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	championInsightAlgorithm          = "OBSERVED_PERCENTILES_V1"
	championInsightMinGames           = 500
	championInsightMinPlayers         = 50
	championInsightMinBucketGames     = 50
	championInsightMinBucketPlayers   = 10
	championInsightMinNonEmptyBuckets = 5
)

var ErrChampionInsightInsufficientCoverage = errors.New("champion insight publication has insufficient coverage")

type championInsightGates struct {
	minGames           int
	minPlayers         int
	minBucketGames     int
	minBucketPlayers   int
	minNonEmptyBuckets int
}

var productionChampionInsightGates = championInsightGates{
	minGames:           championInsightMinGames,
	minPlayers:         championInsightMinPlayers,
	minBucketGames:     championInsightMinBucketGames,
	minBucketPlayers:   championInsightMinBucketPlayers,
	minNonEmptyBuckets: championInsightMinNonEmptyBuckets,
}

type ChampionWinFactorRollupRefresh struct {
	PublicationID      int64
	Revision           string
	RefreshedAt        time.Time
	DataThrough        *time.Time
	SourceMatches      int64
	EligibleMatches    int64
	SourceParticipants int64
	CohortRows         int64
	FactorRows         int64
	BucketRows         int64
	Duration           time.Duration
	Reused             bool
}

// RebuildChampionWinFactorRollups publishes a revisioned, descriptive JUNGLE
// dataset. Exactly one side is selected from each eligible match using MD5 so
// paired opponents cannot masquerade as independent observations. This path
// does not publish model-adjusted or causal estimates.
func (s *Store) RebuildChampionWinFactorRollups(ctx context.Context) (ChampionWinFactorRollupRefresh, error) {
	return s.rebuildChampionWinFactorRollups(ctx, productionChampionInsightGates)
}

func (s *Store) rebuildChampionWinFactorRollups(ctx context.Context, gates championInsightGates) (ChampionWinFactorRollupRefresh, error) {
	started := time.Now()
	result := ChampionWinFactorRollupRefresh{RefreshedAt: time.Now().UTC()}
	err := s.withChampionInsightSnapshot(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM matches m
			JOIN statistics_match_membership smm
			  ON smm.match_id=m.match_id AND smm.dataset_key='ranked-solo-v1'
			WHERE m.fetch_status='done' AND m.queue_id=420`).Scan(&result.SourceMatches); err != nil {
			return fmt.Errorf("count champion insight source matches: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			CREATE TEMP TABLE champion_insight_observations ON COMMIT DROP AS
			WITH participant_quality AS (
				SELECT m.match_id,
				       COUNT(mp.*)::int AS participant_rows,
				       COUNT(DISTINCT mp.participant_id) FILTER (WHERE mp.participant_id BETWEEN 1 AND 10)::int AS participant_ids,
				       COUNT(*) FILTER (WHERE mp.team_id=100)::int AS team_100_rows,
				       COUNT(*) FILTER (WHERE mp.team_id=200)::int AS team_200_rows,
				       COUNT(DISTINCT (mp.team_id, mp.team_position)) FILTER (
				           WHERE mp.team_id IN (100,200)
				             AND mp.team_position IN ('TOP','JUNGLE','MIDDLE','BOTTOM','UTILITY')
				       )::int AS team_positions,
				       COUNT(*) FILTER (
				           WHERE mp.champion_id IS NULL OR mp.champion_id <= 0
				              OR NULLIF(btrim(mp.champion_name), '') IS NULL
				              OR mp.win IS NULL
				       )::int AS invalid_rows,
				       COUNT(*) FILTER (WHERE mp.team_id=100 AND mp.win)::int AS team_100_wins,
				       COUNT(*) FILTER (WHERE mp.team_id=200 AND mp.win)::int AS team_200_wins
				FROM matches m
				JOIN statistics_match_membership smm
				  ON smm.match_id=m.match_id AND smm.dataset_key='ranked-solo-v1'
				LEFT JOIN match_participants mp ON mp.match_id=m.match_id
				WHERE m.fetch_status='done' AND m.queue_id=420
				GROUP BY m.match_id
			), eligible_matches AS (
				SELECT m.match_id, m.version, m.region, m.avg_tier AS tier_bucket,
				       m.game_end_ts,
				       CASE WHEN get_byte(decode(md5(m.match_id),'hex'),0) % 2 = 0 THEN 100 ELSE 200 END AS selected_team
				FROM matches m
				JOIN statistics_match_membership smm
				  ON smm.match_id=m.match_id AND smm.dataset_key='ranked-solo-v1'
				JOIN participant_quality pq ON pq.match_id=m.match_id
				WHERE m.fetch_status='done' AND m.timeline_status='done' AND m.queue_id=420
				  AND m.game_duration >= 900
				  AND m.game_end_ts IS NOT NULL
				  AND m.version <> '' AND m.region IN ('KR','NA1')
				  AND m.avg_tier IN ('IRON','BRONZE','SILVER','GOLD','PLATINUM','EMERALD','DIAMOND','MASTER','GRANDMASTER','CHALLENGER')
				  AND pq.participant_rows=10 AND pq.participant_ids=10
				  AND pq.team_100_rows=5 AND pq.team_200_rows=5
				  AND pq.team_positions=10 AND pq.invalid_rows=0
				  AND ((pq.team_100_wins=5 AND pq.team_200_wins=0) OR
				       (pq.team_100_wins=0 AND pq.team_200_wins=5))
			)
			SELECT em.match_id, em.version, em.region, em.tier_bucket, em.game_end_ts,
			       mp.puuid, mp.champion_id, mp.champion_name, mp.team_position, mp.win,
			       s10.jungle_cs AS jungle_cs_10,
			       s15.jungle_cs-s10.jungle_cs AS jungle_cs_gain_10_15,
			       s10.dmg_to_champs AS damage_to_champions_10,
			       s15.dmg_to_champs-s10.dmg_to_champs AS damage_to_champions_gain_10_15
			FROM eligible_matches em
			JOIN match_participants mp
			  ON mp.match_id=em.match_id AND mp.team_id=em.selected_team AND mp.team_position='JUNGLE'
			JOIN match_participant_snapshots s10
			  ON s10.match_id=mp.match_id AND s10.participant_id=mp.participant_id AND s10.minute=10
			JOIN match_participant_snapshots s15
			  ON s15.match_id=mp.match_id AND s15.participant_id=mp.participant_id AND s15.minute=15
			WHERE NULLIF(mp.puuid,'') IS NOT NULL
			  AND s10.jungle_cs IS NOT NULL AND s15.jungle_cs IS NOT NULL
			  AND s10.dmg_to_champs IS NOT NULL AND s15.dmg_to_champs IS NOT NULL
			  AND s15.jungle_cs >= s10.jungle_cs
			  AND s15.dmg_to_champs >= s10.dmg_to_champs;

			CREATE UNIQUE INDEX ON champion_insight_observations(match_id);
			ANALYZE champion_insight_observations`); err != nil {
			return fmt.Errorf("build champion insight observations: %w", err)
		}
		var inputFingerprint int64
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*), COUNT(*), MAX(game_end_ts),
			       COALESCE(bit_xor(hashtextextended(concat_ws(E'\x1f',
			           match_id,version,region,tier_bucket,puuid,champion_id::text,
			           champion_name,team_position,win::text,jungle_cs_10::text,
			           jungle_cs_gain_10_15::text,damage_to_champions_10::text,
			           damage_to_champions_gain_10_15::text
			       ),0)),0)::bigint
			FROM champion_insight_observations`).Scan(
			&result.EligibleMatches, &result.SourceParticipants, &result.DataThrough, &inputFingerprint,
		); err != nil {
			return fmt.Errorf("summarize champion insight observations: %w", err)
		}
		if result.EligibleMatches == 0 {
			return fmt.Errorf("%w: no eligible observations", ErrChampionInsightInsufficientCoverage)
		}
		stamp := result.DataThrough.UTC().Format("20060102T150405Z")
		result.Revision = fmt.Sprintf(
			"observed-percentiles-v1-g%d-p%d-bg%d-bp%d-nb%d-%s-%d-%d-%016x",
			gates.minGames, gates.minPlayers, gates.minBucketGames, gates.minBucketPlayers,
			gates.minNonEmptyBuckets, stamp, result.SourceMatches, result.EligibleMatches,
			uint64(inputFingerprint),
		)

		var existingStatus string
		if err := tx.QueryRow(ctx, `
			SELECT p.id,p.status,p.source_matches,p.eligible_matches,p.source_participants,p.data_through,
			       (SELECT COUNT(*) FROM champion_insight_cohorts c WHERE c.publication_id=p.id),
			       (SELECT COUNT(*) FROM champion_insight_factors f JOIN champion_insight_cohorts c ON c.id=f.cohort_id WHERE c.publication_id=p.id),
			       (SELECT COUNT(*) FROM champion_insight_factor_buckets b JOIN champion_insight_cohorts c ON c.id=b.cohort_id WHERE c.publication_id=p.id)
			FROM champion_insight_publications p
			WHERE p.revision=$1`, result.Revision).Scan(
			&result.PublicationID, &existingStatus, &result.SourceMatches, &result.EligibleMatches,
			&result.SourceParticipants, &result.DataThrough, &result.CohortRows,
			&result.FactorRows, &result.BucketRows,
		); err == nil {
			if existingStatus == "building" {
				return fmt.Errorf("champion insight revision %q is unexpectedly still building", result.Revision)
			}
			if existingStatus == "superseded" {
				if _, err := tx.Exec(ctx, `
					UPDATE champion_insight_publications
					SET status='superseded'
					WHERE dataset_key='ranked-solo-v1' AND status='published' AND id<>$1`, result.PublicationID); err != nil {
					return fmt.Errorf("supersede current champion insight publication: %w", err)
				}
				if _, err := tx.Exec(ctx, `
					UPDATE champion_insight_publications
					SET status='published',published_at=$2
					WHERE id=$1 AND status='superseded'`, result.PublicationID, result.RefreshedAt); err != nil {
					return fmt.Errorf("reactivate champion insight publication: %w", err)
				}
			}
			result.Reused = true
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("find existing champion insight publication: %w", err)
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO champion_insight_publications (
				revision,dataset_key,algorithm_version,feature_schema_version,status,
				source_matches,eligible_matches,source_participants,data_through,coverage
			) VALUES ($1,'ranked-solo-v1',$2,1,'building',$3,$4,$5,$6,'{}'::jsonb)
			RETURNING id`, result.Revision, championInsightAlgorithm, result.SourceMatches,
			result.EligibleMatches, result.SourceParticipants, result.DataThrough).Scan(&result.PublicationID); err != nil {
			return fmt.Errorf("create champion insight publication: %w", err)
		}

		tag, err := tx.Exec(ctx, `
			WITH region_scopes(region_scope) AS (VALUES ('ALL'::text),('KR'),('NA1')),
			tier_groups(tier_group) AS (
				VALUES ('ALL'::text),('MASTER'),('MASTER_PLUS'),('GRANDMASTER'),('GRANDMASTER_PLUS'),('CHALLENGER')
			), cohorts AS (
				SELECT o.version, rs.region_scope, tg.tier_group,
				       o.champion_id, MAX(o.champion_name)::text AS champion_name,
				       COUNT(*)::bigint AS sample_games,
				       COUNT(DISTINCT o.puuid)::bigint AS sample_players
				FROM champion_insight_observations o
				CROSS JOIN region_scopes rs
				CROSS JOIN tier_groups tg
				WHERE (rs.region_scope='ALL' OR rs.region_scope=o.region)
				  AND (tg.tier_group='ALL'
				       OR (tg.tier_group='MASTER' AND o.tier_bucket='MASTER')
				       OR (tg.tier_group='MASTER_PLUS' AND o.tier_bucket IN ('MASTER','GRANDMASTER','CHALLENGER'))
				       OR (tg.tier_group='GRANDMASTER' AND o.tier_bucket='GRANDMASTER')
				       OR (tg.tier_group='GRANDMASTER_PLUS' AND o.tier_bucket IN ('GRANDMASTER','CHALLENGER'))
				       OR (tg.tier_group='CHALLENGER' AND o.tier_bucket='CHALLENGER'))
				GROUP BY o.version, rs.region_scope, tg.tier_group, o.champion_id
			)
			INSERT INTO champion_insight_cohorts (
				publication_id,queue_id,version,region_scope,tier_group,champion_id,
				champion_name,team_position,sample_games,sample_players,
				availability,reason_code,cohort_scope
			)
			SELECT $1,420,version,region_scope,tier_group,champion_id,champion_name,'JUNGLE',
			       sample_games,sample_players,
			       CASE WHEN sample_games >= $2 AND sample_players >= $3
			            THEN 'AVAILABLE' ELSE 'INSUFFICIENT_SAMPLE' END,
			       CASE WHEN sample_games < $2 THEN 'MIN_GAMES'
			            WHEN sample_players < $3 THEN 'MIN_PLAYERS' END,
			       'CHAMPION_POSITION'
			FROM cohorts`, result.PublicationID, gates.minGames, gates.minPlayers)
		if err != nil {
			return fmt.Errorf("insert champion insight cohorts: %w", err)
		}
		result.CohortRows = tag.RowsAffected()

		if _, err := tx.Exec(ctx, `
			CREATE TEMP TABLE champion_insight_metric_values ON COMMIT DROP AS
			SELECT c.id AS cohort_id, metric.metric_key, metric.metric_value, o.win, o.puuid
			FROM champion_insight_cohorts c
			JOIN champion_insight_observations o
			  ON o.version=c.version AND o.champion_id=c.champion_id
			 AND (c.region_scope='ALL' OR c.region_scope=o.region)
			 AND (c.tier_group='ALL'
			      OR (c.tier_group='MASTER' AND o.tier_bucket='MASTER')
			      OR (c.tier_group='MASTER_PLUS' AND o.tier_bucket IN ('MASTER','GRANDMASTER','CHALLENGER'))
			      OR (c.tier_group='GRANDMASTER' AND o.tier_bucket='GRANDMASTER')
			      OR (c.tier_group='GRANDMASTER_PLUS' AND o.tier_bucket IN ('GRANDMASTER','CHALLENGER'))
			      OR (c.tier_group='CHALLENGER' AND o.tier_bucket='CHALLENGER'))
			CROSS JOIN LATERAL (VALUES
				('JUNGLE_CS_GAIN_10_15'::text,o.jungle_cs_gain_10_15::numeric),
				('DAMAGE_TO_CHAMPIONS_GAIN_10_15',o.damage_to_champions_gain_10_15::numeric),
				('DAMAGE_TO_CHAMPIONS_10',o.damage_to_champions_10::numeric),
				('JUNGLE_CS_10',o.jungle_cs_10::numeric)
			) metric(metric_key,metric_value)
			WHERE c.publication_id=$1 AND c.availability='AVAILABLE'`, result.PublicationID); err != nil {
			return fmt.Errorf("build champion insight metric values: %w", err)
		}
		if _, err := tx.Exec(ctx, `CREATE INDEX ON champion_insight_metric_values(cohort_id,metric_key,metric_value)`); err != nil {
			return fmt.Errorf("index champion insight metric values: %w", err)
		}
		if _, err := tx.Exec(ctx, `ANALYZE champion_insight_metric_values`); err != nil {
			return fmt.Errorf("analyze champion insight metric values: %w", err)
		}

		tag, err = tx.Exec(ctx, `
			INSERT INTO champion_insight_factors (
				cohort_id,metric_key,kind,start_minute,end_minute,unit,direction,
				p50,p70,p90,evidence_grade,display_order
			)
			SELECT cohort_id,metric_key,'BEHAVIOR_METRIC',
			       CASE WHEN metric_key LIKE '%GAIN%' THEN 10 ELSE 0 END,
			       CASE WHEN metric_key LIKE '%GAIN%' THEN 15 ELSE 10 END,
			       CASE WHEN metric_key LIKE 'JUNGLE_CS%' THEN 'COUNT' ELSE 'DAMAGE' END,
			       'OBSERVED_TREND',
			       percentile_disc(0.5) WITHIN GROUP (ORDER BY metric_value),
			       percentile_disc(0.7) WITHIN GROUP (ORDER BY metric_value),
			       percentile_disc(0.9) WITHIN GROUP (ORDER BY metric_value),
			       'OBSERVED',
			       CASE metric_key
			           WHEN 'JUNGLE_CS_GAIN_10_15' THEN 1
			           WHEN 'DAMAGE_TO_CHAMPIONS_GAIN_10_15' THEN 2
			           WHEN 'DAMAGE_TO_CHAMPIONS_10' THEN 3
			           ELSE 4
			       END
			FROM champion_insight_metric_values
			GROUP BY cohort_id,metric_key`)
		if err != nil {
			return fmt.Errorf("insert champion insight factors: %w", err)
		}
		result.FactorRows = tag.RowsAffected()

		tag, err = tx.Exec(ctx, `
			WITH ranked AS (
				SELECT cohort_id,metric_key,metric_value,win,puuid,
				       LEAST(floor(percent_rank() OVER (
				           PARTITION BY cohort_id,metric_key ORDER BY metric_value
				       ) * 10)::int + 1, 10) AS ordinal
				FROM champion_insight_metric_values
			)
			INSERT INTO champion_insight_factor_buckets (
				cohort_id,metric_key,ordinal,lower_bound,upper_bound,games,wins,sample_players
			)
			SELECT cohort_id,metric_key,ordinal,MIN(metric_value),MAX(metric_value),
			       COUNT(*)::bigint,COUNT(*) FILTER (WHERE win)::bigint,
			       COUNT(DISTINCT puuid)::bigint
			FROM ranked
			GROUP BY cohort_id,metric_key,ordinal`)
		if err != nil {
			return fmt.Errorf("insert champion insight buckets: %w", err)
		}
		result.BucketRows = tag.RowsAffected()

		if _, err := tx.Exec(ctx, `
			DELETE FROM champion_insight_factors f
			WHERE EXISTS (
				SELECT 1
				FROM champion_insight_factor_buckets b
				WHERE b.cohort_id=f.cohort_id AND b.metric_key=f.metric_key
				GROUP BY b.cohort_id,b.metric_key
				HAVING COUNT(*) < $3
				    OR bool_or(b.games < $1 OR b.sample_players < $2)
			)`, gates.minBucketGames, gates.minBucketPlayers, gates.minNonEmptyBuckets); err != nil {
			return fmt.Errorf("suppress sparse champion insight factors: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			WITH factor_coverage AS (
				SELECT c.id,COUNT(f.*)::int AS factor_count
				FROM champion_insight_cohorts c
				LEFT JOIN champion_insight_factors f ON f.cohort_id=c.id
				WHERE c.publication_id=$1 AND c.availability='AVAILABLE'
				GROUP BY c.id
			)
			UPDATE champion_insight_cohorts c
			SET availability='INSUFFICIENT_SAMPLE',reason_code='SPARSE_BUCKETS'
			FROM factor_coverage fc
			WHERE c.id=fc.id AND fc.factor_count<>4`, result.PublicationID); err != nil {
			return fmt.Errorf("mark incomplete champion insight cohorts unavailable: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM champion_insight_factors f
			USING champion_insight_cohorts c
			WHERE f.cohort_id=c.id AND c.publication_id=$1 AND c.availability<>'AVAILABLE'`, result.PublicationID); err != nil {
			return fmt.Errorf("remove incomplete champion insight cohorts: %w", err)
		}
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM champion_insight_factors WHERE cohort_id IN (SELECT id FROM champion_insight_cohorts WHERE publication_id=$1)`, result.PublicationID).Scan(&result.FactorRows); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM champion_insight_factor_buckets WHERE cohort_id IN (SELECT id FROM champion_insight_cohorts WHERE publication_id=$1)`, result.PublicationID).Scan(&result.BucketRows); err != nil {
			return err
		}
		if result.FactorRows == 0 || result.BucketRows == 0 {
			return fmt.Errorf("%w: no complete cohorts passed factor and bucket gates", ErrChampionInsightInsufficientCoverage)
		}
		var violations int64
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM (
				SELECT f.cohort_id,f.metric_key,c.sample_games,
				       COALESCE(SUM(b.games),0)::bigint AS bucket_games
				FROM champion_insight_factors f
				JOIN champion_insight_cohorts c ON c.id=f.cohort_id
				LEFT JOIN champion_insight_factor_buckets b
				  ON b.cohort_id=f.cohort_id AND b.metric_key=f.metric_key
				WHERE c.publication_id=$1
				GROUP BY f.cohort_id,f.metric_key,c.sample_games
			) x WHERE sample_games<>bucket_games`, result.PublicationID).Scan(&violations); err != nil {
			return err
		}
		if violations != 0 {
			return fmt.Errorf("champion insight bucket reconciliation failed: %d factors", violations)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE champion_insight_publications
			SET coverage=jsonb_build_object(
				'cohorts',$2::bigint,'factors',$3::bigint,'buckets',$4::bigint,
				'minGames',$5::int,'minPlayers',$6::int,'minBucketGames',$7::int,
				'minBucketPlayers',$8::int,'minNonEmptyBuckets',$9::int,
				'position','JUNGLE','metrics',4
			)
			WHERE id=$1`, result.PublicationID, result.CohortRows, result.FactorRows,
			result.BucketRows, gates.minGames, gates.minPlayers, gates.minBucketGames,
			gates.minBucketPlayers, gates.minNonEmptyBuckets); err != nil {
			return fmt.Errorf("record champion insight coverage: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE champion_insight_publications
			SET status='superseded'
			WHERE dataset_key='ranked-solo-v1' AND status='published' AND id<>$1`, result.PublicationID); err != nil {
			return fmt.Errorf("supersede prior champion insight publication: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE champion_insight_publications
			SET status='published',published_at=$2
			WHERE id=$1 AND status='building'`, result.PublicationID, result.RefreshedAt); err != nil {
			return fmt.Errorf("publish champion insights: %w", err)
		}
		return nil
	})
	result.Duration = time.Since(started)
	return result, err
}

// withChampionInsightSnapshot serializes publishers before opening the
// repeatable-read transaction. Acquiring the session lock first is important:
// a transaction-scoped lock would establish an old snapshot while waiting for
// another publisher and could miss the publication it just committed.
func (s *Store) withChampionInsightSnapshot(ctx context.Context, fn func(pgx.Tx) error) (err error) {
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire champion insight connection: %w", err)
	}
	locked := false
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if locked {
			var unlocked bool
			unlockErr := conn.QueryRow(cleanupCtx, `SELECT pg_advisory_unlock(hashtextextended('gogg:champion_insight_publish', 0))`).Scan(&unlocked)
			if unlockErr != nil || !unlocked {
				_ = conn.Conn().Close(cleanupCtx)
				if err == nil {
					err = fmt.Errorf("unlock champion insight publication: %w", unlockErr)
				}
			}
		}
		conn.Release()
	}()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended('gogg:champion_insight_publish', 0))`); err != nil {
		return fmt.Errorf("lock champion insight publication: %w", err)
	}
	locked = true

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return fmt.Errorf("begin champion insight snapshot: %w", err)
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit champion insight publication: %w", err)
	}
	return nil
}
