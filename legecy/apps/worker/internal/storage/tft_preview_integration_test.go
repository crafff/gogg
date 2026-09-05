//go:build integration

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

func TestTFTObservedPreviewDeduplicatesDiscoveryEdgesAndFiltersCatalogUnits(t *testing.T) {
	store := newRankingsTestStore(t)
	ctx := context.Background()
	q := sqlcgen.New(store.Pool)
	unique := fmt.Sprintf("preview-%d", time.Now().UnixNano())
	now := time.Now().UTC()
	t.Cleanup(func() {
		_, _ = store.Pool.Exec(context.Background(), `DELETE FROM tft_crawl_runs WHERE workflow_id LIKE $1`, unique+"%")
		_, _ = store.Pool.Exec(context.Background(), `DELETE FROM tft_matches WHERE match_id LIKE $1`, unique+"%")
		_, _ = store.Pool.Exec(context.Background(), `DELETE FROM tft_static_snapshots WHERE revision LIKE $1`, unique+"%")
	})

	run := createCompletedPreviewRun(t, q, unique, now)
	otherRun := createCompletedPreviewRun(t, q, unique+"-other", now)
	snapshotID := insertPreviewCatalog(t, store, unique, now)
	matchIDs := []string{unique + "-match-1", unique + "-match-2", unique + "-match-3"}
	unitIDs := []string{unique + "-A", unique + "-B", unique + "-C", unique + "-D", unique + "-E", unique + "-F"}
	summonID := unique + "-summon"

	for matchIndex, matchID := range matchIDs {
		insertPreviewMatch(t, store, matchID, "", "invalid_game_version", normalPlacements(), append(unitIDs, summonID), now)
		insertPreviewEquipmentAndStars(t, store, matchID, unitIDs[0], summonID, matchIndex+1)
		insertPreviewDiscovery(t, store, run.ID, matchID, unique+"-master", "MASTER_PLUS")
		insertPreviewDiscovery(t, store, run.ID, matchID, unique+"-diamond", "DIAMOND")
	}
	// These facts must not enter the preview: one belongs to another run, one
	// has a known patch, and one does not have eight distinct placements.
	otherMatch := unique + "-other-match"
	insertPreviewMatch(t, store, otherMatch, "", "invalid_game_version", normalPlacements(), unitIDs, now)
	insertPreviewDiscovery(t, store, otherRun.ID, otherMatch, unique+"-other-seed", "MASTER_PLUS")
	knownPatchMatch := unique + "-known-patch"
	insertPreviewMatch(t, store, knownPatchMatch, "99.1", "", normalPlacements(), unitIDs, now)
	insertPreviewDiscovery(t, store, run.ID, knownPatchMatch, unique+"-known-seed", "MASTER_PLUS")
	badPlacementMatch := unique + "-bad-placement"
	insertPreviewMatch(t, store, badPlacementMatch, "", "invalid_game_version", []int{1, 2, 3, 4, 5, 6, 7, 7}, unitIDs, now)
	insertPreviewDiscovery(t, store, run.ID, badPlacementMatch, unique+"-bad-seed", "MASTER_PLUS")

	row, err := q.GetTFTObservedLineupPreview(ctx, run.ID, "KR")
	require.NoError(t, err)
	require.Equal(t, int64(3), row.SourceMatches, "multiple seed/cohort discovery edges must count each match once")
	require.Equal(t, int64(24), row.SourceParticipants)
	require.Equal(t, int64(24), row.UsableParticipants)
	require.Equal(t, int64(1), row.ExactLineups)
	require.Equal(t, snapshotID, row.CatalogSnapshotID)
	require.Equal(t, "99.1", row.CatalogPatch)
	require.Equal(t, unique+"-catalog-revision", row.CatalogRevision)

	body, err := json.Marshal(row.Lineups)
	require.NoError(t, err)
	var lineups []struct {
		ID            string   `json:"id"`
		UnitIDs       []string `json:"unit_ids"`
		SampleSize    int64    `json:"sample_size"`
		LobbyCount    int64    `json:"lobby_count"`
		ContestedRate float64  `json:"contested_rate"`
	}
	require.NoError(t, json.Unmarshal(body, &lineups))
	require.Len(t, lineups, 1)
	require.Equal(t, int64(24), lineups[0].SampleSize)
	require.Equal(t, int64(3), lineups[0].LobbyCount)
	require.Equal(t, 1.0, lineups[0].ContestedRate)
	require.Equal(t, unitIDs, lineups[0].UnitIDs)
	require.NotContains(t, lineups[0].ID, summonID)
	require.NotContains(t, string(body), "puuid")

	newerSnapshotID := insertPreviewCatalog(t, store, unique+"-newer", now.Add(2*time.Hour))
	require.NotEqual(t, snapshotID, newerSnapshotID)
	detailsValue, err := q.GetTFTObservedLineupDetails(ctx, sqlcgen.GetTFTObservedLineupDetailsParams{
		RunID: run.ID, Platform: "KR", CatalogSnapshotID: row.CatalogSnapshotID,
		Signatures: []string{lineups[0].ID},
	})
	require.NoError(t, err)
	detailsBody, err := json.Marshal(detailsValue)
	require.NoError(t, err)
	var details []struct {
		ID                          string `json:"id"`
		StarCompositionKnownSamples int64  `json:"star_composition_known_samples"`
		UnitItems                   []struct {
			UnitID             string  `json:"unit_id"`
			CoreRank           *int    `json:"core_rank"`
			AverageItems       float64 `json:"average_items"`
			ItemInvestmentRate float64 `json:"item_investment_rate"`
			EquippedRate       float64 `json:"equipped_rate"`
			ThreeItemRate      float64 `json:"three_item_rate"`
			KnownStarSamples   int64   `json:"known_star_samples"`
			UnknownStarSamples int64   `json:"unknown_star_samples"`
			StarCoverage       float64 `json:"star_coverage"`
			Items              []struct {
				ID    string  `json:"id"`
				Count int64   `json:"count"`
				Rate  float64 `json:"rate"`
			} `json:"items"`
			StarDistribution []struct {
				Stars        int      `json:"stars"`
				SampleSize   int64    `json:"sample_size"`
				Rate         float64  `json:"rate"`
				KnownRate    float64  `json:"known_rate"`
				AvgPlacement *float64 `json:"avg_placement"`
			} `json:"star_distribution"`
		} `json:"unit_items"`
		StarLevels []struct {
			TotalStars   int     `json:"total_stars"`
			SampleSize   int64   `json:"sample_size"`
			LobbyCount   int64   `json:"lobby_count"`
			Rate         float64 `json:"rate"`
			AvgPlacement float64 `json:"avg_placement"`
			FirstRate    float64 `json:"first_rate"`
			Top4Rate     float64 `json:"top4_rate"`
		} `json:"star_levels"`
		StarCompositions []struct {
			StarLevels   []int    `json:"star_levels"`
			TotalStars   int      `json:"total_stars"`
			SampleSize   int64    `json:"sample_size"`
			Rate         float64  `json:"rate"`
			AvgPlacement *float64 `json:"avg_placement"`
		} `json:"star_compositions"`
	}
	require.NoError(t, json.Unmarshal(detailsBody, &details))
	require.Len(t, details, 1)
	require.Equal(t, lineups[0].ID, details[0].ID)
	require.Len(t, details[0].UnitItems, 6)
	unitDetails := make(map[string]struct {
		CoreRank           *int
		AverageItems       float64
		ItemInvestmentRate float64
		EquippedRate       float64
		ThreeItemRate      float64
		KnownStarSamples   int64
		UnknownStarSamples int64
		StarCoverage       float64
	})
	for _, unit := range details[0].UnitItems {
		unitDetails[unit.UnitID] = struct {
			CoreRank           *int
			AverageItems       float64
			ItemInvestmentRate float64
			EquippedRate       float64
			ThreeItemRate      float64
			KnownStarSamples   int64
			UnknownStarSamples int64
			StarCoverage       float64
		}{unit.CoreRank, unit.AverageItems, unit.ItemInvestmentRate, unit.EquippedRate, unit.ThreeItemRate, unit.KnownStarSamples, unit.UnknownStarSamples, unit.StarCoverage}
	}
	require.Equal(t, 1, *unitDetails[unitIDs[0]].CoreRank)
	require.Equal(t, 1, *unitDetails[unitIDs[1]].CoreRank, "identical item investment must share the same core rank")
	require.Nil(t, unitDetails[unitIDs[2]].CoreRank)
	require.InDelta(t, 1.0, unitDetails[unitIDs[0]].ItemInvestmentRate, 0.0001)
	require.InDelta(t, 2.0/3.0, unitDetails[unitIDs[2]].ItemInvestmentRate, 0.0001)
	require.Equal(t, int64(1), unitDetails[unitIDs[5]].UnknownStarSamples)
	require.InDelta(t, 23.0/24.0, unitDetails[unitIDs[5]].StarCoverage, 0.0001)
	require.Equal(t, unitIDs[0], details[0].UnitItems[0].UnitID)
	require.Equal(t, []string{unique + "-item-common-1", unique + "-item-common-2"}, []string{
		details[0].UnitItems[0].Items[0].ID,
		details[0].UnitItems[0].Items[1].ID,
	})
	require.Equal(t, int64(24), details[0].UnitItems[0].Items[0].Count)
	require.Equal(t, 1.0, details[0].UnitItems[0].Items[0].Rate)
	require.Equal(t, int64(24), details[0].UnitItems[0].Items[1].Count)
	require.Equal(t, 1.0, details[0].UnitItems[0].Items[1].Rate)
	require.Len(t, details[0].StarLevels, 3)
	for index, totalStars := range []int{9, 14, 19} {
		strength := details[0].StarLevels[index]
		require.Equal(t, totalStars, strength.TotalStars)
		expectedSamples := int64(8)
		if totalStars == 19 {
			expectedSamples = 7
		}
		require.Equal(t, expectedSamples, strength.SampleSize)
		require.Equal(t, int64(1), strength.LobbyCount)
		require.InDelta(t, float64(expectedSamples)/24.0, strength.Rate, 0.0001)
	}
	require.Equal(t, int64(23), details[0].StarCompositionKnownSamples)
	require.Len(t, details[0].StarCompositions, 4)
	compositionByTotal := make(map[int]struct {
		Levels  []int
		Samples int64
	})
	var totalFourteen []struct {
		Levels  []int
		Samples int64
	}
	for _, composition := range details[0].StarCompositions {
		if composition.TotalStars == 14 {
			totalFourteen = append(totalFourteen, struct {
				Levels  []int
				Samples int64
			}{composition.StarLevels, composition.SampleSize})
		}
		compositionByTotal[composition.TotalStars] = struct {
			Levels  []int
			Samples int64
		}{composition.StarLevels, composition.SampleSize}
	}
	require.Equal(t, []int{1, 1, 1, 1, 1, 4}, compositionByTotal[9].Levels)
	require.Equal(t, int64(8), compositionByTotal[9].Samples)
	require.Equal(t, []int{3, 3, 3, 3, 3, 4}, compositionByTotal[19].Levels)
	require.Equal(t, int64(7), compositionByTotal[19].Samples)
	require.ElementsMatch(t, []struct {
		Levels  []int
		Samples int64
	}{
		{[]int{1, 2, 2, 2, 3, 4}, 1},
		{[]int{2, 2, 2, 2, 2, 4}, 7},
	}, totalFourteen, "different star distributions with the same total must remain separate")
	require.NotContains(t, string(detailsBody), unique+"-duplicate-item-1")
	require.NotContains(t, string(detailsBody), unique+"-duplicate-item-2")
	require.NotContains(t, string(detailsBody), unique+"-summon-item")
	require.NotContains(t, string(detailsBody), "puuid")
}

