package tft

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

func TestValidateTFTFilter(t *testing.T) {
	f := Normalize(Filter{Platform: "kr", SetNumber: 15})
	require.NoError(t, Validate(f))
	require.Equal(t, "KR", f.Platform)
	require.Equal(t, 200, f.MinSamples)
	require.Equal(t, "PATCH", f.Window)
}

func TestHistoryCursorIsBoundToQueryScope(t *testing.T) {
	fingerprint := historyFingerprint(HistoryFilter{Identity: Identity{Platform: "KR"}, QueueID: 1100, Locale: "zh_cn"}, "puuid-1")
	raw := encodeHistoryCursor(historyCursor{Time: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC), MatchID: "KR_1", Seen: 20, Fingerprint: fingerprint})

	decoded, err := decodeHistoryCursor(raw, fingerprint)
	require.NoError(t, err)
	require.Equal(t, 20, decoded.Seen)
	_, err = decodeHistoryCursor(raw, historyFingerprint(HistoryFilter{Identity: Identity{Platform: "KR"}, QueueID: 0, Locale: "zh_cn"}, "puuid-1"))
	require.Error(t, err)
	_, err = decodeHistoryCursor(raw, historyFingerprint(HistoryFilter{Identity: Identity{Platform: "KR"}, QueueID: 1100, Locale: "en_us"}, "puuid-1"))
	require.Error(t, err)
}

func TestLocalTFTIconURLUsesPublishedLocalAssetPath(t *testing.T) {
	url := localTFTIconURL([]byte(`{"icon":"/lol-game-data/assets/ASSETS/Characters/Ahri.png"}`), "16.17", "revision-1")

	require.True(t, strings.HasPrefix(url, "/game-assets/tft/static/cdragon/16.17/revision-1/assets/"))
	require.True(t, strings.HasSuffix(url, ".png"))
	require.NotContains(t, url, "communitydragon.org")
}

func TestLocalTFTIconURLSupportsClientCatalogSquareIcon(t *testing.T) {
	url := localTFTIconURL([]byte(`{"squareIconPath":"/lol-game-data/assets/ASSETS/Characters/Ahri.png"}`), "16.17", "revision-1")

	require.True(t, strings.HasPrefix(url, "/game-assets/tft/static/cdragon/16.17/revision-1/assets/"))
	require.True(t, strings.HasSuffix(url, ".png"))
}

type reconnectQuerier struct {
	FullQuerier
	job sqlcgen.TftPlayerLookupJob
}

func (q *reconnectQuerier) GetTFTPlayerIdentity(context.Context, string, string, string) (sqlcgen.TftPlayerIdentity, error) {
	return sqlcgen.TftPlayerIdentity{}, pgx.ErrNoRows
}
func (q *reconnectQuerier) DeleteExpiredTFTPlayerLookupJobs(context.Context) (int64, error) {
	return 0, nil
}
func (q *reconnectQuerier) FindActiveTFTPlayerLookupJob(context.Context, string, string, string) (sqlcgen.TftPlayerLookupJob, error) {
	return q.job, nil
}

type recordingStarter struct{ calls []WorkflowInput }

func (s *recordingStarter) StartTFTPlayerWorkflow(_ context.Context, _ string, input WorkflowInput) error {
	s.calls = append(s.calls, input)
	return nil
}

func TestRefreshReissuesStableWorkflowForRunningJob(t *testing.T) {
	now := time.Now()
	q := &reconnectQuerier{job: sqlcgen.TftPlayerLookupJob{
		ID: "00000000-0000-4000-8000-000000000001", Platform: "KR",
		RequestedGameName: "Player", RequestedTagLine: "KR1", Status: "RUNNING", Stage: "FETCH_MATCHES",
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}}
	starter := &recordingStarter{}
	service := New(q, nil, starter, RuntimeConfig{Freshness: 10 * time.Minute})

	result, err := service.Refresh(context.Background(), Identity{Platform: "kr", GameName: "Player", TagLine: "kr1"}, "127.0.0.1")

	require.NoError(t, err)
	require.True(t, result.Reused)
	require.NotNil(t, result.Job)
	require.Len(t, starter.calls, 1)
	require.Equal(t, q.job.ID, starter.calls[0].JobID)
}
func TestValidateTFTFilterRejectsUnknownLocale(t *testing.T) {
	f := Normalize(Filter{Platform: "KR", SetNumber: 15, Locale: "fr_fr"})
	require.Error(t, Validate(f))
}

