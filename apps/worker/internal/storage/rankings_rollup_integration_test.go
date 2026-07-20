//go:build integration

package storage

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
	canonicalmigrations "github.com/crafff/gogg/packages/sqlc/migrations"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
)

type rollupParticipant struct {
	participantID int
	teamID        int
	position      string
	win           *bool
	championID    *int
	championName  *string
	kills         *int
	deaths        *int
	assists       *int
}

func TestRebuildRankingsRollupsStrictEligibility(t *testing.T) {
	store := newRankingsTestStore(t)
	ctx := context.Background()

	insertValidRankingsMatch(t, store, "valid-1", 600, "MASTER", true)
	insertValidRankingsMatch(t, store, "valid-2", 1800, "MASTER", false)
	mustExec(t, store, `
		UPDATE match_bans
		SET champion_id = CASE pick_turn WHEN 1 THEN 1 WHEN 2 THEN 2 ELSE -1 END
		WHERE match_id = 'valid-1' AND team_id = 100`)
	mustExec(t, store, `
		UPDATE match_bans SET champion_id = 1
		WHERE match_id = 'valid-2' AND team_id = 100 AND pick_turn = 1`)

	// Not part of source_completed_matches.
	insertValidRankingsMatch(t, store, "pending", 1800, "MASTER", true)
	mustExec(t, store, `UPDATE matches SET fetch_status = 'pending' WHERE match_id = 'pending'`)
	insertValidRankingsMatch(t, store, "other-queue", 1800, "MASTER", true)
	mustExec(t, store, `UPDATE matches SET queue_id = 430 WHERE match_id = 'other-queue'`)

	metadataCases := []struct {
		id  string
		sql string
	}{
		{"metadata-version", `UPDATE matches SET version = '' WHERE match_id = 'metadata-version'`},
		{"metadata-region", `UPDATE matches SET region = '' WHERE match_id = 'metadata-region'`},
		// Also invalid duration: metadata must win the classification order.
		{"metadata-precedence", `UPDATE matches SET version = '', game_duration = 1 WHERE match_id = 'metadata-precedence'`},
	}
	for _, tc := range metadataCases {
		insertValidRankingsMatch(t, store, tc.id, 1800, "MASTER", true)
		mustExec(t, store, tc.sql)
	}

	tierCases := []struct {
		id  string
		sql string
	}{
		{"tier-null", `UPDATE matches SET avg_tier = NULL WHERE match_id = 'tier-null'`},
		{"tier-blank", `UPDATE matches SET avg_tier = '' WHERE match_id = 'tier-blank'`},
		{"tier-unknown", `UPDATE matches SET avg_tier = 'UNKNOWN' WHERE match_id = 'tier-unknown'`},
	}
	for _, tc := range tierCases {
		insertValidRankingsMatch(t, store, tc.id, 1800, "MASTER", true)
		mustExec(t, store, tc.sql)
	}

	insertValidRankingsMatch(t, store, "duration-599", 599, "MASTER", true)
	insertValidRankingsMatch(t, store, "duration-null", 1800, "MASTER", true)
	mustExec(t, store, `UPDATE matches SET game_duration = NULL WHERE match_id = 'duration-null'`)

	shapeCases := []struct {
		id  string
		sql string
	}{
		{"shape-nine", `DELETE FROM match_participants WHERE match_id = 'shape-nine' AND participant_id = 10`},
		{"shape-duplicate-id", `UPDATE match_participants SET participant_id = 1 WHERE match_id = 'shape-duplicate-id' AND participant_id = 2`},
		{"shape-team", `UPDATE match_participants SET team_id = 100 WHERE match_id = 'shape-team' AND participant_id = 6`},
		{"shape-position", `UPDATE match_participants SET team_position = '' WHERE match_id = 'shape-position' AND participant_id = 1`},
		{"shape-team-position", `UPDATE match_participants SET team_position = 'TOP' WHERE match_id = 'shape-team-position' AND participant_id = 2`},
	}
	for _, tc := range shapeCases {
		insertValidRankingsMatch(t, store, tc.id, 1800, "MASTER", true)
		mustExec(t, store, tc.sql)
	}
	insertValidRankingsMatch(t, store, "shape-eleven", 1800, "MASTER", true)
	extra := validRollupParticipants(true)[0]
	extra.participantID = 11
	extra.championID = intPointer(99)
	extra.championName = stringPointer("Champion99")
	insertRankingsParticipant(t, store, "shape-eleven", extra)

	factCases := []struct {
		id  string
		sql string
	}{
		{"facts-zero-champion", `UPDATE match_participants SET champion_id = 0 WHERE match_id = 'facts-zero-champion' AND participant_id = 1`},
		{"facts-duplicate-champion", `UPDATE match_participants SET champion_id = 1 WHERE match_id = 'facts-duplicate-champion' AND participant_id = 2`},
		{"facts-name", `UPDATE match_participants SET champion_name = '' WHERE match_id = 'facts-name' AND participant_id = 1`},
		{"facts-win-null", `UPDATE match_participants SET win = NULL WHERE match_id = 'facts-win-null' AND participant_id = 1`},
		{"facts-kills", `UPDATE match_participants SET kills = -1 WHERE match_id = 'facts-kills' AND participant_id = 1`},
		{"facts-deaths", `UPDATE match_participants SET deaths = NULL WHERE match_id = 'facts-deaths' AND participant_id = 1`},
		{"facts-assists", `UPDATE match_participants SET assists = -1 WHERE match_id = 'facts-assists' AND participant_id = 1`},
		{"facts-winners", `UPDATE match_participants SET win = true WHERE match_id = 'facts-winners' AND participant_id = 6`},
	}
	for _, tc := range factCases {
		insertValidRankingsMatch(t, store, tc.id, 1800, "MASTER", true)
		mustExec(t, store, tc.sql)
	}

	banShapeCases := []struct {
		id  string
		sql string
	}{
		{"ban-shape-nine", `DELETE FROM match_bans WHERE match_id = 'ban-shape-nine' AND team_id = 200 AND pick_turn = 10`},
		{"ban-shape-team", `UPDATE match_bans SET team_id = 300 WHERE match_id = 'ban-shape-team' AND team_id = 200 AND pick_turn = 10`},
		{"ban-shape-turn", `UPDATE match_bans SET pick_turn = 11 WHERE match_id = 'ban-shape-turn' AND team_id = 200 AND pick_turn = 10`},
		{"ban-shape-null", `UPDATE match_bans SET champion_id = NULL WHERE match_id = 'ban-shape-null' AND team_id = 200 AND pick_turn = 10`},
	}
	for _, tc := range banShapeCases {
		insertValidRankingsMatch(t, store, tc.id, 1800, "MASTER", true)
		mustExec(t, store, tc.sql)
	}

	result, err := store.RebuildRankingsRollups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.SourceCompletedMatches != 28 || result.EligibleMatches != 2 {
		t.Fatalf("source=%d eligible=%d", result.SourceCompletedMatches, result.EligibleMatches)
	}
	if result.ExcludedMetadataMatches != 3 || result.ExcludedTierMatches != 3 ||
		result.ExcludedDurationMatches != 2 || result.ExcludedParticipantShapeMatches != 6 ||
		result.ExcludedParticipantFactsMatches != 8 || result.ExcludedBanShapeMatches != 4 {
		t.Fatalf("unexpected exclusions: %+v", result)
	}

	var matchCount int64
	mustQueryRow(t, store, `SELECT total_matches FROM rankings_match_count_rollup`).Scan(&matchCount)
	if matchCount != 2 {
		t.Fatalf("total_matches=%d", matchCount)
	}

	var games, wins int64
	var kdaSum float64
	mustQueryRow(t, store, `
		SELECT games, wins, kda_contribution_sum
		FROM rankings_champion_position_rollup
		WHERE champion_id = 1`).Scan(&games, &wins, &kdaSum)
	if games != 2 || wins != 1 || kdaSum != 4 {
		t.Fatalf("champion aggregate games=%d wins=%d kda_sum=%v", games, wins, kdaSum)
	}

	var totalGames, totalWins, topGames int64
	mustQueryRow(t, store, `SELECT SUM(games), SUM(wins) FROM rankings_champion_position_rollup`).Scan(&totalGames, &totalWins)
	mustQueryRow(t, store, `SELECT SUM(games) FROM rankings_champion_position_rollup WHERE team_position = 'TOP'`).Scan(&topGames)
	if totalGames != 20 || totalWins != 10 || topGames != 4 {
		t.Fatalf("games=%d wins=%d top_games=%d", totalGames, totalWins, topGames)
	}

	var championOneBans, championTwoBans int64
	mustQueryRow(t, store, `SELECT ban_matches FROM rankings_champion_ban_rollup WHERE champion_id = 1`).Scan(&championOneBans)
	mustQueryRow(t, store, `SELECT ban_matches FROM rankings_champion_ban_rollup WHERE champion_id = 2`).Scan(&championTwoBans)
	if championOneBans != 2 || championTwoBans != 1 {
		t.Fatalf("ban counts champion1=%d champion2=%d", championOneBans, championTwoBans)
	}

	queries := sqlcgen.New(store.Pool)
	overall, err := queries.ListOverallRankings(ctx, sqlcgen.ListOverallRankingsParams{
		QueueID: 420, VersionFilter: "16.13", RegionFilter: "KR",
		AvgTiers: []string{"MASTER"}, PositionThreshold: 5,
		MinGames: 1, RowLimit: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(overall) != 10 || overall[0].TotalMatches != 2 {
		t.Fatalf("overall rows=%d total_matches=%d", len(overall), overall[0].TotalMatches)
	}
	var overallChampionOne *sqlcgen.ListOverallRankingsRow
	for i := range overall {
		if overall[i].ChampionID == 1 {
			overallChampionOne = &overall[i]
			break
		}
	}
	if overallChampionOne == nil || overallChampionOne.PickRate != 100 ||
		overallChampionOne.BanRate != 100 || overallChampionOne.Kda != 2 {
		t.Fatalf("unexpected overall champion 1: %+v", overallChampionOne)
	}

	byPosition, err := queries.ListRankingsByPosition(ctx, sqlcgen.ListRankingsByPositionParams{
		QueueID: 420, PositionFilter: "TOP", VersionFilter: "16.13",
		RegionFilter: "KR", AvgTiers: []string{"MASTER"}, MinGames: 1, RowLimit: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(byPosition) != 2 || byPosition[0].TotalMatches != 2 {
		t.Fatalf("position rows=%d total_matches=%d", len(byPosition), byPosition[0].TotalMatches)
	}
	for _, row := range byPosition {
		if row.PickRate != 100 {
			t.Fatalf("position champion %d pick_rate=%v", row.ChampionID, row.PickRate)
		}
	}

	// Rebuilding the same source replaces rather than duplicates aggregates.
	second, err := store.RebuildRankingsRollups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if second.EligibleMatches != result.EligibleMatches || second.ChampionPositionRows != result.ChampionPositionRows {
		t.Fatalf("non-idempotent rebuild: first=%+v second=%+v", result, second)
	}
	mustQueryRow(t, store, `SELECT SUM(games) FROM rankings_champion_position_rollup`).Scan(&totalGames)
	if totalGames != 20 {
		t.Fatalf("games after second rebuild=%d", totalGames)
	}

	// A changed source replaces the previous generation with the new one.
	insertValidRankingsMatch(t, store, "valid-3", 1800, "MASTER", true)
	third, err := store.RebuildRankingsRollups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if third.EligibleMatches != 3 {
		t.Fatalf("eligible after source change=%d", third.EligibleMatches)
	}
	mustQueryRow(t, store, `SELECT SUM(games) FROM rankings_champion_position_rollup`).Scan(&totalGames)
	if totalGames != 30 {
		t.Fatalf("games after source change=%d", totalGames)
	}
}

func TestRankingsRollupSchemaConstraints(t *testing.T) {
	store := newRankingsTestStore(t)
	tests := []struct {
		name string
		sql  string
	}{
		{"queue", `INSERT INTO rankings_match_count_rollup VALUES (430, '1.0', 'KR', 'MASTER', 1)`},
		{"tier", `INSERT INTO rankings_match_count_rollup VALUES (420, '1.0', 'KR', 'UNKNOWN', 1)`},
		{"zero matches", `INSERT INTO rankings_match_count_rollup VALUES (420, '1.0', 'KR', 'MASTER', 0)`},
		{"position", `INSERT INTO rankings_champion_position_rollup VALUES (420, '1.0', 'KR', 'MASTER', 1, 'A', '', 1, 0, 0)`},
		{"champion", `INSERT INTO rankings_champion_position_rollup VALUES (420, '1.0', 'KR', 'MASTER', 0, 'A', 'TOP', 1, 0, 0)`},
		{"wins", `INSERT INTO rankings_champion_position_rollup VALUES (420, '1.0', 'KR', 'MASTER', 1, 'A', 'TOP', 1, 2, 0)`},
		{"state reconciliation", `INSERT INTO rankings_rollup_state VALUES (true, now(), NULL, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := store.Pool.Exec(context.Background(), tc.sql); err == nil {
				t.Fatal("expected constraint violation")
			}
		})
	}

	var removedColumns int
	mustQueryRow(t, store, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'rankings_match_count_rollup'
		  AND column_name IN ('position_mode', 'total_participants')`).Scan(&removedColumns)
	if removedColumns != 0 {
		t.Fatalf("obsolete match-count columns=%d", removedColumns)
	}
}

func TestRankingsRollupMigrationDown(t *testing.T) {
	store := newRankingsTestStore(t)
	source, err := iofs.New(canonicalmigrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	migration, err := migrate.NewWithSourceInstance("iofs", source, "pgx5://"+store.dsn[len("postgres://"):])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = migration.Close() })
	if err := migration.Steps(-1); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{
		"rankings_champion_position_rollup",
		"rankings_champion_ban_rollup",
		"rankings_match_count_rollup",
		"rankings_rollup_state",
	} {
		var relation *string
		mustQueryRow(t, store, `SELECT to_regclass($1)::text`, "public."+table).Scan(&relation)
		if relation != nil {
			t.Fatalf("table %s still exists after migration down", table)
		}
	}
}

func newRankingsTestStore(t *testing.T) *Store {
	t.Helper()
	if os.Getenv("GOGG_INTTEST") == "" {
		t.Skip("set GOGG_INTTEST=1 to run PostgreSQL integration tests")
	}
	adminDSN := os.Getenv("GOGG_TEST_DATABASE_DSN")
	if adminDSN == "" {
		adminDSN = "postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	adminConfig, err := pgx.ParseConfig(adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal(err)
	}
	database := fmt.Sprintf("gogg_rollup_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize()); err != nil {
		admin.Close(ctx)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_, _ = admin.Exec(cleanupCtx, "DROP DATABASE "+pgx.Identifier{database}.Sanitize()+" WITH (FORCE)")
		_ = admin.Close(cleanupCtx)
	})
	testURL, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	testURL.Path = "/" + database
	store, err := New(ctx, testURL.String(), 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	if err := store.InitSchema(ctx); err != nil {
		t.Fatal(err)
	}
	return store
}

func insertValidRankingsMatch(t *testing.T, store *Store, matchID string, duration int, tier string, team100Wins bool) {
	t.Helper()
	mustExec(t, store, `
		INSERT INTO matches (
			match_id, queue_id, fetch_status, version, region, avg_tier,
			game_duration, game_end_ts
		) VALUES ($1, 420, 'done', '16.13', 'KR', $2, $3, now())`, matchID, tier, duration)
	for _, participant := range validRollupParticipants(team100Wins) {
		insertRankingsParticipant(t, store, matchID, participant)
	}
	for _, teamID := range []int{100, 200} {
		firstPickTurn := 1
		if teamID == 200 {
			firstPickTurn = 6
		}
		for pickTurn := firstPickTurn; pickTurn < firstPickTurn+5; pickTurn++ {
			mustExec(t, store, `
				INSERT INTO match_bans (match_id, team_id, pick_turn, champion_id)
				VALUES ($1, $2, $3, -1)`, matchID, teamID, pickTurn)
		}
	}
}

func validRollupParticipants(team100Wins bool) []rollupParticipant {
	positions := []string{"TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"}
	participants := make([]rollupParticipant, 0, 10)
	for i := 1; i <= 10; i++ {
		teamID := 100
		win := team100Wins
		if i > 5 {
			teamID = 200
			win = !team100Wins
		}
		championID := i
		championName := fmt.Sprintf("Champion%d", i)
		kills, deaths, assists := i, 1, 1
		participants = append(participants, rollupParticipant{
			participantID: i,
			teamID:        teamID,
			position:      positions[(i-1)%5],
			win:           &win,
			championID:    &championID,
			championName:  &championName,
			kills:         &kills,
			deaths:        &deaths,
			assists:       &assists,
		})
	}
	return participants
}

func insertRankingsParticipant(t *testing.T, store *Store, matchID string, p rollupParticipant) {
	t.Helper()
	mustExec(t, store, `
		INSERT INTO match_participants (
			match_id, participant_id, team_id, team_position, win,
			champion_id, champion_name, kills, deaths, assists
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		matchID, p.participantID, p.teamID, p.position, p.win,
		p.championID, p.championName, p.kills, p.deaths, p.assists)
}

func mustExec(t *testing.T, store *Store, sql string, args ...any) {
	t.Helper()
	if _, err := store.Pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", strings.TrimSpace(sql), err)
	}
}

func mustQueryRow(t *testing.T, store *Store, sql string, args ...any) pgx.Row {
	t.Helper()
	return store.Pool.QueryRow(context.Background(), sql, args...)
}

func intPointer(value int) *int          { return &value }
func stringPointer(value string) *string { return &value }
