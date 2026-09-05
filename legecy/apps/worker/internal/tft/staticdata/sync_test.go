package staticdata

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
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
	doc := document{name: "rich.json", kind: "rich", body: []byte(`{"setData":[{"champions":[{"apiName":"TFT15_A","name":"A","cost":3,"icon":"/lol-game-data/assets/ASSETS/Characters/A.png"}],"traits":[{"apiName":"TFT15_Trait","name":"Trait"}],"items":[{"apiName":"TFT15_Item","name":"Item"}]}]}`)}
	objects, assets, err := parseDocument("cdragon", "16.17", doc)
	require.NoError(t, err)
	require.Len(t, objects, 3)
	byID := objectsByID(objects)
	require.Equal(t, "unit", byID["TFT15_A"].kind)
	require.Equal(t, int32(3), *byID["TFT15_A"].cost)
	require.Equal(t, "trait", byID["TFT15_Trait"].kind)
	require.Equal(t, "item", byID["TFT15_Item"].kind)
	require.Len(t, assets, 1)
	require.Contains(t, assets[0], "/16.17/plugins/")
}

func TestParseCurrentCDragonSetMetadata(t *testing.T) {
	doc := document{name: "tftsets.json", kind: "tftsets", body: []byte(`{
		"LCTFTModeData": {
			"mActiveSets": [
				{"SetName":"TFTSet17","SetDisplayName":"Space Gods"},
				{"SetName":"TFTSet18","SetDisplayName":"Enchanted Wilds"}
			],
			"mDefaultSet":{"SetName":"TFTSet18","SetDisplayName":"Enchanted Wilds"}
		}
	}`)}

	objects, assets, err := parseDocument("cdragon", "16.17", doc)
	require.NoError(t, err)
	require.Empty(t, assets)
	require.Len(t, objects, 2)
	byID := objectsByID(objects)
	require.Equal(t, "set", byID["TFTSet18"].kind)
	require.Equal(t, "Enchanted Wilds", byID["TFTSet18"].name)
}

