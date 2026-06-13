//go:build integration

package crawler_test

// Full-pipeline integration test with reduced data.
//
// Run with:
//
//	RIOT_API_KEY=<key> go test -tags integration -v -timeout 10m \
//	  ./internal/crawler/ -run TestFullPipelineReduced
//
// The test schema (crawl_inttest) is NOT dropped automatically.
// Inspect the data afterwards, then clean up with:
//
//	go run ./cmd/inttest-cleanup/

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/crafff/gogg/internal/config"
	"github.com/crafff/gogg/internal/crawler"
	"github.com/crafff/gogg/internal/crawler/phase0"
	"github.com/crafff/gogg/internal/crawler/phase1"
	"github.com/crafff/gogg/internal/crawler/phase2"
	"github.com/crafff/gogg/internal/crawler/phase3"
	"github.com/crafff/gogg/internal/crawler/phase35"
	"github.com/crafff/gogg/internal/crawler/phase4"
	"github.com/crafff/gogg/internal/crawler/phase5"
	"github.com/crafff/gogg/internal/crawler/phase55"
	"github.com/crafff/gogg/internal/riotapi"
	"github.com/crafff/gogg/internal/storage"
	"github.com/crafff/gogg/internal/storage/testutil"
)

const (
	intTestSchema   = "crawl_inttest"
	intTestRegion   = "KR"
	intTestPlatform = "https://kr.api.riotgames.com"
	intTestRegional = "https://asia.api.riotgames.com"
	keepPlayers = 3
)

