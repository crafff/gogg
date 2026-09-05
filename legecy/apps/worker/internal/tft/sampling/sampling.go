package sampling

import (
	"crypto/sha256"
	"sort"
	"strings"
	"time"
)

type Seed struct {
	ID           int64
	Puuid        string
	Tier         string
	Division     string
	LeaguePoints int
}

type Limits struct {
	Master             int
	DiamondPerDivision int
}

func EffectiveLimits(patchAge time.Duration, observations int, base, scaled Limits, scaleAfter time.Duration, scaleBelow int) Limits {
	if patchAge >= scaleAfter && observations < scaleBelow {
		return scaled
	}
	return base
}

// Select implements the frozen high-tier stratification: Challenger and GM
// full, Master evenly across LP deciles, Diamond independently per division.
func Select(seeds []Seed, salt string, limits Limits) map[int64]bool {
	selected := make(map[int64]bool)
	var master []Seed
	diamond := make(map[string][]Seed)
	for _, seed := range seeds {
		switch strings.ToUpper(seed.Tier) {
		case "CHALLENGER", "GRANDMASTER":
			selected[seed.ID] = true
		case "MASTER":
			master = append(master, seed)
		case "DIAMOND":
			diamond[strings.ToUpper(seed.Division)] = append(diamond[strings.ToUpper(seed.Division)], seed)
		}
	}
	sort.Slice(master, func(i, j int) bool {
		if master[i].LeaguePoints != master[j].LeaguePoints {
			return master[i].LeaguePoints > master[j].LeaguePoints
		}
		return master[i].Puuid < master[j].Puuid
	})
	for decile := 0; decile < 10; decile++ {
		start, end := len(master)*decile/10, len(master)*(decile+1)/10
		chooseByHash(master[start:end], salt, (limits.Master+9)/10, selected)
	}
	for _, divisionSeeds := range diamond {
		chooseByHash(divisionSeeds, salt, limits.DiamondPerDivision, selected)
	}
	return selected
}

func chooseByHash(seeds []Seed, salt string, limit int, selected map[int64]bool) {
	if limit <= 0 {
		return
	}
	sort.Slice(seeds, func(i, j int) bool {
		a := sha256.Sum256([]byte(seeds[i].Puuid + salt))
		b := sha256.Sum256([]byte(seeds[j].Puuid + salt))
		return string(a[:]) < string(b[:])
	})
	if limit > len(seeds) {
		limit = len(seeds)
	}
	for _, seed := range seeds[:limit] {
		selected[seed.ID] = true
	}
}