func createCompletedPreviewRun(t *testing.T, q *sqlcgen.Queries, unique string, now time.Time) sqlcgen.TftCrawlRun {
	t.Helper()
	run, err := q.CreateTFTRun(context.Background(), sqlcgen.CreateTFTRunParams{
		WorkflowID: unique, WorkflowRunID: unique + "-workflow-run", ScheduleID: unique,
		ProfileName: unique, Platform: "GLOBAL", RoutingRegion: "GLOBAL",
		QueueType: "RANKED_TFT", QueueID: 1100, Status: "completed",
		WindowStart: pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true},
		WindowEnd:   pgtype.Timestamptz{Time: now, Valid: true},
		Config:      []byte(`{}`),
	})
	require.NoError(t, err)
	return run
}

func insertPreviewCatalog(t *testing.T, store *Store, unique string, now time.Time) int64 {
	t.Helper()
	var snapshotID int64
	err := store.Pool.QueryRow(context.Background(), `
		INSERT INTO tft_static_snapshots (
			source, patch, build, revision, locale, status, source_url,
			parser_version, fetched_at, published_at
		) VALUES ('ddragon', '99.1', $1, $2, 'en_us', 'published',
			'https://example.invalid/tft-preview-test', 'test-v1', $3::timestamptz, $3::timestamptz + interval '1 day')
		RETURNING id`, unique, unique+"-catalog-revision", now).Scan(&snapshotID)
	require.NoError(t, err)
	for _, suffix := range []string{"A", "B", "C", "D", "E", "F"} {
		id := unique + "-" + suffix
		_, err = store.Pool.Exec(context.Background(), `
			INSERT INTO tft_static_objects (
				snapshot_id, object_kind, object_id, name, purchasable, cost, payload
			) VALUES ($1, 'unit', $2::text, $2::text, true, 1, jsonb_build_object('id', $2::text))`, snapshotID, id)
		require.NoError(t, err)
	}
	_, err = store.Pool.Exec(context.Background(), `
		INSERT INTO tft_static_objects (
			snapshot_id, object_kind, object_id, name, purchasable, cost, payload
		) VALUES ($1, 'unit', $2::text, $2::text, false, 0, jsonb_build_object('id', $2::text))`, snapshotID, unique+"-summon")
	require.NoError(t, err)
	return snapshotID
}