func TestParseCurrentCDragonClientCatalogs(t *testing.T) {
	tests := []struct {
		name, kind, body, id, objectKind, displayName string
		wantAsset                                     bool
	}{
		{
			name: "champion", kind: "tftchampions", id: "TFT18_A", objectKind: "unit", displayName: "A",
			body: `[{"name":"TFT18_A","character_record":{"character_id":"TFT18_A","display_name":"A","rarity":2,"squareIconPath":"/lol-game-data/assets/ASSETS/Characters/A.png"}}]`, wantAsset: true,
		},
		{
			name: "trait", kind: "tfttraits", id: "TFT18_Trait", objectKind: "trait", displayName: "Trait",
			body: `[{"trait_id":"TFT18_Trait","display_name":"Trait","icon_path":"/lol-game-data/assets/ASSETS/Traits/Trait.png"}]`, wantAsset: true,
		},
		{
			name: "item", kind: "tftitems", id: "TFT18_Item", objectKind: "item", displayName: "Item",
			body: `[{"id":0,"nameId":"TFT18_Item","name":"Item","squareIconPath":"/lol-game-data/assets/ASSETS/Items/Item.png"}]`, wantAsset: true,
		},
		{
			name: "augment", kind: "tftitems", id: "TFT18_Augment_Power", objectKind: "augment", displayName: "Power",
			body: `[{"id":0,"nameId":"TFT18_Augment_Power","name":"Power","squareIconPath":"/lol-game-data/assets/ASSETS/Augments/Power.png"}]`, wantAsset: true,
		},
		{
			name: "augment flag", kind: "tftitems", id: "TFT18_SpecialChoice", objectKind: "augment", displayName: "Choice",
			body: `[{"id":0,"nameId":"TFT18_SpecialChoice","name":"Choice","isAugment":true,"squareIconPath":"/lol-game-data/assets/ASSETS/Augments/Choice.png"}]`, wantAsset: true,
		},
		{
			name: "portal", kind: "tftregionportals", id: "TFT18_Portal", objectKind: "portal", displayName: "Portal",
			body: `[{"nameId":"TFT18_Portal","displayName":"Portal","iconPath":"/lol-game-data/assets/"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects, assets, err := parseDocument("cdragon", "16.17", document{name: tt.kind + ".json", kind: tt.kind, body: []byte(tt.body)})
			require.NoError(t, err)
			require.Len(t, objects, 1)
			require.Equal(t, tt.id, objects[0].id)
			require.Equal(t, tt.objectKind, objects[0].kind)
			require.Equal(t, tt.displayName, objects[0].name)
			if tt.name == "champion" {
				require.Nil(t, objects[0].cost)
				require.Nil(t, objects[0].purchasable)
			}
			if tt.wantAsset {
				require.Len(t, assets, 1)
			} else {
				require.Empty(t, assets)
			}
		})
	}
}

func TestParseTeamPlannerDocumentKeepsSetScopedAuthoritativeCodes(t *testing.T) {
	units, err := parseTeamPlannerDocument([]byte(`{
		"TFTSet17":[{"character_id":"TFT17_Enemy_Aatrox","tier":5,"team_planner_code":0}],
		"TFTSet18":[
			{"character_id":"DA_Cinderling18","tier":1,"team_planner_code":1015},
			{"character_id":"DA_18_Sentry","tier":1,"team_planner_code":1065}
		]
	}`))

	require.NoError(t, err)
	require.Len(t, units, 2)
	require.Equal(t, "TFTSet18", units[0].setID)
	require.Equal(t, "DA_Cinderling18", units[0].characterID)
	require.Equal(t, 1015, units[0].plannerCode)
	require.Equal(t, int32(1), *units[0].cost)
	require.Equal(t, 1065, units[1].plannerCode)
}

func TestParseTeamPlannerDocumentRejectsDuplicateCodes(t *testing.T) {
	_, err := parseTeamPlannerDocument([]byte(`{"TFTSet18":[
		{"character_id":"DA_A","team_planner_code":1001},
		{"character_id":"DA_B","team_planner_code":1001}
	]}`))

	require.ErrorContains(t, err, "duplicate team planner mapping")
}

func TestParseCDragonItemUsesExplicitAugmentFlagBeforeIDHeuristic(t *testing.T) {
	doc := document{name: "rich.json", kind: "rich", body: []byte(`{"items":[
		{"apiName":"DA_FocusedFire","name":"Focused Fire","isAugment":true},
		{"apiName":"TFT18_AugmentLikeItem","name":"Ordinary Item","isAugment":false}
	],"setData":[{"champions":[{"apiName":"TFT18_A","name":"A","cost":1}]}]}`)}

	objects, _, err := parseDocument("cdragon", "16.17", doc)
	require.NoError(t, err)
	byID := objectsByID(objects)
	require.Equal(t, "augment", byID["DA_FocusedFire"].kind)
	require.Equal(t, "item", byID["TFT18_AugmentLikeItem"].kind)
}

func TestMergeParsedDocumentsUsesRichItemClassification(t *testing.T) {
	richPayload := []byte(`{"apiName":"DA_FocusedFire","isAugment":true}`)
	clientPayload := []byte(`{"nameId":"DA_FocusedFire","name":"Localized","squareIconPath":"/lol-game-data/assets/ASSETS/Augments/FocusedFire.png"}`)
	documents := []parsedDocument{
		{kind: "rich", objects: []staticObject{{kind: "augment", id: "DA_FocusedFire", name: "Focused Fire", payload: richPayload}}},
		{kind: "tftitems", objects: []staticObject{{kind: "item", id: "DA_FocusedFire", name: "Localized", payload: clientPayload}}},
	}

	objects, _ := mergeParsedDocuments("cdragon", documents)
	require.Len(t, objects, 1)
	require.Equal(t, "augment", objects[0].kind)
	require.Equal(t, "Localized", objects[0].name)
	require.JSONEq(t, string(clientPayload), string(objects[0].payload))
}

func TestDocumentsSHAIncludesParserVersion(t *testing.T) {
	documents := []document{{name: "catalog.json", body: []byte(`{"data":[]}`)}}

	require.NotEqual(t, documentsSHA("tft-static-v1", documents), documentsSHA("tft-static-v2", documents))
}

func TestSyncCDragonDoesNotReuseSnapshotFromRichValidatorAlone(t *testing.T) {
	etag := `"unchanged-rich"`
	q := &recordingStaticQuerier{latest: sqlcgen.TftStaticSnapshot{
		ID: 1, Source: "cdragon", Patch: "16.17", Build: "16.17.1", Revision: "old-joint-revision",
		Locale: "en_us", Status: "published", Etag: &etag, ParserVersion: parserVersion,
	}}
	requests := []string{}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		body := cdragonTestResponse(req.URL.Path)
		response := &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}
		if strings.HasSuffix(req.URL.Path, "/en_us.json") {
			response.Header.Set("ETag", etag)
		}
		return response, nil
	})}

	id, _, err := syncCDragon(context.Background(), q, Options{Root: t.TempDir(), Client: client}, "16.17.1", "16.17", "en_us")
	require.NoError(t, err)
	require.Equal(t, int64(2), id)
	require.Len(t, requests, 7)
	for _, request := range requests {
		require.True(t, strings.HasPrefix(request, http.MethodGet+" "))
	}
	require.Len(t, q.created, 1)
	require.NotEqual(t, q.latest.Revision, q.created[0].Revision)
}

func TestSyncDDragonPublishedSnapshotIsNotRepublished(t *testing.T) {
	q := &recordingStaticQuerier{latest: sqlcgen.TftStaticSnapshot{
		ID: 11, Source: "ddragon", Patch: "16.17", Build: "16.17.1",
		Locale: "en_us", Status: "published", ParserVersion: parserVersion,
	}}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("published DDragon snapshot must not fetch documents")
		return nil, nil
	})}

	id, jobs, err := syncDDragon(context.Background(), q, Options{Root: t.TempDir(), Client: client}, "16.17.1", "16.17", "en_us")

	require.NoError(t, err)
	require.Zero(t, id)
	require.Zero(t, jobs)
	require.Empty(t, q.created)
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

func objectsByID(objects []staticObject) map[string]staticObject {
	out := make(map[string]staticObject, len(objects))
	for _, object := range objects {
		out[object.id] = object
	}
	return out
}

type recordingStaticQuerier struct {
	latest  sqlcgen.TftStaticSnapshot
	created []sqlcgen.CreateTFTStaticSnapshotParams
}

func (q *recordingStaticQuerier) GetLatestTFTStaticSnapshotAnyStatus(context.Context, string, string) (sqlcgen.TftStaticSnapshot, error) {
	return q.latest, nil
}

func (q *recordingStaticQuerier) CreateTFTStaticSnapshot(_ context.Context, arg sqlcgen.CreateTFTStaticSnapshotParams) (sqlcgen.TftStaticSnapshot, error) {
	q.created = append(q.created, arg)
	return sqlcgen.TftStaticSnapshot{ID: 2, Source: arg.Source, Patch: arg.Patch, Build: arg.Build, Revision: arg.Revision, Locale: arg.Locale, Status: "building", ParserVersion: arg.ParserVersion}, nil
}

func (*recordingStaticQuerier) UpsertTFTStaticObject(context.Context, sqlcgen.UpsertTFTStaticObjectParams) error {
	return nil
}

func (*recordingStaticQuerier) UpsertTFTStaticTeamPlannerUnit(context.Context, sqlcgen.UpsertTFTStaticTeamPlannerUnitParams) error {
	return nil
}

func (*recordingStaticQuerier) EnqueueTFTStaticAsset(context.Context, sqlcgen.EnqueueTFTStaticAssetParams) (int64, error) {
	return 1, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func cdragonTestResponse(path string) string {
	switch {
	case strings.HasSuffix(path, "/en_us.json"):
		return `{"setData":[{"champions":[{"apiName":"TFT18_A","name":"A","cost":1}]}]}`
	case strings.HasSuffix(path, "/tftsets.json"):
		return `{"LCTFTModeData":{"mActiveSets":[{"SetName":"TFTSet18","SetDisplayName":"Set 18"}]}}`
	case strings.HasSuffix(path, "/tftchampions.json"):
		return `[{"character_record":{"character_id":"TFT18_A","display_name":"A"}}]`
	case strings.HasSuffix(path, "/tftchampions-teamplanner.json"):
		return `{"TFTSet18":[{"character_id":"TFT18_A","tier":1,"team_planner_code":1001}]}`
	case strings.HasSuffix(path, "/tfttraits.json"):
		return `[{"trait_id":"TFT18_Trait","display_name":"Trait"}]`
	case strings.HasSuffix(path, "/tftitems.json"):
		return `[{"nameId":"TFT18_Item","name":"Changed client item"}]`
	case strings.HasSuffix(path, "/tftregionportals.json"):
		return `[{"nameId":"TFT18_Portal","displayName":"Portal"}]`
	default:
		return `{}`
	}
}
