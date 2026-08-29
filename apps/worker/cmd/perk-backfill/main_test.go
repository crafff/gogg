package main

import (
	"fmt"
	"testing"

	"github.com/crafff/gogg/packages/riotapi"
	"github.com/stretchr/testify/require"
)

func TestPerkRowsBuildsTenCompleteRows(t *testing.T) {
	detail := &riotapi.MatchDetailDTO{}
	detail.Metadata.MatchID = "KR_1"
	for i := 0; i < 10; i++ {
		p := riotapi.ParticipantDTO{Puuid: fmt.Sprintf("p%d", i)}
		p.Perks.Styles = []riotapi.PerkStyleDTO{
			{Style: 8000, Selections: []riotapi.PerkSelectionDTO{{Perk: 1}, {Perk: 2}, {Perk: 3}, {Perk: 4}}},
			{Style: 8300, Selections: []riotapi.PerkSelectionDTO{{Perk: 5}, {Perk: 6}}},
		}
		detail.Info.Participants = append(detail.Info.Participants, p)
	}
	rows, err := perkRows("KR_1", detail)
	require.NoError(t, err)
	require.Len(t, rows, 10)
	require.Equal(t, [6]int{1, 2, 3, 4, 5, 6}, rows[0].Perk)
}

func TestPerkRowsRejectsIncompleteResponse(t *testing.T) {
	detail := &riotapi.MatchDetailDTO{Info: riotapi.InfoDTO{Participants: make([]riotapi.ParticipantDTO, 10)}}
	_, err := perkRows("KR_1", detail)
	require.ErrorContains(t, err, "empty puuid")
}
