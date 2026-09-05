package tft

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeTeamPlannerCodeUsesVersionTwoThreeHexSlots(t *testing.T) {
	codes := []int{0x3f7, 0x429, 0x420, 0x400, 0x437, 0x428, 0x412, 0x42c}
	units := make([]Entity, 0, len(codes))
	for index := range codes {
		code := codes[index]
		units = append(units, Entity{ID: string(rune('A' + index)), PlannerCode: &code})
	}

	got := encodeTeamPlannerCode(18, units)

	require.NotNil(t, got)
	require.Equal(t, "023f742942040043742841242c000000TFTSet18", *got)
}

func TestEncodeTeamPlannerCodeRejectsMissingZeroOrOversizedRoster(t *testing.T) {
	require.Nil(t, encodeTeamPlannerCode(18, []Entity{{ID: "missing"}}))
	zero := 0
	require.Nil(t, encodeTeamPlannerCode(18, []Entity{{ID: "zero", PlannerCode: &zero}}))
	valid := 1
	tooMany := make([]Entity, 11)
	for index := range tooMany {
		tooMany[index] = Entity{ID: "unit", PlannerCode: &valid}
	}
	require.Nil(t, encodeTeamPlannerCode(18, tooMany))
}

func TestItemClassUsesStructuredNamespaces(t *testing.T) {
	require.Equal(t, TFTItemClassEmblem, itemClass("item", "DA_18_EmblemBrawler"))
	require.Equal(t, TFTItemClassArtifact, itemClass("item", "DA_Artifact_NavoriFlickerblade"))
	require.Equal(t, TFTItemClassStandard, itemClass("item", "DA_GuinsoosRageblade"))
	require.Equal(t, TFTItemClassUnknown, itemClass("augment", "DA_Artifact_NameOnly"))
}

func TestObservedSignalsRequireCoverageSupportAndPrevalence(t *testing.T) {
	coreRank := 1
	lineup := Lineup{
		UnitItems: []UnitItems{{
			Unit: Entity{ID: "unit"}, CoreRank: &coreRank, StarCoverage: 0.95,
			CommonItems: []Count{{
				Entity: Entity{ID: "artifact", ItemClass: TFTItemClassArtifact},
				Count:  25, Rate: 0.50,
			}},
			StarDistribution: []UnitStarStrength{{Stars: 3, SampleSize: 20, Rate: 0.40}},
		}},
	}

	signals := observedSignals(lineup)

	require.Len(t, signals, 2)
	require.Equal(t, TFTSignalArtifact, signals[0].Kind)
	require.Equal(t, TFTSignalThreeStar, signals[1].Kind)
}

func TestSortEntitiesByCostKeepsUnknownLast(t *testing.T) {
	one, four := 1, 4
	entities := []Entity{
		{ID: "unknown"},
		{ID: "four", Cost: &four},
		{ID: "one", Cost: &one},
	}

	sortEntitiesByCost(entities)

	require.Equal(t, []string{"one", "four", "unknown"}, []string{
		entities[0].ID, entities[1].ID, entities[2].ID,
	})
}
