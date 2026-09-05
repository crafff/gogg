package analytics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClusterUsesFrozenJaccardAndCoreThresholds(t *testing.T) {
	exacts := []*Exact{
		{Signature: "a", Units: []string{"A", "B", "C", "D", "E", "F"}, Sample: 60, Lobbies: map[string]bool{"1": true}, LobbyCounts: map[string]int{"1": 60}, Items: map[string]int{}, Augments: map[string]int{}, Traits: map[string]int{}},
		{Signature: "b", Units: []string{"A", "B", "C", "D", "E", "X"}, Sample: 40, Lobbies: map[string]bool{"2": true}, LobbyCounts: map[string]int{"2": 40}, Items: map[string]int{}, Augments: map[string]int{}, Traits: map[string]int{}},
		{Signature: "c", Units: []string{"A", "B", "Y", "Z", "Q", "R"}, Sample: 20, Lobbies: map[string]bool{"3": true}, LobbyCounts: map[string]int{"3": 20}, Items: map[string]int{}, Augments: map[string]int{}, Traits: map[string]int{}},
	}
	families := Cluster(exacts)
	require.Len(t, families, 2)
	require.Equal(t, []string{"A", "B", "C", "D", "E", "F"}, families[0].CoreUnits)
}

func TestBuildExactCountsContestedPerLobby(t *testing.T) {
	observations := []Observation{{MatchID: "m", Puuid: "1", Signature: "s", Placement: 1, Units: []string{"A"}}, {MatchID: "m", Puuid: "2", Signature: "s", Placement: 4, Units: []string{"A"}}}
	exact := BuildExact(observations)[0]
	require.Equal(t, 2, exact.Sample)
	require.Equal(t, 2, exact.Contested)
	require.Equal(t, 1, exact.First)
	require.Equal(t, 2, exact.Top4)
}
