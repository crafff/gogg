package riotapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQuotaScopeSharesProductApplicationBudget(t *testing.T) {
	lol, err := quotaScopeForRequest("secret", "https://asia.api.riotgames.com/lol/match/v5/matches/KR_1")
	require.NoError(t, err)
	tft, err := quotaScopeForRequest("secret", "https://asia.api.riotgames.com/tft/match/v1/matches/KR_1")
	require.NoError(t, err)
	require.Equal(t, lol.Credential, tft.Credential)
	require.Equal(t, lol.Route, tft.Route)
	require.Equal(t, lol.Family, tft.Family)
	require.NotEqual(t, lol.Method, tft.Method)
}

func TestQuotaScopeSeparatesMatchMethods(t *testing.T) {
	list, err := quotaScopeForRequest("secret", "https://asia.api.riotgames.com/lol/match/v5/matches/by-puuid/p/ids")
	require.NoError(t, err)
	detail, err := quotaScopeForRequest("secret", "https://asia.api.riotgames.com/lol/match/v5/matches/KR_1")
	require.NoError(t, err)
	timeline, err := quotaScopeForRequest("secret", "https://asia.api.riotgames.com/lol/match/v5/matches/KR_1/timeline")
	require.NoError(t, err)
	require.NotEqual(t, list.Method, detail.Method)
	require.NotEqual(t, detail.Method, timeline.Method)
}

func TestParseRateWindowsKeepsHeadroomAndCounts(t *testing.T) {
	windows := parseRateWindows("20:1,100:120", "7:1,42:120")
	require.Len(t, windows, 2)
	got := map[time.Duration]rateWindow{}
	for _, window := range windows {
		got[window.Period] = window
	}
	require.Equal(t, 18, got[time.Second].Limit)
	require.Equal(t, 7, got[time.Second].Count)
	require.Equal(t, 90, got[120*time.Second].Limit)
	require.Equal(t, 42, got[120*time.Second].Count)
}

func TestParseRateWindowsRequiresMatchingCounts(t *testing.T) {
	require.Empty(t, parseRateWindows("500:10,30000:600", ""))
	require.Empty(t, parseRateWindows("500:10", "7:1"))
}

func TestNormalizedQuotaProduct(t *testing.T) {
	require.Equal(t, "tft", normalizedQuotaProduct("TFT"))
	require.Equal(t, "lol", normalizedQuotaProduct("lol"))
	require.Equal(t, "lol", normalizedQuotaProduct("unknown"))
}

func TestQuotaShareLimit(t *testing.T) {
	require.Equal(t, 60, quotaShareLimit(90, 2, 3))
	require.Equal(t, 30, quotaShareLimit(90, 1, 3))
	require.Equal(t, 90, quotaShareLimit(90, 1, 1))
	require.Equal(t, 1, quotaShareLimit(1, 1, 3))
}
