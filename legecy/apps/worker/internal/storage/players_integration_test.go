//go:build integration

package storage

import (
	"context"
	"testing"
	"time"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
	canonicalmigrations "github.com/crafff/gogg/packages/sqlc/migrations"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func TestMatchIdentityCannotOverwriteCurrentAccountIdentity(t *testing.T) {
	store := newRankingsTestStore(t)
	ctx := context.Background()
	currentName, currentTag := "RuitaoZhou", "123"
	oldName, oldTag := "OldRiotID", "NA1"

	if err := store.UpsertPlayer(ctx, "target-puuid", "NA1", &currentName, &currentTag); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPlayerFromMatch(ctx, "target-puuid", "NA1", &oldName, &oldTag); err != nil {
		t.Fatal(err)
	}

	var gotName, gotTag *string
	if err := store.Pool.QueryRow(ctx, `SELECT game_name, tag_line FROM players WHERE puuid='target-puuid'`).Scan(&gotName, &gotTag); err != nil {
		t.Fatal(err)
	}
	if gotName == nil || *gotName != currentName || gotTag == nil || *gotTag != currentTag {
		t.Fatalf("identity was overwritten by historical match: name=%v tag=%v", gotName, gotTag)
	}
}

func TestMatchIdentityFillsPreviouslyUnknownPlayer(t *testing.T) {
	store := newRankingsTestStore(t)
	ctx := context.Background()
	name, tag := "KnownFromMatch", "NA1"

	if err := store.UpsertPlayerFromMatch(ctx, "new-puuid", "NA1", &name, &tag); err != nil {
		t.Fatal(err)
	}

	var gotName, gotTag *string
	if err := store.Pool.QueryRow(ctx, `SELECT game_name, tag_line FROM players WHERE puuid='new-puuid'`).Scan(&gotName, &gotTag); err != nil {
		t.Fatal(err)
	}
	if gotName == nil || *gotName != name || gotTag == nil || *gotTag != tag {
		t.Fatalf("match identity was not stored: name=%v tag=%v", gotName, gotTag)
	}
}

func TestExpiredLookupCleanupPreservesActiveJobs(t *testing.T) {
	store := newRankingsTestStore(t)
	ctx := context.Background()
	old := time.Now().Add(-8 * 24 * time.Hour)
	for _, status := range []string{"QUEUED", "RUNNING", "COMPLETED", "PARTIAL", "FAILED"} {
		_, err := store.Pool.Exec(ctx, `
			INSERT INTO summoner_lookup_jobs (
				id, region, requested_game_name, requested_tag_line,
				requested_game_name_norm, requested_tag_line_norm,
				status, stage, created_at, updated_at
			) VALUES ($1, 'KR', $2, 'KR1', $3, 'KR1', $4, 'QUEUED', $5, $5)`,
			"job-"+status, "Player"+status, "player"+status, status, old)
		if err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := sqlcgen.New(store.Pool).DeleteExpiredSummonerLookupJobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 3 {
		t.Fatalf("deleted=%d, want 3 terminal jobs", deleted)
	}

	for _, status := range []string{"QUEUED", "RUNNING"} {
		var exists bool
		if err := store.Pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM summoner_lookup_jobs WHERE id=$1)`,
			"job-"+status,
		).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("active %s job was deleted", status)
		}
	}
}

func TestSummonerRankBackfillIsLookupScoped(t *testing.T) {
	store := newRankingsTestStore(t)
	ctx := context.Background()
	queries := sqlcgen.New(store.Pool)

	mustExec(t, store, `INSERT INTO players (puuid) VALUES ('ranked-puuid'), ('unranked-puuid')`)
	mustExec(t, store, `
		INSERT INTO matches (match_id, queue_id, fetch_status, version, region, game_start_ts)
		VALUES
			('lookup-match', 420, 'done', '16.17', 'NA1', now() - interval '2 hours'),
			('other-match', 420, 'done', '16.17', 'NA1', now() - interval '3 hours')`)
	mustExec(t, store, `
		INSERT INTO match_participants (match_id, participant_id, puuid)
		VALUES
			('lookup-match', 1, 'ranked-puuid'),
			('lookup-match', 2, 'unranked-puuid'),
			('other-match', 1, 'ranked-puuid')`)
	mustExec(t, store, `
		INSERT INTO summoner_lookup_jobs (
			id, region, requested_game_name, requested_tag_line,
			requested_game_name_norm, requested_tag_line_norm, status, stage
		) VALUES ('rank-job', 'NA1', 'Ranked', 'NA1', 'ranked', 'NA1', 'RUNNING', 'ENRICH_RANKS')`)
	mustExec(t, store, `
		INSERT INTO summoner_lookup_job_matches (job_id, match_id, ordinal, supported)
		VALUES ('rank-job', 'lookup-match', 0, true)`)

	targets, err := queries.ListSummonerLookupRankTargets(ctx, "rank-job")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 || targets[0] != "ranked-puuid" || targets[1] != "unranked-puuid" {
		t.Fatalf("targets = %#v", targets)
	}

	tier, division, lp, rankedPUUID := "CHALLENGER", "I", int32(1200), "ranked-puuid"
	if err := queries.BackfillSummonerLookupParticipantRank(ctx, sqlcgen.BackfillSummonerLookupParticipantRankParams{
		Tier: &tier, Division: division, LeaguePoints: &lp, JobID: "rank-job", Puuid: &rankedPUUID,
	}); err != nil {
		t.Fatal(err)
	}
	unrankedPUUID := "unranked-puuid"
	if err := queries.MarkSummonerLookupParticipantUnranked(ctx, "rank-job", &unrankedPUUID); err != nil {
		t.Fatal(err)
	}

	var gotTier *string
	if err := store.Pool.QueryRow(ctx, `
		SELECT tier_at_match FROM match_participants
		WHERE match_id='other-match' AND puuid='ranked-puuid'`).Scan(&gotTier); err != nil {
		t.Fatal(err)
	}
	if gotTier != nil {
		t.Fatalf("unrelated match was updated to %q", *gotTier)
	}

	rows, err := store.Pool.Query(ctx, `
		SELECT puuid, tier_at_match, division_at_match, lp_at_match,
		       tier_snapshot_delta_h
		FROM match_participants
		WHERE match_id='lookup-match'
		ORDER BY participant_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type rankRow struct {
		puuid, tier string
		division    *string
		lp, delta   *int32
	}
	var got []rankRow
	for rows.Next() {
		var row rankRow
		if err := rows.Scan(&row.puuid, &row.tier, &row.division, &row.lp, &row.delta); err != nil {
			t.Fatal(err)
		}
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].tier != "CHALLENGER" || got[0].lp == nil || *got[0].lp != 1200 ||
		got[0].delta == nil || got[1].tier != "UNRANKED" || got[1].lp != nil {
		t.Fatalf("lookup ranks = %#v", got)
	}

	thresholds, err := queries.GetSummonerLookupApexThresholds(ctx, "rank-job")
	if err != nil {
		t.Fatal(err)
	}
	if thresholds.ChallengerMinLp != 1200 || thresholds.GrandmasterMinLp != -1 {
		t.Fatalf("thresholds = %#v", thresholds)
	}
}

func TestSummonerRankStageMigrationDownAndUp(t *testing.T) {
	store := newRankingsTestStore(t)
	ctx := context.Background()
	mustExec(t, store, `
		INSERT INTO summoner_lookup_jobs (
			id, region, requested_game_name, requested_tag_line,
			requested_game_name_norm, requested_tag_line_norm, status, stage
		) VALUES ('migration-job', 'KR', 'Migration', 'KR1', 'migration', 'KR1', 'RUNNING', 'ENRICH_RANKS')`)

	source, err := iofs.New(canonicalmigrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	migration, err := migrate.NewWithSourceInstance("iofs", source, "pgx5://"+store.dsn[len("postgres://"):])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = migration.Close() })
	if err := migration.Migrate(22); err != nil {
		t.Fatal(err)
	}

	var stage string
	if err := store.Pool.QueryRow(ctx, `SELECT stage FROM summoner_lookup_jobs WHERE id='migration-job'`).Scan(&stage); err != nil {
		t.Fatal(err)
	}
	if stage != "FETCH_MATCHES" {
		t.Fatalf("stage after down migration = %q", stage)
	}
	if _, err := store.Pool.Exec(ctx, `UPDATE summoner_lookup_jobs SET stage='ENRICH_RANKS' WHERE id='migration-job'`); err == nil {
		t.Fatal("v22 stage constraint unexpectedly accepted ENRICH_RANKS")
	}
	if err := migration.Migrate(23); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(ctx, `UPDATE summoner_lookup_jobs SET stage='ENRICH_RANKS' WHERE id='migration-job'`); err != nil {
		t.Fatalf("v23 stage constraint rejected ENRICH_RANKS: %v", err)
	}
}
