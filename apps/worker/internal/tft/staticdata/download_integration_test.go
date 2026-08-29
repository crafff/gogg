package staticdata

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

func TestAssetFailureBackoffAndExhaustedRequeue(t *testing.T) {
	dsn := os.Getenv("GOGG_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set GOGG_TEST_DATABASE_DSN to run the PostgreSQL queue test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	q := sqlcgen.New(tx)
	unique := fmt.Sprintf("backoff-%d", time.Now().UnixNano())
	snapshot, err := q.CreateTFTStaticSnapshot(ctx, sqlcgen.CreateTFTStaticSnapshotParams{
		Source: "ddragon", Patch: unique, Build: unique, Revision: unique,
		Locale: "en_us", SourceUrl: "https://assets.example/catalog.json", ParserVersion: parserVersion,
	})
	require.NoError(t, err)
	_, err = q.EnqueueTFTStaticAsset(ctx, sqlcgen.EnqueueTFTStaticAssetParams{
		SnapshotID: snapshot.ID, AssetKey: "backoff-asset",
		SourceUrl: "https://assets.example/asset.png", RelativePath: "test/asset.png",
	})
	require.NoError(t, err)

	owner := "backoff-owner"
	claimed, err := q.ClaimTFTStaticAssetGroups(ctx, sqlcgen.ClaimTFTStaticAssetGroupsParams{
		SnapshotIds: []int64{snapshot.ID}, RowLimit: 1, LeaseOwner: &owner, LeaseSeconds: 660,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	require.Equal(t, int32(1), claimed[0].Attempt)
	message := "temporary 503"
	rows, err := q.FailTFTStaticAssetGroup(ctx, sqlcgen.FailTFTStaticAssetGroupParams{
		LastError: &message, SnapshotIds: []int64{snapshot.ID}, AssetKey: "backoff-asset", LeaseOwner: &owner,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	var retryAfter time.Time
	err = tx.QueryRow(ctx, `SELECT lease_expires_at FROM tft_static_asset_jobs WHERE snapshot_id=$1 AND asset_key=$2`, snapshot.ID, "backoff-asset").Scan(&retryAfter)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(5*time.Second), retryAfter, 2*time.Second)
	claimed, err = q.ClaimTFTStaticAssetGroups(ctx, sqlcgen.ClaimTFTStaticAssetGroupsParams{
		SnapshotIds: []int64{snapshot.ID}, RowLimit: 1, LeaseOwner: &owner, LeaseSeconds: 660,
	})
	require.NoError(t, err)
	require.Empty(t, claimed, "a failed asset must not be reclaimed before its retry time")
	queue, err := q.GetTFTStaticAssetQueueStateForSnapshots(ctx, []int64{snapshot.ID})
	require.NoError(t, err)
	require.True(t, queue.NextEligibleAt.Valid)
	require.WithinDuration(t, retryAfter, queue.NextEligibleAt.Time, time.Second)

	_, err = tx.Exec(ctx, `UPDATE tft_static_asset_jobs SET status='failed', attempt=5, lease_owner='stale-owner', lease_expires_at=now()+interval '1 hour' WHERE snapshot_id=$1 AND asset_key=$2`, snapshot.ID, "backoff-asset")
	require.NoError(t, err)
	_, err = q.EnqueueTFTStaticAsset(ctx, sqlcgen.EnqueueTFTStaticAssetParams{
		SnapshotID: snapshot.ID, AssetKey: "backoff-asset",
		SourceUrl: "https://assets.example/asset.png", RelativePath: "test/asset.png",
	})
	require.NoError(t, err)
	var status string
	var attempt int32
	var leaseOwner *string
	var leaseExpires *time.Time
	err = tx.QueryRow(ctx, `SELECT status, attempt, lease_owner, lease_expires_at FROM tft_static_asset_jobs WHERE snapshot_id=$1 AND asset_key=$2`, snapshot.ID, "backoff-asset").Scan(&status, &attempt, &leaseOwner, &leaseExpires)
	require.NoError(t, err)
	require.Equal(t, "pending", status)
	require.Zero(t, attempt)
	require.Nil(t, leaseOwner)
	require.Nil(t, leaseExpires)
	claimed, err = q.ClaimTFTStaticAssetGroups(ctx, sqlcgen.ClaimTFTStaticAssetGroupsParams{
		SnapshotIds: []int64{snapshot.ID}, RowLimit: 1, LeaseOwner: &owner, LeaseSeconds: 660,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1, "an exhausted asset must be immediately claimable after re-enqueue")
	for range 5 {
		require.NoError(t, q.ReleaseTFTStaticAssetLeases(ctx, &owner))
		err = tx.QueryRow(ctx, `SELECT status, attempt FROM tft_static_asset_jobs WHERE snapshot_id=$1 AND asset_key=$2`, snapshot.ID, "backoff-asset").Scan(&status, &attempt)
		require.NoError(t, err)
		require.Equal(t, "pending", status)
		require.Zero(t, attempt, "releasing unfinished work must not consume an attempt")
		claimed, err = q.ClaimTFTStaticAssetGroups(ctx, sqlcgen.ClaimTFTStaticAssetGroupsParams{
			SnapshotIds: []int64{snapshot.ID}, RowLimit: 1, LeaseOwner: &owner, LeaseSeconds: 660,
		})
		require.NoError(t, err)
		require.Len(t, claimed, 1, "repeated claim and release must remain immediately reclaimable")
		require.Equal(t, int32(1), claimed[0].Attempt)
	}
}

func TestSkippedAssetRequiresLeaseAndAllowsPublication(t *testing.T) {
	dsn := os.Getenv("GOGG_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set GOGG_TEST_DATABASE_DSN to run the PostgreSQL queue test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	q := sqlcgen.New(tx)
	unique := fmt.Sprintf("skip-%d", time.Now().UnixNano())
	snapshot, err := q.CreateTFTStaticSnapshot(ctx, sqlcgen.CreateTFTStaticSnapshotParams{
		Source: "ddragon", Patch: unique, Build: unique, Revision: unique,
		Locale: "en_us", SourceUrl: "https://assets.example/catalog.json", ParserVersion: parserVersion,
	})
	require.NoError(t, err)
	_, err = q.EnqueueTFTStaticAsset(ctx, sqlcgen.EnqueueTFTStaticAssetParams{
		SnapshotID: snapshot.ID, AssetKey: "optional-icon",
		SourceUrl: "https://assets.example/icon.png", RelativePath: "test/icon.png",
	})
	require.NoError(t, err)
	owner := "asset-owner"
	claimed, err := q.ClaimTFTStaticAssetGroups(ctx, sqlcgen.ClaimTFTStaticAssetGroupsParams{
		SnapshotIds: []int64{snapshot.ID}, RowLimit: 1, LeaseOwner: &owner, LeaseSeconds: 660,
	})
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	digest := strings.Repeat("a", 64)
	message := "known optional icon is absent"
	wrongOwner := "wrong-owner"
	rows, err := q.SkipTFTStaticAssetGroup(ctx, sqlcgen.SkipTFTStaticAssetGroupParams{
		LastError: &message, SnapshotIds: []int64{snapshot.ID}, AssetKey: "optional-icon",
		LeaseOwner: &wrongOwner, Sha256: digest,
	})
	require.NoError(t, err)
	require.Zero(t, rows, "a different lease owner must not finalize the asset")
	rows, err = q.SkipTFTStaticAssetGroup(ctx, sqlcgen.SkipTFTStaticAssetGroupParams{
		LastError: &message, SnapshotIds: []int64{snapshot.ID}, AssetKey: "optional-icon",
		LeaseOwner: &owner, Sha256: digest,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)
	rows, err = q.PublishTFTStaticSnapshot(ctx, snapshot.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	var status string
	var assets int
	err = tx.QueryRow(ctx, `SELECT status FROM tft_static_snapshots WHERE id=$1`, snapshot.ID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "published", status)
	err = tx.QueryRow(ctx, `SELECT count(*) FROM tft_static_assets WHERE snapshot_id=$1 AND asset_key=$2`, snapshot.ID, "optional-icon").Scan(&assets)
	require.NoError(t, err)
	require.Equal(t, 1, assets)
}
