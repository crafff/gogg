package riotapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
)

// CDragonItem represents one item entry from the CommunityDragon items endpoint.
type CDragonItem struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	From       []int    `json:"from"`       // component item IDs required to build this
	To         []int    `json:"to"`         // items this builds into (empty = terminal)
	Categories []string `json:"categories"` // e.g. "Boots", "Damage"
	Price      int      `json:"price"`      // base gold cost
	PriceTotal int      `json:"priceTotal"` // total gold cost including components
	InStore    bool     `json:"inStore"`
}

var cdragnonHTTP = &http.Client{Timeout: 30 * time.Second}

// GetItemCatalog fetches the item list for the given CDragon patch (e.g. "15.1").
func GetItemCatalog(ctx context.Context, patch string) ([]CDragonItem, error) {
	url := fmt.Sprintf(
		"https://raw.communitydragon.org/%s/plugins/rcp-be-lol-game-data/global/default/v1/items.json",
		patch,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := cdragnonHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cdragon items: HTTP %d for patch %s", resp.StatusCode, patch)
	}
	var items []CDragonItem
	return items, sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&items)
}

// ExtractCDragonPatch converts a Riot game version ("15.1.419.9163") to the
// CDragon patch segment ("15.1") used in CDragon URLs.
func ExtractCDragonPatch(gameVersion string) string {
	parts := strings.SplitN(gameVersion, ".", 3)
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return gameVersion
}

// IsCompletedItem returns true when the item is a terminal recipe item
// (has components, nothing builds from it, sufficient value, available in shop).
func IsCompletedItem(item CDragonItem) bool {
	if !item.InStore {
		return false
	}
	if len(item.To) > 0 {
		return false // something else builds from this item
	}
	if len(item.From) == 0 {
		return false // no recipe — basic/starter item
	}
	if item.PriceTotal < 1000 {
		return false
	}
	for _, cat := range item.Categories {
		switch cat {
		case "Consumable", "Trinket", "Vision":
			return false
		}
	}
	return true
}

// IsBoots returns true for completed boots items.
func IsBoots(item CDragonItem) bool {
	for _, cat := range item.Categories {
		if cat == "Boots" {
			return true
		}
	}
	return false
}

// IsSkippable returns true for consumables, trinkets, and vision wards —
// items that should not appear in starter-item or build-path analysis.
func IsSkippable(item CDragonItem) bool {
	for _, cat := range item.Categories {
		switch cat {
		case "Consumable", "Trinket", "Vision":
			return true
		}
	}
	return false
}