func insertPreviewMatch(t *testing.T, store *Store, matchID, patch, exclusion string, placements []int, unitIDs []string, now time.Time) {
	t.Helper()
	var exclusionValue any
	if exclusion != "" {
		exclusionValue = exclusion
	}
	_, err := store.Pool.Exec(context.Background(), `
		INSERT INTO tft_matches (
			match_id, platform, routing_region, queue_id, game_version, patch,
			game_datetime, set_number, participant_count, eligible, exclusion_reason
		) VALUES ($1, 'KR', 'ASIA', 1100, 'TFT Unreal Version ?.?.?.?', $2,
			$3, 18, 8, false, $4)`, matchID, patch, now, exclusionValue)
	require.NoError(t, err)
	for index, placement := range placements {
		puuid := fmt.Sprintf("%s-puuid-%d", matchID, index)
		_, err = store.Pool.Exec(context.Background(), `
			INSERT INTO tft_match_participants (
				match_id, puuid, placement, mapped_unit_count, abnormal
			) VALUES ($1, $2, $3, 0, true)`, matchID, puuid, placement)
		require.NoError(t, err)
		for unitIndex, unitID := range unitIDs {
			_, err = store.Pool.Exec(context.Background(), `
				INSERT INTO tft_match_units (
					match_id, puuid, unit_index, character_id, mapped
				) VALUES ($1, $2, $3, $4, false)`, matchID, puuid, unitIndex, unitID)
			require.NoError(t, err)
		}
	}
}

