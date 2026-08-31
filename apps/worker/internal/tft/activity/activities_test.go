package activity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEffectiveWindowStartUsesPriorWatermarkOverlap(t *testing.T) {
	base := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	previousEnd := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	require.Equal(t, previousEnd.Add(-6*time.Hour), effectiveWindowStart(base, previousEnd, 6*time.Hour))
}

func TestEffectiveWindowStartNeverMovesBeforeBase(t *testing.T) {
	base := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	previousEnd := base.Add(2 * time.Hour)
	require.Equal(t, base, effectiveWindowStart(base, previousEnd, 6*time.Hour))
}

func TestClonePlayerHeartbeatOwnsWorksetSlices(t *testing.T) {
	original := FetchPlayerHistoryHeartbeat{
		Scanned: 2, Fetched: 1,
		MatchIDs:          []string{"KR_1", "KR_2"},
		CompletedMatchIDs: []string{"KR_1"},
	}

	cloned := clonePlayerHeartbeat(original)
	cloned.MatchIDs[0] = "KR_CHANGED"
	cloned.CompletedMatchIDs[0] = "KR_CHANGED"

	require.Equal(t, []string{"KR_1", "KR_2"}, original.MatchIDs)
	require.Equal(t, []string{"KR_1"}, original.CompletedMatchIDs)
}

func TestBalancedSelectionKeyIsStableAndSeparatesRuns(t *testing.T) {
	key := balancedSelectionKey("route-balance-v1", 42, "SEA", "VN2_123")
	require.Equal(t, key, balancedSelectionKey("route-balance-v1", 42, "SEA", "VN2_123"))
	require.NotEqual(t, key, balancedSelectionKey("route-balance-v2", 42, "SEA", "VN2_123"))
	require.NotEqual(t, key, balancedSelectionKey("route-balance-v1", 43, "SEA", "VN2_123"))
	require.NotEqual(t, key, balancedSelectionKey("route-balance-v1", 42, "ASIA", "VN2_123"))
	require.NotEqual(t, key, balancedSelectionKey("route-balance-v1", 42, "SEA", "VN2_124"))
}
