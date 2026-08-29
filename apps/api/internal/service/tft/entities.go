package tft

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

func indexLocalizedEntities(rows []sqlcgen.ListTFTLocalizedStaticObjectsRow) map[string]Entity {
	out := make(map[string]Entity, len(rows))
	for _, row := range rows {
		entity := Entity{ID: row.ObjectID, IconURL: localTFTIconURL(row.Payload, row.Patch, row.Revision)}
		if row.Name != nil {
			entity.Name = *row.Name
		}
		out[row.ObjectID] = entity
	}
	return out
}

func localizedEntity(index map[string]Entity, id string) Entity {
	if entity, ok := index[id]; ok {
		return entity
	}
	return Entity{ID: id}
}

// Published snapshots require every asset job to be completed or explicitly
// skipped with a local placeholder. Reproducing the static sync's
// content-addressed path here therefore yields only local /game-assets URLs and
// never a browser dependency on CDragon.
func localTFTIconURL(payload []byte, patch, revision string) string {
	var object map[string]any
	if json.Unmarshal(payload, &object) != nil {
		return ""
	}
	icon := firstString(object, "iconPath", "icon", "icon_path", "squareIconPath")
	if !strings.HasPrefix(strings.ToLower(icon), "/lol-game-data/assets") {
		return ""
	}
	suffix := strings.TrimPrefix(strings.ToLower(icon), "/lol-game-data/assets")
	source := "https://raw.communitydragon.org/" + patch + "/plugins/rcp-be-lol-game-data/global/default" + suffix
	digest := sha256.Sum256([]byte(source))
	key := hex.EncodeToString(digest[:])
	ext := filepath.Ext(strings.Split(source, "?")[0])
	if ext == "" || len(ext) > 6 {
		ext = ".png"
	}
	return "/game-assets/tft/static/cdragon/" + patch + "/" + revision + "/assets/" + key + ext
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}
