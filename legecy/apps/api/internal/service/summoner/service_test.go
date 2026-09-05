package summoner

import (
	"testing"
	"time"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
	"github.com/stretchr/testify/require"
)

func TestValidateIdentityNormalizesRegionAndWhitespace(t *testing.T) {
	got, err := validateIdentity(Identity{Region: " kr ", GameName: " Hide on bush ", TagLine: " KR1 "})
	require.NoError(t, err)
	require.Equal(t, Identity{Region: "KR", GameName: "Hide on bush", TagLine: "KR1"}, got)
}

func TestValidateIdentityRejectsUnsupportedRegion(t *testing.T) {
	_, err := validateIdentity(Identity{Region: "EUW1", GameName: "Player", TagLine: "EUW"})
	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	require.Equal(t, "region", validationErr.Field)
}

func TestHistoryCursorRoundTrip(t *testing.T) {
	want := historyCursor{Time: time.Date(2026, 8, 26, 10, 30, 0, 0, time.UTC), MatchID: "KR_123", Seen: 20}
	got, err := decodeCursor(encodeCursor(want))
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestHistoryCursorRejectsMalformedAndOutOfRangeValues(t *testing.T) {
	_, err := decodeCursor("not-base64")
	require.Error(t, err)

	invalid := encodeCursor(historyCursor{Time: time.Now(), MatchID: "KR_123", Seen: MaxHistoryItems + 1})
	_, err = decodeCursor(invalid)
	require.Error(t, err)
}

func TestQueueFromID(t *testing.T) {
	require.Equal(t, QueueDraft, queueFromID(400))
	require.Equal(t, QueueRankedSolo, queueFromID(420))
	require.Equal(t, QueueRankedFlex, queueFromID(440))
	require.Equal(t, QueueSwiftplay, queueFromID(480))
}

func TestWithinRefreshWindow(t *testing.T) {
	now := time.Date(2026, 8, 26, 21, 30, 0, 0, time.UTC)
	require.True(t, withinRefreshWindow(now, now.Add(-5*time.Minute), 5*time.Minute))
	require.False(t, withinRefreshWindow(now, now.Add(-5*time.Minute-time.Nanosecond), 5*time.Minute))
	require.False(t, withinRefreshWindow(now, now.Add(time.Second), 5*time.Minute))
}

func TestMapProfileSupportsImportedPlayerWithoutRefreshedProfile(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	svc := &Service{cfg: Config{Freshness: 5 * time.Minute}, now: func() time.Time { return now }}

	got := svc.mapProfile(sqlcgen.GetSummonerByIdentityRow{
		Region: "NA1", GameName: testPtr("M1d Gaap"), TagLine: testPtr("NA1"),
	})

	require.Equal(t, "NA1", got.Region)
	require.Equal(t, "M1d Gaap", got.GameName)
	require.Equal(t, "NA1", got.TagLine)
	require.Zero(t, got.ProfileIconID)
	require.Zero(t, got.SummonerLevel)
	require.Nil(t, got.LastRefreshedAt)
	require.True(t, got.IsStale)
}

func TestMapParticipantMapsDetailsAndCurrentPlayer(t *testing.T) {
	row := sqlcgen.ListSummonerMatchParticipantsRow{
		MatchID: "KR_123", ParticipantID: testPtr(int32(3)), Puuid: testPtr("current-puuid"),
		GameName: testPtr("Hide on bush"), TagLine: testPtr("KR1"), TeamID: testPtr(int32(100)),
		TierAtMatch: testPtr("MASTER"), DivisionAtMatch: testPtr("I"), LpAtMatch: testPtr(int32(321)),
		TierSnapshotDeltaH: testPtr(int32(6)),
		TeamPosition:       testPtr("MIDDLE"), Win: testPtr(true), ChampionID: testPtr(int32(7)),
		ChampionName: testPtr("LeBlanc"), ChampLevel: testPtr(int32(18)),
		Kills: testPtr(int32(10)), Deaths: testPtr(int32(2)), Assists: testPtr(int32(8)),
		TotalMinionsKilled: testPtr(int32(220)), NeutralMinionsKilled: testPtr(int32(12)),
		GoldEarned: testPtr(int32(14500)), TotalDamageDealtToChampions: testPtr(int32(30123)),
		VisionScore: testPtr(int32(28)), Item0: testPtr(int32(3089)), Item1: testPtr(int32(3020)),
		Summoner1ID: testPtr(int32(4)), Summoner2ID: testPtr(int32(12)),
		Style0: testPtr(int32(8100)), Style1: testPtr(int32(8300)), Perk0: testPtr(int32(8112)),
	}

	got := mapParticipant(row, "current-puuid")

	require.True(t, got.IsCurrentPlayer)
	require.Equal(t, "Hide on bush", got.GameName)
	require.Equal(t, "KR1", got.TagLine)
	require.Equal(t, "MIDDLE", got.Position)
	require.Equal(t, 9.0, got.KDA)
	require.Equal(t, 232, got.MinionsKilled)
	require.Equal(t, []int{3089, 3020, 0, 0, 0, 0, 0}, got.ItemIDs)
	require.Equal(t, []int{4, 12}, got.SummonerSpellIDs)
	require.Equal(t, []int{8112, 0, 0, 0, 0, 0}, got.PerkIDs)
	require.Equal(t, &ParticipantRank{
		Tier: "MASTER", Division: testPtr("I"), LeaguePoints: testPtr(321), SnapshotDeltaHours: testPtr(6),
	}, got.Rank)
}

func TestMapMatchIncludesAverageTierCoverage(t *testing.T) {
	got := mapMatch(sqlcgen.ListSummonerMatchesRow{
		MatchID: "KR_123", Version: "16.17", AvgTier: testPtr("EMERALD"),
		AvgDivision: testPtr("II"), TierCoverage: testPtr(int16(8)),
	})

	require.Equal(t, testPtr("EMERALD"), got.AverageTier)
	require.Equal(t, testPtr("II"), got.AverageDivision)
	require.Equal(t, 8, got.TierCoverage)
}

func testPtr[T any](value T) *T { return &value }
