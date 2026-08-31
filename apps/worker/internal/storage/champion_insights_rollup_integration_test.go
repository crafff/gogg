//go:build integration

package storage

import (
	"context"
	"fmt"
	"testing"

	canonicalmigrations "github.com/crafff/gogg/packages/sqlc/migrations"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/require"
)

func TestChampionInsightsPublicationIsolatedAndIdempotent(t *testing.T) {
	store := newRankingsTestStore(t)
	insertInsightMatch(t, store, "scheduled", true)
	insertInsightMatch(t, store, "on-demand", false)
	insertInsightMatch(t, store, "null-end", true)
	mustExec(t, store, `UPDATE matches SET game_end_ts=NULL WHERE match_id='null-end'`)
	gates := championInsightGates{
		minGames: 1, minPlayers: 1, minBucketGames: 1,
		minBucketPlayers: 1, minNonEmptyBuckets: 1,
	}

	first, err := store.rebuildChampionWinFactorRollups(context.Background(), gates)
	require.NoError(t, err)
	require.False(t, first.Reused)
	require.EqualValues(t, 2, first.SourceMatches)
	require.EqualValues(t, 1, first.EligibleMatches)
	require.EqualValues(t, 1, first.SourceParticipants)
	// MASTER belongs to ALL, MASTER, and MASTER_PLUS; each is published for
	// ALL regions and KR, giving six exact request cohorts.
	require.EqualValues(t, 6, first.CohortRows)
	require.EqualValues(t, 24, first.FactorRows)
	require.EqualValues(t, 24, first.BucketRows)
	var bucketPlayers int
	mustQueryRow(t, store, `SELECT sample_players FROM champion_insight_factor_buckets LIMIT 1`).Scan(&bucketPlayers)
	require.Equal(t, 1, bucketPlayers)

	var publications int
	mustQueryRow(t, store, `SELECT COUNT(*) FROM champion_insight_publications WHERE status='published'`).Scan(&publications)
	require.Equal(t, 1, publications)

	second, err := store.rebuildChampionWinFactorRollups(context.Background(), gates)
	require.NoError(t, err)
	require.True(t, second.Reused)
	require.Equal(t, first.PublicationID, second.PublicationID)
	require.EqualValues(t, 24, second.FactorRows)
	require.EqualValues(t, 24, second.BucketRows)
	mustQueryRow(t, store, `SELECT COUNT(*) FROM champion_insight_publications`).Scan(&publications)
	require.Equal(t, 1, publications)

	// A corrected snapshot must produce a new content revision even though the
	// match count and maximum game timestamp remain unchanged.
	mustExec(t, store, `UPDATE match_participant_snapshots SET dmg_to_champs=dmg_to_champs+1 WHERE match_id='scheduled' AND minute=15`)
	changed, err := store.rebuildChampionWinFactorRollups(context.Background(), gates)
	require.NoError(t, err)
	require.False(t, changed.Reused)
	require.NotEqual(t, first.Revision, changed.Revision)
	require.NotEqual(t, first.PublicationID, changed.PublicationID)
	mustQueryRow(t, store, `SELECT COUNT(*) FROM champion_insight_publications`).Scan(&publications)
	require.Equal(t, 2, publications)

	// Returning to identical content reactivates the immutable superseded
	// publication instead of colliding with its unique revision.
	mustExec(t, store, `UPDATE match_participant_snapshots SET dmg_to_champs=dmg_to_champs-1 WHERE match_id='scheduled' AND minute=15`)
	reactivated, err := store.rebuildChampionWinFactorRollups(context.Background(), gates)
	require.NoError(t, err)
	require.True(t, reactivated.Reused)
	require.Equal(t, first.PublicationID, reactivated.PublicationID)

	// A bucket containing too few distinct players, or the production gates on
	// this tiny fixture, must fail without replacing the readable publication.
	privateGates := gates
	privateGates.minBucketPlayers = 2
	_, err = store.rebuildChampionWinFactorRollups(context.Background(), privateGates)
	require.ErrorIs(t, err, ErrChampionInsightInsufficientCoverage)
	_, err = store.RebuildChampionWinFactorRollups(context.Background())
	require.ErrorIs(t, err, ErrChampionInsightInsufficientCoverage)
	var publishedID int64
	mustQueryRow(t, store, `SELECT id FROM champion_insight_publications WHERE status='published'`).Scan(&publishedID)
	require.Equal(t, first.PublicationID, publishedID)
	mustQueryRow(t, store, `SELECT COUNT(*) FROM champion_insight_publications`).Scan(&publications)
	require.Equal(t, 2, publications)
}

