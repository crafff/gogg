package phase3

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/crafff/gogg/packages/riotapi"
)

func TestPerkRowFromDTOStoresFourPrimaryAndTwoSecondaryRunes(t *testing.T) {
	perks := riotapi.PerksDTO{}
	perks.StatPerks.Offense = 5005
	perks.StatPerks.Flex = 5008
	perks.StatPerks.Defense = 5001
	perks.Styles = []riotapi.PerkStyleDTO{
		{
			Style: 8000,
			Selections: []riotapi.PerkSelectionDTO{
				{Perk: 8010, Var1: 1, Var2: 2, Var3: 3},
				{Perk: 9111, Var1: 4, Var2: 5, Var3: 6},
				{Perk: 9105, Var1: 7, Var2: 8, Var3: 9},
				{Perk: 8299, Var1: 10, Var2: 11, Var3: 12},
			},
		},
		{
			Style: 8300,
			Selections: []riotapi.PerkSelectionDTO{
				{Perk: 8347, Var1: 13, Var2: 14, Var3: 15},
				{Perk: 8304, Var1: 16, Var2: 17, Var3: 18},
			},
		},
	}

	row := PerkRowFromDTO("KR_1", "puuid", &perks)

	require.Equal(t, "KR_1", row.MatchID)
	require.Equal(t, "puuid", row.PUUID)
	require.Equal(t, 8000, row.Style0)
	require.Equal(t, 8300, row.Style1)
	require.Equal(t, [6]int{8010, 9111, 9105, 8299, 8347, 8304}, row.Perk)
	require.Equal(t, [3]int{10, 11, 12}, row.Vars[3], "the fourth primary rune must not be overwritten")
	require.Equal(t, [3]int{16, 17, 18}, row.Vars[5])
	require.Equal(t, 5005, row.StatOffense)
	require.Equal(t, 5008, row.StatFlex)
	require.Equal(t, 5001, row.StatDefense)
}

func TestPerkRowFromDTOIgnoresSelectionsBeyondStorageCapacity(t *testing.T) {
	perks := riotapi.PerksDTO{Styles: []riotapi.PerkStyleDTO{{
		Style: 8000,
		Selections: []riotapi.PerkSelectionDTO{
			{Perk: 1}, {Perk: 2}, {Perk: 3}, {Perk: 4}, {Perk: 5}, {Perk: 6}, {Perk: 7},
		},
	}}}

	row := PerkRowFromDTO("match", "player", &perks)
	require.Equal(t, [6]int{1, 2, 3, 4, 5, 6}, row.Perk)
}
