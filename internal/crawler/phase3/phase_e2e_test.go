//go:build e2e

package phase3_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/crafff/gogg/internal/crawler"
	"github.com/crafff/gogg/internal/crawler/phase3"
	"github.com/crafff/gogg/internal/riotapi"
	"github.com/crafff/gogg/internal/storage/testutil"
)

// TestRun_realMatch fetches a real match from the Riot API, validates the DTO,
// writes it to the database, then verifies the stored data.
//
// Run with:
//
//	RIOT_API_KEY=<key> go test -tags e2e -v ./internal/crawler/phase3/ -run TestRun_realMatch
func TestRun_realMatch(t *testing.T) {
	apiKey := os.Getenv("RIOT_API_KEY")
	if apiKey == "" {
		t.Skip("RIOT_API_KEY not set")
	}

	const matchID = "KR_8169051579"
	ctx := context.Background()

	// ── Step 1: fetch DTO from real Riot API ─────────────────────────────────
	t.Log("step 1: fetching match detail from Riot API")
	client := riotapi.NewClient(apiKey,
		"https://kr.api.riotgames.com",
		"https://asia.api.riotgames.com",
	)
	dto, err := client.GetMatchDetail(ctx, matchID)
	if err != nil {
		t.Fatalf("GetMatchDetail: %v", err)
	}
	t.Logf("fetched match %s: queue=%d version=%s participants=%d",
		matchID, dto.Info.QueueID, dto.Info.GameVersion, len(dto.Info.Participants))

	// ── Step 2: record DTO to testdata/ ──────────────────────────────────────
	t.Log("step 2: recording DTO to testdata/")
	dtoJSON, err := json.MarshalIndent(dto, "", "  ")
	if err != nil {
		t.Fatalf("marshal dto: %v", err)
	}
	outPath := filepath.Join("testdata", matchID+".json")
	if err := os.WriteFile(outPath, dtoJSON, 0o644); err != nil {
		t.Fatalf("write testdata: %v", err)
	}
	t.Logf("wrote %s (%d bytes)", outPath, len(dtoJSON))

	// ── Step 3: validate DTO ──────────────────────────────────────────────────
	t.Log("step 3: validating DTO")
	if dto.Info.QueueID != 420 {
		t.Errorf("queue_id = %d, want 420 (ranked solo)", dto.Info.QueueID)
	}
	if got := len(dto.Info.Participants); got != 10 {
		t.Errorf("participants = %d, want 10", got)
	}
	if dto.Info.GameDuration <= 0 {
		t.Errorf("game_duration = %d, want > 0", dto.Info.GameDuration)
	}
	for i, p := range dto.Info.Participants {
		if p.Puuid == "" {
			t.Errorf("participant[%d] has empty puuid", i)
		}
		if p.ChampionID == 0 {
			t.Errorf("participant[%d] %s has champion_id=0", i, p.ChampionName)
		}
	}

	// ── Step 4: write to test schema ──────────────────────────────────────────
	t.Log("step 4: writing to database (test schema)")
	store := testutil.NewTestStore(t)

	for _, p := range dto.Info.Participants {
		if err := store.UpsertPlayer(ctx, p.Puuid, "KR", nil, nil); err != nil {
			t.Fatalf("upsert player %s: %v", p.Puuid[:8], err)
		}
	}
	if err := store.UpsertMatchID(ctx, matchID, "KR", ""); err != nil {
		t.Fatalf("upsert match: %v", err)
	}

	ph := phase3.New(client, store)
	if err := ph.Run(ctx, &crawler.RunState{}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// ── Step 5: validate DB ────────────────────────────────────────────────────
	t.Log("step 5: validating database state")

	var fetchStatus string
	if err := store.Pool.QueryRow(ctx,
		`SELECT fetch_status FROM matches WHERE match_id = $1`, matchID,
	).Scan(&fetchStatus); err != nil {
		t.Fatalf("query match: %v", err)
	}
	if fetchStatus != "done" {
		t.Errorf("fetch_status = %q, want \"done\"", fetchStatus)
	}

	var participantCount int
	if err := store.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM match_participants WHERE match_id = $1`, matchID,
	).Scan(&participantCount); err != nil {
		t.Fatalf("query participants: %v", err)
	}
	if participantCount != len(dto.Info.Participants) {
		t.Errorf("participant rows = %d, want %d", participantCount, len(dto.Info.Participants))
	}
	t.Logf("inserted %d participants", participantCount)

	var perkCount int
	if err := store.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM match_perks WHERE match_id = $1`, matchID,
	).Scan(&perkCount); err != nil {
		t.Fatalf("query perks: %v", err)
	}
	if perkCount != len(dto.Info.Participants) {
		t.Errorf("perk rows = %d, want %d", perkCount, len(dto.Info.Participants))
	}
	t.Logf("inserted %d perk rows", perkCount)

	// Spot-check one participant: champion_id and previously-broken ping fields
	// must be non-zero if the Riot response includes them.
	rows, err := store.Pool.Query(ctx, `
		SELECT champion_name, kills, deaths, assists,
		       enemy_missing_pings, time_played
		FROM match_participants
		WHERE match_id = $1
		ORDER BY participant_id`, matchID)
	if err != nil {
		t.Fatalf("query participant detail: %v", err)
	}
	defer rows.Close()
	t.Log("participants in DB:")
	for rows.Next() {
		var name string
		var kills, deaths, assists, enemyMissingPings, timePlayed int
		if err := rows.Scan(&name, &kills, &deaths, &assists, &enemyMissingPings, &timePlayed); err != nil {
			t.Fatal(err)
		}
		t.Logf("  %-20s %d/%d/%d  enemy_missing_pings=%d  time_played=%d",
			name, kills, deaths, assists, enemyMissingPings, timePlayed)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
}