func TestChampionInsightPrivacyMigrationInvalidatesLegacyPublication(t *testing.T) {
	store := newRankingsTestStore(t)
	source, err := iofs.New(canonicalmigrations.FS, ".")
	require.NoError(t, err)
	migration, err := migrate.NewWithSourceInstance("iofs", source, "pgx5://"+store.dsn[len("postgres://"):])
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = migration.Close() })
	require.NoError(t, migration.Migrate(34))

	var publicationID, cohortID int64
	mustQueryRow(t, store, `
		INSERT INTO champion_insight_publications (
			revision,dataset_key,algorithm_version,feature_schema_version,status,
			source_matches,eligible_matches,source_participants,data_through,published_at
		) VALUES ('legacy','ranked-solo-v1','OBSERVED_PERCENTILES_V1',1,'published',1,1,1,now(),now())
		RETURNING id`).Scan(&publicationID)
	mustQueryRow(t, store, `
		INSERT INTO champion_insight_cohorts (
			publication_id,queue_id,version,region_scope,tier_group,champion_id,
			champion_name,team_position,sample_games,sample_players,availability,model_scope
		) VALUES ($1,420,'16.13','ALL','ALL',1,'Champion1','JUNGLE',1,1,'AVAILABLE','CHAMPION_POSITION')
		RETURNING id`, publicationID).Scan(&cohortID)
	mustExec(t, store, `
		INSERT INTO champion_insight_factors (
			cohort_id,metric_key,kind,start_minute,end_minute,unit,direction,
			p50,p70,p90,evidence_grade,importance_rank
		) VALUES ($1,'JUNGLE_CS_10','TRAINABLE',0,10,'COUNT','HIGHER',1,1,1,'OBSERVED',1)`, cohortID)
	mustExec(t, store, `
		INSERT INTO champion_insight_factor_buckets (
			cohort_id,metric_key,ordinal,lower_bound,upper_bound,games,wins
		) VALUES ($1,'JUNGLE_CS_10',1,1,1,1,1)`, cohortID)

	require.NoError(t, migration.Migrate(35))
	var status string
	var bucketPlayers int64
	mustQueryRow(t, store, `SELECT status FROM champion_insight_publications WHERE id=$1`, publicationID).Scan(&status)
	mustQueryRow(t, store, `SELECT sample_players FROM champion_insight_factor_buckets WHERE cohort_id=$1`, cohortID).Scan(&bucketPlayers)
	require.Equal(t, "superseded", status)
	require.Zero(t, bucketPlayers)
}

func insertInsightMatch(t *testing.T, store *Store, matchID string, admit bool) {
	t.Helper()
	insertValidRankingsMatch(t, store, matchID, 1800, "MASTER", true)
	if !admit {
		mustExec(t, store, `DELETE FROM statistics_match_membership WHERE match_id=$1`, matchID)
	}
	mustExec(t, store, `UPDATE matches SET timeline_status='done' WHERE match_id=$1`, matchID)
	for participantID := 1; participantID <= 10; participantID++ {
		puuid := fmt.Sprintf("%s-puuid-%d", matchID, participantID)
		mustExec(t, store, `INSERT INTO players(puuid,region) VALUES($1,'KR')`, puuid)
		mustExec(t, store, `UPDATE match_participants SET puuid=$1 WHERE match_id=$2 AND participant_id=$3`, puuid, matchID, participantID)
		for _, minute := range []int{10, 15} {
			mustExec(t, store, `
				INSERT INTO match_participant_snapshots(
					match_id,participant_id,minute,jungle_cs,dmg_to_champs
				) VALUES($1,$2,$3,$4,$5)`, matchID, participantID, minute,
				participantID*5+minute, participantID*100+minute*50)
		}
	}
}
