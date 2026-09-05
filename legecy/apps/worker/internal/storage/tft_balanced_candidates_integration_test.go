//go:build integration

package storage

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

func TestTFTBalancedCandidateSelectionIsDeterministicAndIdempotent(t *testing.T) {
	store := newRankingsTestStore(t)
	ctx := context.Background()
	q := sqlcgen.New(store.Pool)
	unique := fmt.Sprintf("balanced-%d", time.Now().UnixNano())
	run, err := q.CreateTFTRun(ctx, sqlcgen.CreateTFTRunParams{
		WorkflowID: unique, WorkflowRunID: unique + "-run", ScheduleID: unique,
		ProfileName: unique, Platform: "GLOBAL", RoutingRegion: "GLOBAL",
		QueueType: "RANKED_TFT", QueueID: 1100, Status: "running",
		WindowStart: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
		WindowEnd:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Config:      []byte(`{"match_target_per_region":3,"match_selection_revision":"test-v1"}`),
	})
	require.NoError(t, err)
	require.NoError(t, q.EnsureTFTRunMatchSampling(ctx, run.ID, 3, "test-v1"))
	windowStart := time.Now().Add(-time.Hour)
	windowEnd := time.Now()
	staged, err := q.UpsertTFTRunPlayerMatchSyncIfOpen(ctx, sqlcgen.UpsertTFTRunPlayerMatchSyncIfOpenParams{
		RunID: run.ID, Platform: "NA1", Puuid: unique + "-seed", QueueType: "RANKED_TFT",
		WindowStart:  pgtype.Timestamptz{Time: windowStart, Valid: true},
		WindowEnd:    pgtype.Timestamptz{Time: windowEnd, Valid: true},
		LastSyncedAt: pgtype.Timestamptz{Time: windowEnd, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), staged)
	_, err = q.GetTFTPlayerMatchSync(ctx, "NA1", unique+"-seed", "RANKED_TFT")
	require.ErrorIs(t, err, pgx.ErrNoRows, "an open candidate run must not advance the global watermark")

	candidates := map[string][]string{
		"AMERICAS": {"a-4", "a-1", "a-3", "a-2"},
		"ASIA":     {"b-2", "b-1"},
		"EUROPE":   {"c-5", "c-1", "c-4", "c-2", "c-3"},
		"SEA":      {"d-3", "d-1", "d-4", "d-2"},
	}
	for route, matches := range candidates {
		for _, matchID := range matches {
			_, err := q.InsertTFTRunMatchCandidateSourceIfOpen(ctx, sqlcgen.InsertTFTRunMatchCandidateSourceIfOpenParams{
				RunID: run.ID, RoutingRegion: route, MatchID: unique + "-" + matchID,
				Platform: route + "-PLATFORM", SeedPuuid: "seed-1", Cohort: "MASTER_PLUS", SelectionKey: matchID,
			})
			require.NoError(t, err)
		}
	}
	// One selected lobby can retain multiple cohort provenance edges while
	// consuming only one route slot.
	_, err = q.InsertTFTRunMatchCandidateSourceIfOpen(ctx, sqlcgen.InsertTFTRunMatchCandidateSourceIfOpenParams{
		RunID: run.ID, RoutingRegion: "AMERICAS", MatchID: unique + "-a-1",
		Platform: "AMERICAS-PLATFORM", SeedPuuid: "seed-2", Cohort: "DIAMOND", SelectionKey: "a-1",
	})
	require.NoError(t, err)

	tx, err := store.Pool.Begin(ctx)
	require.NoError(t, err)
	txQueries := q.WithTx(tx)
	state, err := txQueries.LockTFTRunMatchSampling(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, "open", state.Phase)
	require.NoError(t, txQueries.SelectBalancedTFTRunMatchCandidates(ctx, int64(state.TargetPerRegion), run.ID))
	projected, err := txQueries.ProjectBalancedTFTMatchDiscoveries(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, int64(12), projected, "11 matches plus the second AMERICAS cohort source")
	enqueued, err := txQueries.EnqueueBalancedTFTMatchJobs(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, int64(11), enqueued)
	committedSync, err := txQueries.CommitTFTRunPlayerMatchSync(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), committedSync)
	changed, err := txQueries.MarkTFTRunMatchSamplingFinalized(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), changed)
	require.NoError(t, tx.Commit(ctx))
	first := selectedCandidateIDs(t, store, run.ID)
	require.Equal(t, []string{
		unique + "-a-1", unique + "-a-2", unique + "-a-3",
		unique + "-b-1", unique + "-b-2",
		unique + "-c-1", unique + "-c-2", unique + "-c-3",
		unique + "-d-1", unique + "-d-2", unique + "-d-3",
	}, first)
	globalSync, err := q.GetTFTPlayerMatchSync(ctx, "NA1", unique+"-seed", "RANKED_TFT")
	require.NoError(t, err)
	require.Equal(t, windowEnd.Unix(), globalSync.WindowEnd.Time.Unix())
	lateWindowEnd := windowEnd.Add(time.Hour)
	staged, err = q.UpsertTFTRunPlayerMatchSyncIfOpen(ctx, sqlcgen.UpsertTFTRunPlayerMatchSyncIfOpenParams{
		RunID: run.ID, Platform: "NA1", Puuid: unique + "-seed", QueueType: "RANKED_TFT",
		WindowStart:  pgtype.Timestamptz{Time: windowStart, Valid: true},
		WindowEnd:    pgtype.Timestamptz{Time: lateWindowEnd, Valid: true},
		LastSyncedAt: pgtype.Timestamptz{Time: lateWindowEnd, Valid: true},
	})
	require.NoError(t, err)
	require.Zero(t, staged, "a finalized run cannot advance its staged watermark")
	globalSync, err = q.GetTFTPlayerMatchSync(ctx, "NA1", unique+"-seed", "RANKED_TFT")
	require.NoError(t, err)
	require.Equal(t, windowEnd.Unix(), globalSync.WindowEnd.Time.Unix())
	newerWindowEnd := windowEnd.Add(2 * time.Hour)
	newerMatchID := unique + "-newer"
	require.NoError(t, q.UpsertTFTPlayerMatchSync(ctx, sqlcgen.UpsertTFTPlayerMatchSyncParams{
		Platform: "NA1", Puuid: unique + "-seed", QueueType: "RANKED_TFT",
		WindowStart:  pgtype.Timestamptz{Time: windowStart, Valid: true},
		WindowEnd:    pgtype.Timestamptz{Time: newerWindowEnd, Valid: true},
		LastSyncedAt: pgtype.Timestamptz{Time: newerWindowEnd, Valid: true}, LastMatchID: &newerMatchID,
	}))
	_, err = q.CommitTFTRunPlayerMatchSync(ctx, run.ID)
	require.NoError(t, err)
	globalSync, err = q.GetTFTPlayerMatchSync(ctx, "NA1", unique+"-seed", "RANKED_TFT")
	require.NoError(t, err)
	require.Equal(t, newerWindowEnd.Unix(), globalSync.WindowEnd.Time.Unix(), "an older run finalizing late cannot move the global window backward")
	require.Equal(t, newerMatchID, *globalSync.LastMatchID, "diagnostic metadata must stay aligned with the newest global window")

	// A candidate with a lower key arriving after finalization is rejected and
	// cannot replace or augment the frozen sample.
	inserted, err := q.InsertTFTRunMatchCandidateSourceIfOpen(ctx, sqlcgen.InsertTFTRunMatchCandidateSourceIfOpenParams{
		RunID: run.ID, RoutingRegion: "AMERICAS", MatchID: unique + "-a-late",
		Platform: "AMERICAS-PLATFORM", SeedPuuid: "seed-late", Cohort: "MASTER_PLUS", SelectionKey: "0",
	})
	require.NoError(t, err)
	require.Zero(t, inserted)
	require.Equal(t, first, selectedCandidateIDs(t, store, run.ID))

	// Retrying finalization observes the sealed state and only replays
	// idempotent projection/enqueue operations.
	state, err = q.GetTFTRunMatchSampling(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, "finalized", state.Phase)
	projected, err = q.ProjectBalancedTFTMatchDiscoveries(ctx, run.ID)
	require.NoError(t, err)
	require.Zero(t, projected)
	enqueued, err = q.EnqueueBalancedTFTMatchJobs(ctx, run.ID)
	require.NoError(t, err)
	require.Zero(t, enqueued)

	progress, err := q.ListTFTRunCandidateRouteProgress(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, int64(4), progress[0].CandidateMatches)
	require.Equal(t, int64(3), progress[0].SelectedMatches)
	require.Equal(t, int64(2), progress[1].CandidateMatches)
	require.Equal(t, int64(2), progress[1].SelectedMatches, "a short route reports its natural shortfall")

	var unselectedDiscoveries int
	mustQueryRow(t, store, `
		SELECT count(*)
		FROM tft_match_discoveries
		WHERE run_id = $1 AND match_id IN ($2, $3, $4)`,
		run.ID, unique+"-a-4", unique+"-c-4", unique+"-d-4").Scan(&unselectedDiscoveries)
	require.Zero(t, unselectedDiscoveries)
}

func selectedCandidateIDs(t *testing.T, store *Store, runID int64) []string {
	t.Helper()
	rows, err := store.Pool.Query(context.Background(), `
		SELECT match_id
		FROM tft_run_match_candidates
		WHERE run_id = $1 AND selected`, runID)
	require.NoError(t, err)
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())
	sort.Strings(ids)
	return ids
}