func TestFullPipelineReduced(t *testing.T) {
	apiKey := os.Getenv("RIOT_API_KEY")
	if apiKey == "" {
		t.Skip("RIOT_API_KEY not set")
	}

	ctx := context.Background()
	store := testutil.NewPersistentStore(t, intTestSchema)
	riot := riotapi.NewClient(apiKey, intTestPlatform, intTestRegional)

	profile := &config.RunProfile{
		Region:            intTestRegion,
		Mode:              config.ModeIncremental,
		TargetTiers:       []string{"CHALLENGER"},
		RankPrefetchTiers: []string{"CHALLENGER"},
		Queue:             "RANKED_SOLO_5x5",
		Execution:         config.ExecutionPipeline,
	}

	state, err := crawler.NewRunState(ctx, store, nil, profile, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("create run state: %v", err)
	}
	t.Logf("run id=%d  schema=%s", state.ID, intTestSchema)

	// ── Phase 0: version sync ─────────────────────────────────────────────────
	t.Log("── Phase 0: Version Sync")
	if err := phase0.New(riot, store).Run(ctx, state); err != nil {
		t.Fatalf("phase0: %v", err)
	}
	t.Logf("game version: %s", state.Profile.Version)

	// ── Phase 1: rank sync, then prune to keepPlayers ─────────────────────────
	t.Log("── Phase 1: Rank Sync")
	if err := phase1.New(riot, store).Run(ctx, state); err != nil {
		t.Fatalf("phase1: %v", err)
	}
	t.Logf("fetched %d players → keeping %d per tier",
		queryInt(ctx, store, `SELECT COUNT(DISTINCT puuid) FROM player_rank_snapshots WHERE run_id = $1`, state.ID),
		keepPlayers)
	// Keep the first keepPlayers rows per tier (ordered by created_at).
	mustExec(t, ctx, store, `
		DELETE FROM player_rank_snapshots
		WHERE run_id = $1
		  AND source = 'phase1'
		  AND puuid NOT IN (
		      SELECT puuid
		      FROM (
		          SELECT puuid,
		                 ROW_NUMBER() OVER (PARTITION BY tier ORDER BY created_at) AS rn
		          FROM player_rank_snapshots
		          WHERE run_id = $1 AND source = 'phase1'
		      ) ranked
		      WHERE rn <= $2
		  )`, state.ID, keepPlayers)

	// ── Phase 2: collect match IDs (full run) ────────────────────────────────
	t.Log("── Phase 2: Match ID Collection")
	if err := phase2.New(riot, store).Run(ctx, state); err != nil {
		t.Fatalf("phase2: %v", err)
	}
	t.Logf("collected %d pending matches → keeping %d",
		queryInt(ctx, store, `SELECT COUNT(*) FROM matches WHERE region = $1 AND fetch_status = 'pending'`, intTestRegion),
		keepPlayers*3)

	// Prune to keepPlayers*3 pending matches (≈3 per player).
	mustExec(t, ctx, store, `
		DELETE FROM matches
		WHERE region = $1
		  AND fetch_status = 'pending'
		  AND match_id NOT IN (
		      SELECT match_id FROM matches
		      WHERE region = $1 AND fetch_status = 'pending'
		      ORDER BY created_at
		      LIMIT $2
		  )`, intTestRegion, keepPlayers*3)

	// ── Phase 3: fetch match details ──────────────────────────────────────────
	t.Log("── Phase 3: Match Detail Fetch")
	if err := phase3.New(riot, store).Run(ctx, state); err != nil {
		t.Fatalf("phase3: %v", err)
	}
	t.Logf("match details: done=%d  error=%d  participants=%d",
		queryInt(ctx, store, `SELECT COUNT(*) FROM matches WHERE region=$1 AND fetch_status='done'`, intTestRegion),
		queryInt(ctx, store, `SELECT COUNT(*) FROM matches WHERE region=$1 AND fetch_status='error'`, intTestRegion),
		queryInt(ctx, store, `SELECT COUNT(*) FROM match_participants`),
	)

	// ── Phase 3.5: on-demand rank backfill ────────────────────────────────────
	t.Log("── Phase 3.5: On-Demand Rank")
	if err := phase35.New(riot, store).Run(ctx, state); err != nil {
		t.Fatalf("phase35: %v", err)
	}
	t.Logf("tier backfill: filled=%d  missing=%d",
		queryInt(ctx, store, `SELECT COUNT(*) FROM match_participants WHERE tier_at_match IS NOT NULL`),
		queryInt(ctx, store, `SELECT COUNT(*) FROM match_participants WHERE tier_at_match IS NULL`),
	)

	// ── Phase 4: avg tier score ────────────────────────────────────────────────
	t.Log("── Phase 4: Avg Tier Calc")
	if err := phase4.New(store).Run(ctx, state); err != nil {
		t.Fatalf("phase4: %v", err)
	}
	t.Logf("scored matches: %d",
		queryInt(ctx, store, `SELECT COUNT(*) FROM matches WHERE region=$1 AND avg_tier_score IS NOT NULL`, intTestRegion))

	// ── Phase 5: timeline fetch ───────────────────────────────────────────────
	t.Log("── Phase 5: Timeline")
	if err := phase5.New(riot, store).Run(ctx, state); err != nil {
		t.Fatalf("phase5: %v", err)
	}
	t.Logf("timeline: done=%d  item_events=%d  snapshots=%d",
		queryInt(ctx, store, `SELECT COUNT(*) FROM matches WHERE region=$1 AND timeline_status='done'`, intTestRegion),
		queryInt(ctx, store, `SELECT COUNT(*) FROM match_item_events`),
		queryInt(ctx, store, `SELECT COUNT(*) FROM match_participant_snapshots`),
	)

	// ── Phase 5.5: item classification ───────────────────────────────────────
	t.Log("── Phase 5.5: Item Classification")
	if err := phase55.New(store).Run(ctx, state); err != nil {
		t.Fatalf("phase55: %v", err)
	}
	t.Logf("items: done=%d  completed_items=%d",
		queryInt(ctx, store, `SELECT COUNT(*) FROM matches WHERE region=$1 AND items_status='done'`, intTestRegion),
		queryInt(ctx, store, `SELECT COUNT(*) FROM match_completed_items`),
	)

	if err := state.Complete(ctx); err != nil {
		t.Logf("warn: complete run: %v", err)
	}

	// ── Final summary ──────────────────────────────────────────────────────────
	t.Logf("── Summary (schema=%q  run_id=%d) ──", intTestSchema, state.ID)
	for _, tbl := range []string{
		"game_versions", "players", "player_rank_snapshots",
		"matches", "match_participants", "match_perks", "match_teams", "match_bans",
		"match_item_events", "match_skill_events", "match_participant_snapshots",
		"item_catalog", "match_completed_items", "match_starter_items",
	} {
		t.Logf("  %-28s %d rows", tbl, queryInt(ctx, store, "SELECT COUNT(*) FROM "+tbl))
	}
	t.Logf("")
	t.Logf("Inspect: SET search_path TO %s; SELECT * FROM matches;", intTestSchema)
	t.Logf("Cleanup: go run ./cmd/inttest-cleanup/")
}

func queryInt(ctx context.Context, store *storage.Store, sql string, args ...any) int {
	var n int
	store.Pool.QueryRow(ctx, sql, args...).Scan(&n)
	return n
}

func mustExec(t *testing.T, ctx context.Context, store *storage.Store, sql string, args ...any) {
	t.Helper()
	if _, err := store.Pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("exec: %v\nsql: %s", err, sql)
	}
}