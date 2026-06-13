package phase3_test

import (
	"context"
	"testing"

	"github.com/crafff/gogg/internal/config"
	"github.com/crafff/gogg/internal/crawler"
	"github.com/crafff/gogg/internal/crawler/phase3"
	"github.com/crafff/gogg/internal/riotapi"
	"github.com/crafff/gogg/internal/storage/testutil"
)

// fakeRiot implements the matchFetcher interface used by Phase.
type fakeRiot struct {
	detail *riotapi.MatchDetailDTO
	err    error
}

func (f *fakeRiot) GetMatchDetail(_ context.Context, _ string) (*riotapi.MatchDetailDTO, error) {
	return f.detail, f.err
}

// TestRun_insertsParticipants verifies that phase3.Run correctly writes all
// participant fields to the database, including the ping columns that were
// previously missing from the INSERT VALUES clause ($115–$127).
func TestRun_insertsParticipants(t *testing.T) {
	store := testutil.NewTestStore(t)
	ctx := context.Background()

	const matchID = "KR_TEST001"
	const puuid = "test-puuid-0000-0000-0000-000000000001"

	if err := store.UpsertPlayer(ctx, puuid, "KR", nil, nil); err != nil {
		t.Fatalf("seed player: %v", err)
	}
	if err := store.UpsertMatchID(ctx, matchID, "KR", "15.1"); err != nil {
		t.Fatalf("seed match: %v", err)
	}

	dto := &riotapi.MatchDetailDTO{
		Metadata: riotapi.MetadataDTO{DataVersion: "2", MatchID: matchID},
		Info: riotapi.InfoDTO{
			QueueID:            420,
			GameVersion:        "15.1.1",
			GameStartTimestamp: 1700000000000,
			GameEndTimestamp:   1700002000000,
			GameDuration:       2000,
			Participants: []riotapi.ParticipantDTO{
				{
					ParticipantID: 1,
					Puuid:         puuid,
					ChampionID:    99,
					ChampionName:  "Lux",
					Win:           true,
					Kills:         10,
					Deaths:        2,
					Assists:       5,
					// Fields that were missing from VALUES ($115–$127):
					EnemyMissingPings:          3,
					GetBackPings:               1,
					HoldPings:                  2,
					OnMyWayPings:               4,
					NeedVisionPings:            5,
					PushPings:                  6,
					RetreatPings:               7,
					EnemyVisionPings:           8,
					VisionClearedPings:         9,
					GameEndedInEarlySurrender:  false,
					GameEndedInSurrender:       true,
					TeamEarlySurrendered:       false,
					TimePlayed:                 1800,
				},
			},
		},
	}

	p := phase3.New(&fakeRiot{detail: dto}, store)
	state := &crawler.RunState{Profile: &config.RunProfile{Region: "KR", Version: "15.1"}}
	if err := p.Run(ctx, state); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify core fields and the previously-missing ping fields.
	var (
		championID                int
		kills                     int
		enemyMissingPings         int
		retreatPings              int
		gameEndedInSurrender      bool
		timePlayed                int
	)
	err := store.Pool.QueryRow(ctx, `
		SELECT champion_id, kills, enemy_missing_pings, retreat_pings,
		       game_ended_in_surrender, time_played
		FROM match_participants
		WHERE match_id = $1`, matchID,
	).Scan(&championID, &kills, &enemyMissingPings, &retreatPings,
		&gameEndedInSurrender, &timePlayed)
	if err != nil {
		t.Fatalf("query participant: %v", err)
	}

	if championID != 99 {
		t.Errorf("champion_id = %d, want 99", championID)
	}
	if kills != 10 {
		t.Errorf("kills = %d, want 10", kills)
	}
	if enemyMissingPings != 3 {
		t.Errorf("enemy_missing_pings = %d, want 3", enemyMissingPings)
	}
	if retreatPings != 7 {
		t.Errorf("retreat_pings = %d, want 7", retreatPings)
	}
	if !gameEndedInSurrender {
		t.Errorf("game_ended_in_surrender = false, want true")
	}
	if timePlayed != 1800 {
		t.Errorf("time_played = %d, want 1800", timePlayed)
	}

	// Verify match status changed to done.
	var fetchStatus string
	if err := store.Pool.QueryRow(ctx,
		`SELECT fetch_status FROM matches WHERE match_id = $1`, matchID,
	).Scan(&fetchStatus); err != nil {
		t.Fatalf("query match status: %v", err)
	}
	if fetchStatus != "done" {
		t.Errorf("fetch_status = %q, want \"done\"", fetchStatus)
	}

	// Verify perks were inserted.
	var perkCount int
	if err := store.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM match_perks WHERE match_id = $1`, matchID,
	).Scan(&perkCount); err != nil {
		t.Fatalf("query perks: %v", err)
	}
	if perkCount != 1 {
		t.Errorf("perk rows = %d, want 1", perkCount)
	}
}