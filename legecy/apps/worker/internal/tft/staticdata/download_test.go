package staticdata

import (
	"context"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

func TestDownloadAssetsForSnapshotsFetchesDuplicateURLOnce(t *testing.T) {
	var requests atomic.Int32
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		return staticHTTPResponse(req, http.StatusOK, "asset-body"), nil
	})}

	q := newScopedAssetQuerier([]sqlcgen.ClaimTFTStaticAssetGroupsRow{
		{SnapshotID: 11, AssetKey: "same-key", SourceUrl: "https://assets.example/asset.png", RelativePath: "one/asset.png"},
		{SnapshotID: 13, AssetKey: "same-key", SourceUrl: "https://assets.example/asset.png", RelativePath: "two/asset.png"},
	})
	root := t.TempDir()
	result, err := DownloadAssetsForSnapshots(context.Background(), q, root, client, "owner", DownloadInput{
		SnapshotIDs: []int64{11, 13, 11}, BatchSize: 128, Concurrency: 16,
	})

	require.NoError(t, err)
	require.Equal(t, int32(1), requests.Load())
	require.Equal(t, 1, result.Fetched)
	require.Equal(t, 2, result.Processed)
	require.Equal(t, int64(2), result.Completed)
	require.Zero(t, result.Remaining)
	require.Equal(t, []int64{11, 13}, q.claim.SnapshotIds)
	require.True(t, q.released)
	for _, relative := range []string{"one/asset.png", "two/asset.png"} {
		body, readErr := os.ReadFile(filepath.Join(root, relative))
		require.NoError(t, readErr)
		require.Equal(t, []byte("asset-body"), body)
	}
}

func TestDownloadAssetsForSnapshotsSkipsKnownMissingQueueIcon(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return staticHTTPResponse(req, http.StatusForbidden, "missing"), nil
	})}
	q := newScopedAssetQuerier([]sqlcgen.ClaimTFTStaticAssetGroupsRow{{
		SnapshotID: 11, AssetKey: "queue-icon",
		SourceUrl:    "https://ddragon.example/cdn/16.17.1/img/tft-queue-type/TFTM_ModeIcon_CT.png",
		RelativePath: "queue/icon.png",
	}})
	root := t.TempDir()

	result, err := DownloadAssetsForSnapshots(context.Background(), q, root, client, "owner", DownloadInput{
		SnapshotIDs: []int64{11}, BatchSize: 8, Concurrency: 2,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), result.Skipped)
	require.Zero(t, result.Failed)
	file, openErr := os.Open(filepath.Join(root, "queue/icon.png"))
	require.NoError(t, openErr)
	defer file.Close()
	_, decodeErr := png.Decode(file)
	require.NoError(t, decodeErr)
}

func TestDownloadAssetsForSnapshotsRetriesUnexpectedForbiddenIcon(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return staticHTTPResponse(req, http.StatusForbidden, "forbidden"), nil
	})}
	q := newScopedAssetQuerier([]sqlcgen.ClaimTFTStaticAssetGroupsRow{{
		SnapshotID: 12, AssetKey: "unit-icon",
		SourceUrl:    "https://raw.communitydragon.example/plugins/rcp-be-lol-game-data/global/default/assets/unit.png",
		RelativePath: "unit/icon.png",
	}})
	root := t.TempDir()

	result, err := DownloadAssetsForSnapshots(context.Background(), q, root, client, "owner", DownloadInput{
		SnapshotIDs: []int64{12}, BatchSize: 8, Concurrency: 2,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), result.Failed)
	require.Equal(t, int64(1), result.Remaining)
	require.True(t, result.HasMore)
	require.Equal(t, "failed", q.status[12])
	_, statErr := os.Stat(filepath.Join(root, "unit/icon.png"))
	require.Error(t, statErr)
}

func TestDownloadAssetsForSnapshotsRejectsUnsafeBatchToWorkerRatio(t *testing.T) {
	q := newScopedAssetQuerier(nil)

	_, err := DownloadAssetsForSnapshots(context.Background(), q, t.TempDir(), nil, "owner", DownloadInput{
		SnapshotIDs: []int64{1}, BatchSize: 9, Concurrency: 1,
	})

	require.ErrorContains(t, err, "exceeds safe limit 8")
}

func TestDownloadAssetsForSnapshotsRejectsOversizedAsset(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return staticHTTPResponse(req, http.StatusOK, strings.Repeat("x", maxAssetBody+1)), nil
	})}
	q := newScopedAssetQuerier([]sqlcgen.ClaimTFTStaticAssetGroupsRow{{
		SnapshotID: 11, AssetKey: "oversized",
		SourceUrl: "https://assets.example/oversized.png", RelativePath: "oversized.png",
	}})

	result, err := DownloadAssetsForSnapshots(context.Background(), q, t.TempDir(), client, "owner", DownloadInput{
		SnapshotIDs: []int64{11}, BatchSize: 8, Concurrency: 2,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), result.Failed)
	require.Equal(t, int64(1), result.Remaining)
	require.Equal(t, "failed", q.status[11])
}