func TestParseCoverage(t *testing.T) {
	coverage := parseCoverage([]byte(`{"window_start":"2026-08-20T00:00:00Z","window_end":"2026-08-23T00:00:00Z","exact_lineups":31,"family_threshold":200}`))

	require.Equal(t, time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), coverage.WindowStart)
	require.Equal(t, time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC), coverage.WindowEnd)
	require.Equal(t, 31, coverage.ExactLineups)
	require.Equal(t, 200, coverage.FamilyThreshold)
}

type observedQuerier struct {
	FullQuerier
	mu              sync.Mutex
	previewRunID    int64
	previewPlatform string
	previewCalls    int
	detailCalls     int
	detailSnapshot  int64
	detailBatches   [][]string
	detailStarted   chan struct{}
	detailRelease   <-chan struct{}
	detailStartOnce sync.Once
	localizedKinds  []string
}

func (q *observedQuerier) GetLatestCompletedTFTObservedRun(context.Context, string) (sqlcgen.TftCrawlRun, error) {
	return sqlcgen.TftCrawlRun{ID: 5, Status: "completed"}, nil
}

func (q *observedQuerier) GetTFTObservedLineupPreview(_ context.Context, runID int64, platform string) (sqlcgen.GetTFTObservedLineupPreviewRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.previewRunID = runID
	q.previewPlatform = platform
	q.previewCalls++
	setNumber := int32(18)
	return sqlcgen.GetTFTObservedLineupPreviewRow{
		RunID: 5, Platform: "KR", Platforms: []string{"KR", "NA1"},
		QueueID: 1100, SetNumber: &setNumber,
		RawGameVersions: []string{"TFT Unreal Version ?.?.?.?"},
		SourceMatches:   946, SourceParticipants: 7568, UsableParticipants: 6838,
		ExactLineups: 3675, CatalogSnapshotID: 41,
		CatalogPatch: "16.17", CatalogRevision: "catalog-rev",
		WindowStart: pgtype.Timestamptz{Time: time.Date(2026, 8, 26, 21, 7, 43, 0, time.UTC), Valid: true},
		WindowEnd:   pgtype.Timestamptz{Time: time.Date(2026, 8, 29, 18, 7, 59, 0, time.UTC), Valid: true},
		Lineups: []map[string]any{{
			"id": "DA_18_Aphelios|DA_18_Xayah", "unit_ids": []string{"DA_18_Aphelios", "DA_18_Xayah"},
			"sample_size": 58, "lobby_count": 58, "pick_rate": 0.0085,
			"avg_placement": 2.62, "first_rate": 0.36, "top4_rate": 0.86, "contested_rate": 0.0,
		}, {
			"id":          "DA_18_Aphelios|DA_18_Xayah|DA_18_Yasuo",
			"unit_ids":    []string{"DA_18_Aphelios", "DA_18_Xayah", "DA_18_Yasuo"},
			"sample_size": 40, "lobby_count": 40, "pick_rate": 0.006,
			"avg_placement": 2.8, "first_rate": 0.3, "top4_rate": 0.8, "contested_rate": 0.0,
		}},
	}, nil
}

