package riotapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type captureRecorder struct {
	called bool
	body   []byte
}

func (r *captureRecorder) Record(_ context.Context, _ ResponseMeta, body []byte) error {
	r.called = true
	r.body = append([]byte(nil), body...)
	return nil
}

func TestTFTMatchArchivesBeforeDecode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"metadata":{"match_id":"KR_1"},"info":`))
	}))
	defer server.Close()

	recorder := &captureRecorder{}
	client := NewClient("key", server.URL, server.URL)
	client.SetResponseRecorder("KR", recorder)
	_, err := client.GetTFTMatchDetail(context.Background(), "KR_1")
	require.Error(t, err)
	require.True(t, recorder.called)
	require.NotEmpty(t, recorder.body)
}

func TestTFTMatchDecodesUnknownFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"metadata":{"data_version":"5","match_id":"KR_1","participants":["p1"]},"info":{"queue_id":1100,"tft_set_number":15,"unknown":true,"participants":[{"puuid":"p1","placement":1,"augments":["A"],"traits":[],"units":[]}]}}`))
	}))
	defer server.Close()

	client := NewClient("key", server.URL, server.URL)
	got, err := client.GetTFTMatchDetail(context.Background(), "KR_1")
	require.NoError(t, err)
	require.Equal(t, 1100, got.Info.QueueID)
	require.Equal(t, 1, got.Info.Participants[0].Placement)
}
