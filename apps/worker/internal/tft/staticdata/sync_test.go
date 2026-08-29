package staticdata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDDragonChampionProducesPurchasableUnitAndIcon(t *testing.T) {
	doc := document{name: "tft-champion.json", kind: "tft-champion", body: []byte(`{"data":{"TFT15_A":{"id":"TFT15_A","name":"A","tier":4,"image":{"group":"tft-champion","full":"A.png"}}}}`)}
	objects, assets, err := parseDocument("ddragon", "16.17.1", doc)
	require.NoError(t, err)
	require.Len(t, objects, 1)
	require.Equal(t, "unit", objects[0].kind)
	require.Equal(t, int32(4), *objects[0].cost)
	require.True(t, *objects[0].purchasable)
	require.Equal(t, []string{"https://ddragon.leagueoflegends.com/cdn/16.17.1/img/tft-champion/A.png"}, assets)
}

func TestParseCDragonRichExtractsCatalogUnit(t *testing.T) {
	doc := document{name: "rich.json", kind: "rich", body: []byte(`{"setData":[{"champions":[{"apiName":"TFT15_A","name":"A","cost":3,"icon":"/lol-game-data/assets/ASSETS/Characters/A.png"}]}]}`)}
	objects, assets, err := parseDocument("cdragon", "16.17", doc)
	require.NoError(t, err)
	require.Len(t, objects, 1)
	require.Equal(t, "unit", objects[0].kind)
	require.Equal(t, int32(3), *objects[0].cost)
	require.Len(t, assets, 1)
	require.Contains(t, assets[0], "/16.17/plugins/")
}

func TestPatchOfPreservesMajorMinor(t *testing.T) {
	require.Equal(t, "16.17", patchOf("16.17.1"))
}

func TestParseDocumentRejectsMalformedOrEmptyPayloads(t *testing.T) {
	for _, body := range []string{`{`, `{"data":{}}`, `{"data":{"TFT15_Internal":{"tier":0}}}`} {
		_, _, err := parseDocument("ddragon", "16.17.1", document{
			name: "tft-champion.json",
			kind: "tft-champion",
			body: []byte(body),
		})
		require.Error(t, err)
	}
}