func TestLegacyDownloadAssetsSkipsKnownMissingQueueIcon(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return staticHTTPResponse(req, http.StatusForbidden, "missing"), nil
	})}
	q := &legacyAssetQuerier{job: sqlcgen.TftStaticAssetJob{
		SnapshotID: 11, AssetKey: "queue-icon",
		SourceUrl:    "https://ddragon.example/cdn/16.17.1/img/tft-queue-type/TFTM_ModeIcon_CT.png",
		RelativePath: "legacy/queue.png",
	}}
	root := t.TempDir()

	result, err := DownloadAssets(context.Background(), q, root, client, "owner", 20)

	require.NoError(t, err)
	require.True(t, q.skipped)
	require.False(t, result.HasMore)
	file, openErr := os.Open(filepath.Join(root, "legacy/queue.png"))
	require.NoError(t, openErr)
	defer file.Close()
	_, decodeErr := png.Decode(file)
	require.NoError(t, decodeErr)
}

func staticHTTPResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     strconv.Itoa(status) + " " + http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

type scopedAssetQuerier struct {
	mu       sync.Mutex
	jobs     []sqlcgen.ClaimTFTStaticAssetGroupsRow
	status   map[int64]string
	claim    sqlcgen.ClaimTFTStaticAssetGroupsParams
	released bool
}

func newScopedAssetQuerier(jobs []sqlcgen.ClaimTFTStaticAssetGroupsRow) *scopedAssetQuerier {
	status := make(map[int64]string, len(jobs))
	for _, job := range jobs {
		status[job.SnapshotID] = "pending"
	}
	return &scopedAssetQuerier{jobs: jobs, status: status}
}

func (q *scopedAssetQuerier) ClaimTFTStaticAssetGroups(_ context.Context, arg sqlcgen.ClaimTFTStaticAssetGroupsParams) ([]sqlcgen.ClaimTFTStaticAssetGroupsRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.claim = arg
	for _, job := range q.jobs {
		q.status[job.SnapshotID] = "leased"
	}
	return append([]sqlcgen.ClaimTFTStaticAssetGroupsRow(nil), q.jobs...), nil
}

func (q *scopedAssetQuerier) CompleteTFTStaticAssetGroup(_ context.Context, arg sqlcgen.CompleteTFTStaticAssetGroupParams) (int64, error) {
	return q.finish(arg.AssetKey, "completed"), nil
}

func (q *scopedAssetQuerier) FailTFTStaticAssetGroup(_ context.Context, arg sqlcgen.FailTFTStaticAssetGroupParams) (int64, error) {
	return q.finish(arg.AssetKey, "failed"), nil
}

func (q *scopedAssetQuerier) SkipTFTStaticAssetGroup(_ context.Context, arg sqlcgen.SkipTFTStaticAssetGroupParams) (int64, error) {
	return q.finish(arg.AssetKey, "skipped"), nil
}

func (q *scopedAssetQuerier) finish(assetKey, status string) int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	var rows int64
	for _, job := range q.jobs {
		if job.AssetKey == assetKey && q.status[job.SnapshotID] == "leased" {
			q.status[job.SnapshotID] = status
			rows++
		}
	}
	return rows
}

func (q *scopedAssetQuerier) ReleaseTFTStaticAssetLeases(context.Context, *string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.released = true
	return nil
}

func (q *scopedAssetQuerier) GetTFTStaticAssetQueueStateForSnapshots(context.Context, []int64) (sqlcgen.GetTFTStaticAssetQueueStateForSnapshotsRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	state := sqlcgen.GetTFTStaticAssetQueueStateForSnapshotsRow{Total: int64(len(q.jobs))}
	for _, status := range q.status {
		switch status {
		case "completed":
			state.Completed++
		case "skipped":
			state.Skipped++
		case "failed":
			state.Failed++
			state.Remaining++
		default:
			state.Remaining++
		}
	}
	return state, nil
}

type legacyAssetQuerier struct {
	job     sqlcgen.TftStaticAssetJob
	skipped bool
}

func (q *legacyAssetQuerier) ClaimTFTStaticAssetJobs(context.Context, *string, int32, int32) ([]sqlcgen.TftStaticAssetJob, error) {
	return []sqlcgen.TftStaticAssetJob{q.job}, nil
}

func (*legacyAssetQuerier) CompleteTFTStaticAssetJob(context.Context, sqlcgen.CompleteTFTStaticAssetJobParams) (int64, error) {
	return 0, nil
}

func (*legacyAssetQuerier) FailTFTStaticAssetJob(context.Context, sqlcgen.FailTFTStaticAssetJobParams) (int64, error) {
	return 0, nil
}

func (q *legacyAssetQuerier) SkipTFTStaticAssetJob(context.Context, sqlcgen.SkipTFTStaticAssetJobParams) (int64, error) {
	q.skipped = true
	return 1, nil
}

func (*legacyAssetQuerier) ReleaseTFTStaticAssetLeases(context.Context, *string) error {
	return nil
}

func (*legacyAssetQuerier) GetTFTStaticAssetQueueState(context.Context) (sqlcgen.GetTFTStaticAssetQueueStateRow, error) {
	return sqlcgen.GetTFTStaticAssetQueueStateRow{}, nil
}