func insertPreviewDiscovery(t *testing.T, store *Store, runID int64, matchID, seed, cohort string) {
	t.Helper()
	_, err := store.Pool.Exec(context.Background(), `
		INSERT INTO tft_match_discoveries (
			run_id, platform, routing_region, seed_puuid, match_id, cohort
		) VALUES ($1, 'KR', 'ASIA', $2, $3, $4)`, runID, seed+"-"+matchID, matchID, cohort)
	require.NoError(t, err)
}

func insertPreviewEquipmentAndStars(t *testing.T, store *Store, matchID, duplicateUnitID, summonID string, starLevel int) {
	t.Helper()
	for participantIndex := range 8 {
		puuid := fmt.Sprintf("%s-puuid-%d", matchID, participantIndex)
		_, err := store.Pool.Exec(context.Background(), `
			UPDATE tft_match_units
			SET tier = CASE WHEN character_id = $3 THEN 4 ELSE $4 END
			WHERE match_id = $1 AND puuid = $2`, matchID, puuid, summonID, starLevel)
		require.NoError(t, err)

		_, err = store.Pool.Exec(context.Background(), `
			INSERT INTO tft_match_units (
				match_id, puuid, unit_index, character_id, tier, mapped
			) VALUES ($1, $2, 7, $3, 4, false)`, matchID, puuid, duplicateUnitID)
		require.NoError(t, err)

		// The intended representative has three occupied slots but only two
		// distinct item IDs. The higher-tier duplicate below has two occupied
		// slots with two distinct IDs. Representative selection must use slot
		// count, not distinct item count.
		items := []string{"item-common-1", "item-common-1", "item-common-2"}
		for slot, suffix := range items {
			_, err = store.Pool.Exec(context.Background(), `
				INSERT INTO tft_match_unit_items (
					match_id, puuid, unit_index, item_slot, item_id
				) VALUES ($1, $2, 0, $3, $4)`, matchID, puuid, slot, previewPrefix(matchID)+"-"+suffix)
			require.NoError(t, err)
		}
		_, err = store.Pool.Exec(context.Background(), `
			INSERT INTO tft_match_unit_items (
				match_id, puuid, unit_index, item_slot, item_id
			) VALUES
				($1, $2, 7, 0, $3),
				($1, $2, 7, 1, $4),
				($1, $2, 6, 0, $5)`,
			matchID, puuid,
			previewPrefix(matchID)+"-duplicate-item-1",
			previewPrefix(matchID)+"-duplicate-item-2",
			previewPrefix(matchID)+"-summon-item")
		require.NoError(t, err)

		for unitIndex, itemCount := range []int{3, 2} {
			for slot := range itemCount {
				_, err = store.Pool.Exec(context.Background(), `
					INSERT INTO tft_match_unit_items (
						match_id, puuid, unit_index, item_slot, item_id
					) VALUES ($1, $2, $3, $4, $5)`,
					matchID, puuid, unitIndex+1, slot,
					fmt.Sprintf("%s-unit-%d-item-%d", previewPrefix(matchID), unitIndex+1, slot))
				require.NoError(t, err)
			}
		}
		if starLevel == 3 && participantIndex == 7 {
			_, err = store.Pool.Exec(context.Background(), `
				UPDATE tft_match_units
				SET tier = 0
				WHERE match_id = $1 AND puuid = $2 AND character_id = $3`,
				matchID, puuid, previewPrefix(matchID)+"-F")
			require.NoError(t, err)
		}
		if starLevel == 2 && participantIndex == 0 {
			_, err = store.Pool.Exec(context.Background(), `
				UPDATE tft_match_units
				SET tier = CASE character_id WHEN $3 THEN 1 WHEN $4 THEN 3 ELSE tier END
				WHERE match_id = $1 AND puuid = $2`,
				matchID, puuid,
				previewPrefix(matchID)+"-B",
				previewPrefix(matchID)+"-C")
			require.NoError(t, err)
		}
	}
}

func previewPrefix(matchID string) string {
	if index := strings.Index(matchID, "-match-"); index >= 0 {
		return matchID[:index]
	}
	return matchID
}

func normalPlacements() []int { return []int{1, 2, 3, 4, 5, 6, 7, 8} }
