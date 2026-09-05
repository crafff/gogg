package rawarchive

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/crafff/gogg/packages/riotapi"
	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

type fakeQuerier struct {
	object  sqlcgen.UpsertTFTRawObjectParams
	capture sqlcgen.InsertTFTRawCaptureParams
}

func (f *fakeQuerier) UpsertTFTRawObject(_ context.Context, arg sqlcgen.UpsertTFTRawObjectParams) error {
	f.object = arg
	return nil
}

func (f *fakeQuerier) InsertTFTRawCapture(_ context.Context, arg sqlcgen.InsertTFTRawCaptureParams) (sqlcgen.TftRawCapture, error) {
	f.capture = arg
	return sqlcgen.TftRawCapture{}, nil
}

func TestArchiveIsContentAddressedAndRunAware(t *testing.T) {
	q := &fakeQuerier{}
	root := t.TempDir()
	archive := New(root, 6, q)
	ctx := WithRunID(context.Background(), 42)
	err := archive.Record(ctx, riotapi.ResponseMeta{
		Region: "KR", Kind: "tft-match-detail", MatchID: "KR_1", ResourceKey: "KR_1",
		RequestURL: "https://asia.api.riotgames.com/tft/match/v1/matches/KR_1",
	}, []byte(`{"ok":true}`))
	require.NoError(t, err)
	require.NotEmpty(t, q.object.Sha256)
	require.Equal(t, int64(42), *q.capture.RunID)
	require.Equal(t, "ASIA", q.capture.RoutingRegion)
	_, err = os.Stat(filepath.Join(root, filepath.FromSlash(q.object.RelativePath)))
	require.NoError(t, err)
}
