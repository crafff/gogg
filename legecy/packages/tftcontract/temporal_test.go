package tftcontract

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/converter"
)

func TestCrawlStatusAdditiveFieldsDecodeOldPayload(t *testing.T) {
	type oldCrawlStatus struct {
		State        string
		Stage        string
		RunID        int64
		PausePending bool
	}
	payloads, err := converter.GetDefaultDataConverter().ToPayloads(oldCrawlStatus{
		State: "running", Stage: "match_detail", RunID: 42, PausePending: true,
	})
	require.NoError(t, err)

	var decoded CrawlStatus
	require.NoError(t, converter.GetDefaultDataConverter().FromPayloads(payloads, &decoded))
	require.Equal(t, "running", decoded.State)
	require.Equal(t, "match_detail", decoded.Stage)
	require.Equal(t, int64(42), decoded.RunID)
	require.True(t, decoded.PausePending)
	require.Zero(t, decoded.StageCompleted)
	require.Zero(t, decoded.StageTotal)
}
