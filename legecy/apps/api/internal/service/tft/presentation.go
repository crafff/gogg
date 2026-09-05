package tft

import (
	"fmt"
	"sort"
	"strings"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

const (
	TFTItemClassStandard = "STANDARD"
	TFTItemClassEmblem   = "EMBLEM"
	TFTItemClassArtifact = "ARTIFACT"
	TFTItemClassUnknown  = "UNKNOWN"

	TFTSignalEmblem    = "EMBLEM"
	TFTSignalArtifact  = "ARTIFACT"
	TFTSignalThreeStar = "THREE_STAR"
	TFTSignalAugment   = "AUGMENT"
)

// itemClass uses versioned, structured Riot object-id namespaces only. Names
// are localized and unstable, so they are deliberately excluded.
func itemClass(kind, id string) string {
	if kind != "item" {
		return TFTItemClassUnknown
	}
	for _, prefix := range []string{
		"DA_18_Emblem", "TFT18_Item_Emblem", "TFT_Item_Emblem",
	} {
		if strings.HasPrefix(id, prefix) {
			return TFTItemClassEmblem
		}
	}
	for _, prefix := range []string{
		"DA_Artifact_", "TFT_Item_Artifact_", "TFT4_Item_Ornn",
	} {
		if strings.HasPrefix(id, prefix) {
			return TFTItemClassArtifact
		}
	}
	return TFTItemClassStandard
}

func applyPlannerUnits(entities map[string]Entity, rows []sqlcgen.ListTFTTeamPlannerUnitsRow) {
	for _, row := range rows {
		entity, ok := entities[row.CharacterID]
		if !ok {
			entity = Entity{ID: row.CharacterID, ItemClass: TFTItemClassUnknown}
		}
		plannerCode := int(row.PlannerCode)
		entity.PlannerCode = &plannerCode
		if entity.Cost == nil && row.Cost != nil {
			cost := int(*row.Cost)
			entity.Cost = &cost
		}
		entities[row.CharacterID] = entity
	}
}

func sortEntitiesByCost(entities []Entity) {
	sort.SliceStable(entities, func(i, j int) bool {
		left, right := 99, 99
		if entities[i].Cost != nil {
			left = *entities[i].Cost
		}
		if entities[j].Cost != nil {
			right = *entities[j].Cost
		}
		if left != right {
			return left < right
		}
		return entities[i].ID < entities[j].ID
	})
}

func encodeTeamPlannerCode(setNumber int, units []Entity) *string {
	if setNumber <= 0 || len(units) == 0 || len(units) > 10 {
		return nil
	}
	parts := make([]string, 0, 10)
	for _, unit := range units {
		if unit.PlannerCode == nil || *unit.PlannerCode <= 0 || *unit.PlannerCode > 0xfff {
			return nil
		}
		parts = append(parts, fmt.Sprintf("%03x", *unit.PlannerCode))
	}
	for len(parts) < 10 {
		parts = append(parts, "000")
	}
	code := "02" + strings.Join(parts, "") + fmt.Sprintf("TFTSet%d", setNumber)
	return &code
}

// observedSignals reports high-frequency co-occurrence within a lineup. It is
// deliberately not named a dependency or strong association: the observed
// match facts do not record offer availability and this preview has no
// population baseline, lift estimate, or causal comparison.
func observedSignals(lineup Lineup) []Signal {
	signals := []Signal{}
	for _, unit := range lineup.UnitItems {
		for _, item := range unit.CommonItems {
			if item.Count < 20 || item.Rate < 0.25 {
				continue
			}
			kind := ""
			switch item.Entity.ItemClass {
			case TFTItemClassEmblem:
				kind = TFTSignalEmblem
			case TFTItemClassArtifact:
				kind = TFTSignalArtifact
			}
			if kind != "" {
				holder := unit.Unit
				signals = append(signals, Signal{
					Kind: kind, Entity: item.Entity, HolderUnit: &holder,
					SampleSize: item.Count, Rate: item.Rate,
				})
			}
		}
		if unit.CoreRank == nil || unit.StarCoverage < 0.90 {
			continue
		}
		for _, bucket := range unit.StarDistribution {
			if bucket.Stars == 3 && bucket.SampleSize >= 20 && bucket.Rate >= 0.30 {
				signals = append(signals, Signal{
					Kind: TFTSignalThreeStar, Entity: unit.Unit,
					SampleSize: bucket.SampleSize, Rate: bucket.Rate,
				})
			}
		}
	}
	for _, augment := range lineup.CommonAugments {
		if augment.Count >= 20 && augment.Rate >= 0.25 {
			signals = append(signals, Signal{
				Kind: TFTSignalAugment, Entity: augment.Entity,
				SampleSize: augment.Count, Rate: augment.Rate,
			})
		}
	}
	sort.SliceStable(signals, func(i, j int) bool {
		if signals[i].Rate != signals[j].Rate {
			return signals[i].Rate > signals[j].Rate
		}
		if signals[i].Kind != signals[j].Kind {
			return signals[i].Kind < signals[j].Kind
		}
		return signals[i].Entity.ID < signals[j].Entity.ID
	})
	return signals
}