func (q *observedQuerier) GetTFTObservedLineupDetails(ctx context.Context, params sqlcgen.GetTFTObservedLineupDetailsParams) (interface{}, error) {
	q.mu.Lock()
	q.previewRunID = params.RunID
	q.previewPlatform = params.Platform
	q.detailSnapshot = params.CatalogSnapshotID
	q.detailCalls++
	q.detailBatches = append(q.detailBatches, append([]string(nil), params.Signatures...))
	q.mu.Unlock()
	if q.detailStarted != nil {
		q.detailStartOnce.Do(func() { close(q.detailStarted) })
	}
	if q.detailRelease != nil {
		select {
		case <-q.detailRelease:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	details := make([]map[string]any, 0, len(params.Signatures))
	for _, signature := range params.Signatures {
		details = append(details, map[string]any{
			"id": signature,
			"unit_items": []map[string]any{{
				"unit_id":   "DA_18_Aphelios",
				"items":     []map[string]any{{"id": "DA_GuinsoosRageblade", "count": 40, "rate": 40.0 / 58.0}},
				"core_rank": 1, "average_items": 2.95,
				"item_investment_rate": 2.95 / 3.0, "equipped_rate": 1.0,
				"three_item_rate": 0.98, "known_star_samples": 58,
				"unknown_star_samples": 0, "star_coverage": 1.0,
				"star_distribution": []map[string]any{{
					"stars": 3, "sample_size": 57, "rate": 57.0 / 58.0,
					"known_rate": 57.0 / 58.0, "avg_placement": 2.58,
					"first_rate": 0.37, "top4_rate": 0.88,
				}},
			}},
			"star_levels": []map[string]any{{
				"total_stars": 20, "sample_size": 15, "lobby_count": 15, "rate": 15.0 / 58.0,
				"avg_placement": 2.73, "first_rate": 0.27, "top4_rate": 0.73,
			}},
			"star_composition_known_samples": 58,
			"star_compositions": []map[string]any{{
				"star_levels": []int{2, 2, 2, 3, 3, 3},
				"total_stars": 15, "sample_size": 12, "rate": 12.0 / 58.0,
				"avg_placement": 1.92, "first_rate": 0.5, "top4_rate": 1.0,
			}},
		})
	}
	return details, nil
}

func (q *observedQuerier) calls() (preview, details int, snapshot int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.previewCalls, q.detailCalls, q.detailSnapshot
}

func (q *observedQuerier) batches() [][]string {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([][]string, len(q.detailBatches))
	for index := range q.detailBatches {
		out[index] = append([]string(nil), q.detailBatches[index]...)
	}
	return out
}

func (q *observedQuerier) ListLatestTFTLocalizedStaticObjects(_ context.Context, kinds []string, _ string) ([]sqlcgen.ListLatestTFTLocalizedStaticObjectsRow, error) {
	q.localizedKinds = append([]string(nil), kinds...)
	name := "Aphelios"
	itemName := "Guinsoo's Rageblade"
	return []sqlcgen.ListLatestTFTLocalizedStaticObjectsRow{{
		ObjectKind: "unit", ObjectID: "DA_18_Aphelios", Name: &name,
		Payload: []byte(`{"iconPath":"/lol-game-data/assets/ASSETS/Characters/Aphelios.png"}`),
		Patch:   "16.17", Revision: "static-rev",
	}, {
		ObjectKind: "item", ObjectID: "DA_GuinsoosRageblade", Name: &itemName,
		Payload: []byte(`{"squareIconPath":"/lol-game-data/assets/ASSETS/Maps/TFT/Icons/Items/Guinsoos.png"}`),
		Patch:   "16.17", Revision: "static-rev",
	}}, nil
}

func TestObservedLineupsKeepsMatchPatchUnknownAndLabelsReferenceAssets(t *testing.T) {
	q := &observedQuerier{}
	service := New(q, nil, nil, RuntimeConfig{})

	result, err := service.ObservedLineups(context.Background(), ObservedFilter{})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(5), result.RunID)
	require.Nil(t, result.Patch)
	require.Equal(t, ObservedPreviewDataKind, result.DataKind)
	require.Equal(t, ObservedPreviewAlgorithm, result.AlgorithmVersion)
	require.Equal(t, "ddragon", result.CatalogSnapshot.Source)
	require.Equal(t, "catalog-rev", result.CatalogSnapshot.Revision)
	require.Equal(t, "16.17", result.AssetSnapshot.Patch)
	require.Equal(t, int64(6838), result.UsableParticipants)
	require.Len(t, result.Items, 2)
	require.Equal(t, "Aphelios", result.Items[0].CoreUnits[0].Name)
	require.Equal(t, "DA_18_Xayah", result.Items[0].CoreUnits[1].ID)
	require.Empty(t, result.Items[0].CoreUnits[1].Name)
	require.Equal(t, []string{"unit", "item"}, q.localizedKinds)
	require.Len(t, result.Items[0].UnitItems, 2)
	require.Len(t, result.Items[0].UnitItems[0].CommonItems, 1)
	require.Equal(t, "Guinsoo's Rageblade", result.Items[0].UnitItems[0].CommonItems[0].Entity.Name)
	require.Contains(t, result.Items[0].UnitItems[0].CommonItems[0].Entity.IconURL, "/game-assets/tft/static/cdragon/16.17/static-rev/")
	require.Empty(t, result.Items[0].UnitItems[1].CommonItems)
	require.Equal(t, 1, *result.Items[0].UnitItems[0].CoreRank)
	require.InDelta(t, 2.95/3.0, result.Items[0].UnitItems[0].ItemInvestmentRate, 0.0001)
	require.Len(t, result.Items[0].UnitItems[0].StarDistribution, 1)
	require.Equal(t, 3, result.Items[0].UnitItems[0].StarDistribution[0].Stars)
	require.InDelta(t, 2.58, *result.Items[0].UnitItems[0].StarDistribution[0].AvgPlacement, 0.0001)
	require.Len(t, result.Items[0].StarLevels, 1)
	require.Equal(t, 20, result.Items[0].StarLevels[0].TotalStars)
	require.Equal(t, int64(15), result.Items[0].StarLevels[0].SampleSize)
	require.InDelta(t, 15.0/58.0, result.Items[0].StarLevels[0].Rate, 0.0001)
	require.Equal(t, int64(58), result.Items[0].StarCompositionKnownSamples)
	require.Zero(t, result.Items[0].StarCompositionUnknownSamples)
	require.Len(t, result.Items[0].StarCompositions, 1)
	require.Equal(t, []StarCount{{Stars: 2, UnitCount: 3}, {Stars: 3, UnitCount: 3}}, result.Items[0].StarCompositions[0].Levels)
	require.InDelta(t, 1.92, *result.Items[0].StarCompositions[0].AvgPlacement, 0.0001)
	require.Equal(t, int64(5), q.previewRunID)
	require.Equal(t, "KR", q.previewPlatform)

	result, err = service.ObservedLineups(context.Background(), ObservedFilter{MinSamples: 50, Limit: 1})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	previewCalls, detailCalls, snapshotID := q.calls()
	require.Equal(t, 1, previewCalls, "filter variants must reuse the cached run/platform rollup")
	require.Equal(t, 1, detailCalls, "the selected lineup details must reuse the run/platform cache")
	require.Equal(t, int64(41), snapshotID, "details must use the catalog snapshot selected by the base rollup")
}

func TestObservedLineupsCoalescesDifferentFiltersAndCallerCanCancel(t *testing.T) {
	release := make(chan struct{})
	q := &observedQuerier{detailStarted: make(chan struct{}), detailRelease: release}
	service := New(q, nil, nil, RuntimeConfig{})

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.ObservedLineups(context.Background(), ObservedFilter{Limit: 1})
		firstDone <- err
	}()
	<-q.detailStarted

	canceledCtx, cancel := context.WithCancel(context.Background())
	secondDone := make(chan error, 1)
	go func() {
		_, err := service.ObservedLineups(canceledCtx, ObservedFilter{MinSamples: 50, Limit: 2})
		secondDone <- err
	}()
	cancel()

	select {
	case err := <-secondDone:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("canceled caller remained blocked on the shared details query")
	}
	_, detailCalls, _ := q.calls()
	require.Equal(t, 1, detailCalls, "different filters must share one run/platform/catalog details scan")

	close(release)
	require.NoError(t, <-firstDone)
}

