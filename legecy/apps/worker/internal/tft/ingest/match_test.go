package ingest

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/crafff/gogg/packages/riotapi"
)

func TestLineupSignatureUsesPurchasableMappedUnitsOnly(t *testing.T) {
	catalog := Catalog{
		"TFT15_A":     {Purchasable: true, Cost: 1},
		"TFT15_B":     {Purchasable: true, Cost: 5},
		"TFT15_Dummy": {Purchasable: false, Cost: 0},
	}
	count, signature := lineup([]riotapi.TFTUnitDTO{
		{CharacterID: "TFT15_B"}, {CharacterID: "TFT15_Dummy"},
		{CharacterID: "TFT15_A"}, {CharacterID: "Unknown"},
	}, catalog)
	require.Equal(t, 2, count)
	require.Equal(t, "TFT15_A|TFT15_B", signature)
}

func TestMatchEligibilityRejectsAbnormalLobby(t *testing.T) {
	info := riotapi.TFTMatchInfoDTO{QueueID: StandardRankedQueueID}
	for placement := 1; placement <= 8; placement++ {
		info.Participants = append(info.Participants, riotapi.TFTParticipantDTO{Puuid: string(rune('a' + placement)), Placement: placement})
	}
	eligible, reason := matchEligibility(info)
	require.True(t, eligible)
	require.Empty(t, reason)
	info.Participants[7].Placement = 7
	eligible, reason = matchEligibility(info)
	require.False(t, eligible)
	require.Equal(t, "duplicate_placement", reason)
}

func TestNormalizePatch(t *testing.T) {
	require.Equal(t, "16.17", NormalizePatch("Version 16.17.702.1234"))
	require.Equal(t, "16.17", NormalizePatch("16.17.1"))
}

func TestApplyTargetPatchScopeExcludesOtherPatch(t *testing.T) {
	eligible, reason := applyTargetPatchScope(true, "", "16.16", "16.17")
	require.False(t, eligible)
	require.Equal(t, "out_of_scope_patch", reason)

	eligible, reason = applyTargetPatchScope(true, "", "16.17", "16.17")
	require.True(t, eligible)
	require.Empty(t, reason)
}
