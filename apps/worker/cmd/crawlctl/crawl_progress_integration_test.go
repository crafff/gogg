package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

func TestTFTRunProgressUsesUniqueRunMatchesAndSeparatesGlobalQueue(t *testing.T) {
	dsn := os.Getenv("GOGG_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set GOGG_TEST_DATABASE_DSN to run the PostgreSQL progress test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	q := sqlcgen.New(tx)
	unique := fmt.Sprintf("progress-%d", time.Now().UnixNano())
	run, err := q.CreateTFTRun(ctx, sqlcgen.CreateTFTRunParams{
		WorkflowID: unique, WorkflowRunID: unique + "-run", ScheduleID: unique,
		ProfileName: unique, Platform: unique, RoutingRegion: "GLOBAL", QueueType: "RANKED_TFT",
		QueueID: 1100, Status: "running", WindowStart: pgTimestamp(time.Now().Add(-time.Hour)),
		WindowEnd: pgTimestamp(time.Now()), Config: []byte(`{"platforms":["NA1","KR","EUW1","SG2"]}`),
	})
	require.NoError(t, err)
	baselineRoutes, err := q.ListTFTRunRouteProgress(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, baselineRoutes, 4)
	baselineAmericasPending := baselineRoutes[0].GlobalPendingMatches
	otherRun, err := q.CreateTFTRun(ctx, sqlcgen.CreateTFTRunParams{
		WorkflowID: unique + "-other", WorkflowRunID: unique + "-other-run", ScheduleID: unique,
		ProfileName: unique + "-other", Platform: unique + "-other", RoutingRegion: "GLOBAL", QueueType: "RANKED_TFT",
		QueueID: 1100, Status: "running", WindowStart: pgTimestamp(time.Now().Add(-time.Hour)),
		WindowEnd: pgTimestamp(time.Now()), Config: []byte(`{"platforms":["NA1"]}`),
	})
	require.NoError(t, err)

	_, err = tx.Exec(ctx, `
		INSERT INTO tft_seed_snapshots (run_id, platform, queue_type, cohort, tier, puuid, selected)
		VALUES
			($1, 'NA1', 'RANKED_TFT', 'MASTER_PLUS', 'MASTER', 'na-seed-1', true),
			($1, 'NA1', 'RANKED_TFT', 'MASTER_PLUS', 'MASTER', 'na-seed-2', true),
			($1, 'EUW1', 'RANKED_TFT', 'MASTER_PLUS', 'MASTER', 'euw-seed-1', true)`, run.ID)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `
		INSERT INTO tft_crawl_checkpoints (run_id, stage, scope_key, cursor, processed)
		VALUES
			($1, 'match_discovery', 'NA1', '{}', 2),
			($1, 'match_discovery', 'KR', '{}', 0),
			($1, 'match_discovery', 'EUW1', '{}', 0)`, run.ID)
	require.NoError(t, err)

	discoveries := []struct{ route, match, seed string }{
		{"AMERICAS", unique + "-a", "seed-1"},
		{"AMERICAS", unique + "-a", "seed-2"}, // duplicate match from another seed
		{"AMERICAS", unique + "-b", "seed-1"},
		{"ASIA", unique + "-c", "seed-1"},
		{"ASIA", unique + "-d", "seed-1"},
		{"ASIA", unique + "-e", "seed-1"},
		{"EUROPE", unique + "-f", "seed-1"}, // deliberately not enqueued
		{"AMERICAS", unique + "-shared", "seed-3"},
		{"SEA", unique + "-shared", "seed-4"}, // route is part of match identity
	}
	for _, discovery := range discoveries {
		_, err = tx.Exec(ctx, `
			INSERT INTO tft_match_discoveries (run_id, platform, routing_region, seed_puuid, match_id, cohort)
			VALUES ($1, 'TEST', $2, $3, $4, 'MASTER_PLUS')`, run.ID, discovery.route, discovery.seed, discovery.match)
		require.NoError(t, err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO tft_match_discoveries (run_id, platform, routing_region, seed_puuid, match_id, cohort)
		VALUES ($1, 'TEST', 'AMERICAS', 'other-seed', $2, 'MASTER_PLUS')`, otherRun.ID, unique+"-other-match")
	require.NoError(t, err)
	jobs := []struct{ route, match, status string }{
		{"AMERICAS", unique + "-a", "completed"},
		{"AMERICAS", unique + "-b", "pending"},
		{"ASIA", unique + "-c", "terminal"},
		{"ASIA", unique + "-d", "retry"},
		{"ASIA", unique + "-e", "leased"},
		{"AMERICAS", unique + "-shared", "completed"},
		{"SEA", unique + "-shared", "pending"},
		{"AMERICAS", unique + "-other-match", "pending"},
	}
	for _, job := range jobs {
		_, err = tx.Exec(ctx, `
			INSERT INTO tft_match_jobs (routing_region, match_id, platform, status, completed_at)
			VALUES ($1, $2, 'TEST', $3, CASE WHEN $3 IN ('completed','terminal') THEN now() ELSE NULL END)`, job.route, job.match, job.status)
		require.NoError(t, err)
	}

	progress, err := q.GetTFTRunProgress(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), progress.DiscoveredSeeds)
	require.Equal(t, int64(3), progress.SelectedSeeds)
	require.Equal(t, int64(2), progress.CompletedPlatforms, "NA1 and the zero-seed KR platform are complete; EUW1 and SG2 are not")

	routes, err := q.ListTFTRunRouteProgress(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, routes, 4)
	totals := aggregateRouteProgress(routes)
	require.Equal(t, int64(8), totals.DiscoveredMatches)
	require.Equal(t, int64(2), totals.CompletedMatches)
	require.Equal(t, int64(1), totals.TerminalMatches)
	require.Equal(t, int64(2), totals.PendingMatches)
	require.Equal(t, int64(1), totals.RetryMatches)
	require.Equal(t, int64(1), totals.LeasedMatches)
	require.Equal(t, int64(1), totals.NotEnqueuedMatches)
	require.Equal(t, totals.DiscoveredMatches, totals.CompletedMatches+totals.TerminalMatches+totals.PendingMatches+totals.RetryMatches+totals.LeasedMatches+totals.NotEnqueuedMatches)
	require.Equal(t, "AMERICAS", routes[0].RoutingRegion)
	require.Equal(t, int64(3), routes[0].DiscoveredMatches)
	require.Equal(t, baselineAmericasPending+2, routes[0].GlobalPendingMatches, "global queue includes another run's job, run progress does not")
	require.Equal(t, "EUROPE", routes[2].RoutingRegion)
	require.Equal(t, int64(1), routes[2].NotEnqueuedMatches)

	require.NoError(t, q.RefreshTFTRunCounts(ctx, run.ID))
	refreshed, err := q.GetTFTRunByID(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, int64(8), refreshed.DiscoveredMatches)
	require.Equal(t, int64(2), refreshed.CompletedMatches)
	require.Equal(t, int64(1), refreshed.TerminalMatches)
}

func pgTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}