func TestObservedLineupsIncrementallyCachesMissingSelectionDetails(t *testing.T) {
	q := &observedQuerier{}
	service := New(q, nil, nil, RuntimeConfig{})

	first, err := service.ObservedLineups(context.Background(), ObservedFilter{Limit: 1})
	require.NoError(t, err)
	require.Len(t, first.Items, 1)

	second, err := service.ObservedLineups(context.Background(), ObservedFilter{Limit: 2})
	require.NoError(t, err)
	require.Len(t, second.Items, 2)
	require.NotEmpty(t, second.Items[1].UnitItems[0].StarDistribution)

	third, err := service.ObservedLineups(context.Background(), ObservedFilter{Limit: 2})
	require.NoError(t, err)
	require.Len(t, third.Items, 2)
	require.Equal(t, [][]string{
		{"DA_18_Aphelios|DA_18_Xayah"},
		{"DA_18_Aphelios|DA_18_Xayah|DA_18_Yasuo"},
	}, q.batches(), "expanded selections should query only uncached signatures")
}

func TestObservedLineupsReturnsInFlightDetailsAfterBaseCacheExpires(t *testing.T) {
	release := make(chan struct{})
	q := &observedQuerier{detailStarted: make(chan struct{}), detailRelease: release}
	service := New(q, nil, nil, RuntimeConfig{})
	var nowMu sync.Mutex
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time {
		nowMu.Lock()
		defer nowMu.Unlock()
		return now
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := service.ObservedLineups(ctx, ObservedFilter{Limit: 1})
		done <- err
	}()
	<-q.detailStarted

	nowMu.Lock()
	now = now.Add(ObservedPreviewCacheTTL + time.Second)
	nowMu.Unlock()
	close(release)

	require.NoError(t, <-done, "the completed details query must serve its callers even when the cache write expires")
	_, detailCalls, _ := q.calls()
	require.Equal(t, 1, detailCalls, "cache expiry during a query must not trigger another details scan")
}

func TestValidateObservedFilterRejectsNegativeRunID(t *testing.T) {
	filter := NormalizeObservedFilter(ObservedFilter{RunID: -1, Platform: "kr"})
	require.Error(t, ValidateObservedFilter(filter))
	require.Error(t, ValidateObservedFilter(ObservedFilter{
		Platform: "KR", Locale: "en_us", MinSamples: 19, Limit: 30,
	}))
	require.Error(t, ValidateObservedFilter(ObservedFilter{
		Platform: "KR", Locale: "en_us", MinSamples: 20, Limit: 31,
	}))
}
