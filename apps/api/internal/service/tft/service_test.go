package tft

import (
	"context"
	"strings"
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
