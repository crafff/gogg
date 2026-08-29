package staticdata

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

const parserVersion = "tft-static-v2"

const (
	maxDocumentBody = 128 << 20
	maxAssetBody    = 32 << 20
)

var ddragonCategories = []string{
	"tft-arena", "tft-augments", "tft-champion", "tft-item",
	"tft-queues", "tft-regalia", "tft-tactician", "tft-trait",
}

var cdragonClientFiles = []string{"tftsets.json", "tftchampions.json", "tfttraits.json", "tftitems.json", "tftregionportals.json"}

type Querier interface {
	GetLatestTFTStaticSnapshotAnyStatus(context.Context, string, string) (sqlcgen.TftStaticSnapshot, error)
	CreateTFTStaticSnapshot(context.Context, sqlcgen.CreateTFTStaticSnapshotParams) (sqlcgen.TftStaticSnapshot, error)
	UpsertTFTStaticObject(context.Context, sqlcgen.UpsertTFTStaticObjectParams) error
	EnqueueTFTStaticAsset(context.Context, sqlcgen.EnqueueTFTStaticAssetParams) (int64, error)
}

type AssetQuerier interface {
	ClaimTFTStaticAssetJobs(context.Context, *string, int32, int32) ([]sqlcgen.TftStaticAssetJob, error)
	CompleteTFTStaticAssetJob(context.Context, sqlcgen.CompleteTFTStaticAssetJobParams) (int64, error)
	SkipTFTStaticAssetJob(context.Context, sqlcgen.SkipTFTStaticAssetJobParams) (int64, error)
	FailTFTStaticAssetJob(context.Context, sqlcgen.FailTFTStaticAssetJobParams) (int64, error)
	ReleaseTFTStaticAssetLeases(context.Context, *string) error
	GetTFTStaticAssetQueueState(context.Context) (sqlcgen.GetTFTStaticAssetQueueStateRow, error)
}

type ScopedAssetQuerier interface {
	ClaimTFTStaticAssetGroups(context.Context, sqlcgen.ClaimTFTStaticAssetGroupsParams) ([]sqlcgen.ClaimTFTStaticAssetGroupsRow, error)
	CompleteTFTStaticAssetGroup(context.Context, sqlcgen.CompleteTFTStaticAssetGroupParams) (int64, error)
	FailTFTStaticAssetGroup(context.Context, sqlcgen.FailTFTStaticAssetGroupParams) (int64, error)
	SkipTFTStaticAssetGroup(context.Context, sqlcgen.SkipTFTStaticAssetGroupParams) (int64, error)
	ReleaseTFTStaticAssetLeases(context.Context, *string) error
	GetTFTStaticAssetQueueStateForSnapshots(context.Context, []int64) (sqlcgen.GetTFTStaticAssetQueueStateForSnapshotsRow, error)
}

type Options struct {
	Root, DDragonBaseURL, CDragonBaseURL string
	Locales                              []string
	Client                               *http.Client
}

type SnapshotRef struct {
	ID        int64
	Source    string
	Locale    string
	AssetJobs int
}

type SyncResult struct {
	Build, Patch string
	SnapshotIDs  []int64
	Snapshots    []SnapshotRef
	AssetJobs    int
}

type SourceSyncInput struct {
	Source string
	Build  string
	Patch  string
}

type document struct {
	name, kind, url string
	body            []byte
}

func Sync(ctx context.Context, q Querier, opts Options) (SyncResult, error) {
	opts = defaultOptions(opts)
	build, patch, err := resolveBuild(ctx, opts)
	if err != nil {
		return SyncResult{}, err
	}
	result := SyncResult{Build: build, Patch: patch}
	for _, localeValue := range opts.Locales {
		locale := strings.ToLower(localeValue)
		id, jobs, err := syncDDragon(ctx, q, opts, build, patch, locale)
		if err != nil {
			return SyncResult{}, err
		}
		appendSnapshot(&result, id, jobs, "ddragon", locale)
		id, jobs, err = syncCDragon(ctx, q, opts, build, patch, locale)
		if err != nil {
			return SyncResult{}, err
		}
		appendSnapshot(&result, id, jobs, "cdragon", locale)
	}
	return result, nil
}

// SyncSource isolates each provider so the workflow can publish the CDragon
// catalog required by match ingestion before attempting optional DDragon data.
func SyncSource(ctx context.Context, q Querier, opts Options, input SourceSyncInput) (SyncResult, error) {
	opts = defaultOptions(opts)
	if input.Source != "cdragon" && input.Source != "ddragon" {
		return SyncResult{}, fmt.Errorf("unsupported TFT static source %q", input.Source)
	}
	build, patch := input.Build, input.Patch
	if build == "" {
		var err error
		build, patch, err = resolveBuild(ctx, opts)
		if err != nil {
			return SyncResult{}, err
		}
	} else if patch == "" {
		patch = patchOf(build)
	}
	result := SyncResult{Build: build, Patch: patch}
	for _, localeValue := range opts.Locales {
		locale := strings.ToLower(localeValue)
		var id int64
		var jobs int
		var err error
		switch input.Source {
		case "cdragon":
			id, jobs, err = syncCDragon(ctx, q, opts, build, patch, locale)
		case "ddragon":
			id, jobs, err = syncDDragon(ctx, q, opts, build, patch, locale)
		}
		if err != nil {
			return SyncResult{}, err
		}
		appendSnapshot(&result, id, jobs, input.Source, locale)
	}
	return result, nil
}

func defaultOptions(opts Options) Options {
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 90 * time.Second}
	}
	if opts.DDragonBaseURL == "" {
		opts.DDragonBaseURL = "https://ddragon.leagueoflegends.com"
	}
	if opts.CDragonBaseURL == "" {
		opts.CDragonBaseURL = "https://raw.communitydragon.org"
	}
	if len(opts.Locales) == 0 {
		opts.Locales = []string{"en_us", "zh_cn"}
	}
	return opts
}

func resolveBuild(ctx context.Context, opts Options) (string, string, error) {
	var versions []string
	if err := getJSON(ctx, opts.Client, strings.TrimRight(opts.DDragonBaseURL, "/")+"/api/versions.json", &versions); err != nil {
		return "", "", fmt.Errorf("probe Data Dragon versions: %w", err)
	}
	if len(versions) == 0 {
		return "", "", errors.New("Data Dragon versions list is empty")
	}
	build := versions[0]
	return build, patchOf(build), nil
}

func appendSnapshot(result *SyncResult, id int64, jobs int, source, locale string) {
	if id <= 0 {
		return
	}
	result.SnapshotIDs = append(result.SnapshotIDs, id)
	result.Snapshots = append(result.Snapshots, SnapshotRef{ID: id, Source: source, Locale: locale, AssetJobs: jobs})
	result.AssetJobs += jobs
}

func syncDDragon(ctx context.Context, q Querier, opts Options, build, patch, locale string) (int64, int, error) {
	if latest, err := q.GetLatestTFTStaticSnapshotAnyStatus(ctx, "ddragon", locale); err == nil && latest.Build == build && latest.ParserVersion == parserVersion && latest.Status == "published" {
		return 0, 0, nil
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, err
	}
	sourceLocale := mapLocale(locale, true)
	docs := make([]document, 0, len(ddragonCategories))
	for _, category := range ddragonCategories {
		u := fmt.Sprintf("%s/cdn/%s/data/%s/%s.json", strings.TrimRight(opts.DDragonBaseURL, "/"), build, sourceLocale, category)
		body, _, _, err := get(ctx, opts.Client, u)
		if err != nil {
			return 0, 0, fmt.Errorf("fetch Data Dragon %s %s: %w", locale, category, err)
		}
		docs = append(docs, document{name: category + ".json", kind: category, url: u, body: body})
	}
	revision := documentsSHA(parserVersion, docs)
	snapshot, err := q.CreateTFTStaticSnapshot(ctx, sqlcgen.CreateTFTStaticSnapshotParams{Source: "ddragon", Patch: patch, Build: build, Revision: revision, Locale: locale, SourceUrl: docs[0].url, ParserVersion: parserVersion})
	if err != nil {
		return 0, 0, err
	}
	jobs, err := persistDocuments(ctx, q, opts, snapshot.ID, "ddragon", patch, revision, locale, build, docs)
	return snapshot.ID, jobs, err
}

func syncCDragon(ctx context.Context, q Querier, opts Options, build, patch, locale string) (int64, int, error) {
	base := strings.TrimRight(opts.CDragonBaseURL, "/")
	richURL := fmt.Sprintf("%s/%s/cdragon/tft/%s.json", base, patch, locale)
	latest, latestErr := q.GetLatestTFTStaticSnapshotAnyStatus(ctx, "cdragon", locale)
	if latestErr != nil && !errors.Is(latestErr, pgx.ErrNoRows) {
		return 0, 0, latestErr
	}
	richBody, responseETag, responseModified, err := get(ctx, opts.Client, richURL)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch CommunityDragon rich %s: %w", locale, err)
	}
	etag, modified := responseETag, responseModified
	docs := []document{{name: "rich.json", kind: "rich", url: richURL, body: richBody}}
	sourceLocale := mapLocale(locale, false)
	for _, file := range cdragonClientFiles {
		u := fmt.Sprintf("%s/%s/plugins/rcp-be-lol-game-data/global/%s/v1/%s", base, patch, sourceLocale, file)
		body, _, _, err := get(ctx, opts.Client, u)
		if err != nil {
			return 0, 0, fmt.Errorf("fetch CommunityDragon %s %s: %w", locale, file, err)
		}
		docs = append(docs, document{name: file, kind: strings.TrimSuffix(file, ".json"), url: u, body: body})
	}
	revision := documentsSHA(parserVersion, docs)
	if latestErr == nil && latest.Status == "published" && latest.Patch == patch && latest.ParserVersion == parserVersion && latest.Revision == revision {
		return 0, 0, nil
	}
	snapshot, err := q.CreateTFTStaticSnapshot(ctx, sqlcgen.CreateTFTStaticSnapshotParams{Source: "cdragon", Patch: patch, Build: build, Revision: revision, Locale: locale, Etag: optional(etag), LastModified: optional(modified), SourceUrl: richURL, ParserVersion: parserVersion})
	if err != nil {
		return 0, 0, err
	}
	jobs, err := persistDocuments(ctx, q, opts, snapshot.ID, "cdragon", patch, revision, locale, build, docs)
	return snapshot.ID, jobs, err
}

func persistDocuments(ctx context.Context, q Querier, opts Options, snapshotID int64, source, patch, revision, locale, build string, docs []document) (int, error) {
	parsed := make([]parsedDocument, 0, len(docs))
	for _, doc := range docs {
		rel := filepath.Join("tft", "static", source, patch, revision, locale, doc.name)
		if err := writeAtomic(filepath.Join(opts.Root, rel), doc.body, 0o640); err != nil {
			return 0, err
		}
		assetVersion := build
		if source == "cdragon" {
			assetVersion = patch
		}
		objects, assets, err := parseDocument(source, assetVersion, doc)
		if err != nil {
			return 0, fmt.Errorf("parse TFT static document %s: %w", doc.name, err)
		}
		parsed = append(parsed, parsedDocument{kind: doc.kind, objects: objects, assets: assets})
	}
	objects, assets := mergeParsedDocuments(source, parsed)
	for _, object := range objects {
		if err := q.UpsertTFTStaticObject(ctx, sqlcgen.UpsertTFTStaticObjectParams{SnapshotID: snapshotID, ObjectKind: object.kind, ObjectID: object.id, Name: optional(object.name), Purchasable: object.purchasable, Cost: object.cost, Payload: object.payload}); err != nil {
			return 0, err
		}
	}
	jobs := 0
	for _, asset := range assets {
		digest := sha256.Sum256([]byte(asset))
		key := hex.EncodeToString(digest[:])
		ext := filepath.Ext(strings.Split(asset, "?")[0])
		if ext == "" || len(ext) > 6 {
			ext = ".png"
		}
		rel := filepath.Join("tft", "static", source, patch, revision, "assets", key+ext)
		n, err := q.EnqueueTFTStaticAsset(ctx, sqlcgen.EnqueueTFTStaticAssetParams{SnapshotID: snapshotID, AssetKey: key, SourceUrl: asset, RelativePath: filepath.ToSlash(rel)})
		if err != nil {
			return 0, err
		}
		jobs += int(n)
	}
	return jobs, nil
}

type parsedDocument struct {
	kind    string
	objects []staticObject
	assets  []string
}

type staticObject struct {
	kind, id, name string
	purchasable    *bool
	cost           *int32
	payload        []byte
}

func parseDocument(source, build string, doc document) ([]staticObject, []string, error) {
	var root any
	if err := json.Unmarshal(doc.body, &root); err != nil {
		return nil, nil, err
	}
	objects := []staticObject{}
	assets := map[string]bool{}
	if source == "ddragon" {
		if m, ok := root.(map[string]any); ok {
			if data, ok := m["data"].(map[string]any); ok {
				for key, raw := range data {
					object, asset := ddragonObject(doc.kind, key, raw, build)
					objects = append(objects, object)
					if asset != "" {
						assets[asset] = true
					}
				}
			}
		}
	} else {
		walkCDragon(root, doc.kind, build, &objects, assets)
	}
	objects = dedupeStaticObjects(objects)
	urls := make([]string, 0, len(assets))
	for u := range assets {
		urls = append(urls, u)
	}
	sort.Strings(urls)
	if len(objects) == 0 {
		return nil, nil, errors.New("document contains no identifiable objects")
	}
	if requiresPurchasableUnitCatalog(source, doc.kind) {
		hasPurchasableUnit := false
		for _, object := range objects {
			if object.kind == "unit" && object.purchasable != nil && *object.purchasable && object.cost != nil && *object.cost > 0 {
				hasPurchasableUnit = true
				break
			}
		}
		if !hasPurchasableUnit {
			return nil, nil, errors.New("unit catalog contains no purchasable units")
		}
	}
	return objects, urls, nil
}

func ddragonObject(kind, key string, raw any, build string) (staticObject, string) {
	payload, _ := json.Marshal(raw)
	object := staticObject{kind: normalizeKind(kind), id: key, payload: payload}
	m, _ := raw.(map[string]any)
	object.name, _ = m["name"].(string)
	if object.kind == "unit" {
		cost := int32(number(m["tier"]))
		purchasable := cost > 0
		object.cost, object.purchasable = &cost, &purchasable
	}
	image, _ := m["image"].(map[string]any)
	group, _ := image["group"].(string)
	full, _ := image["full"].(string)
	asset := ""
	if group != "" && full != "" {
		asset = fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/img/%s/%s", build, group, full)
	}
	return object, asset
}

func walkCDragon(value any, fallbackKind, patch string, objects *[]staticObject, assets map[string]bool) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			walkCDragon(item, fallbackKind, patch, objects, assets)
		}
	case map[string]any:
		if object, ok := cdragonObject(typed, fallbackKind); ok {
			*objects = append(*objects, object)
		}
		if asset := cdragonAssetURL(stringField(typed, "iconPath", "icon", "icon_path", "squareIconPath"), patch); asset != "" {
			assets[asset] = true
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := typed[key]
			switch child.(type) {
			case []any, map[string]any:
				walkCDragon(child, cdragonChildKind(key, fallbackKind), patch, objects, assets)
			}
		}
	}
}

func cdragonObject(values map[string]any, fallbackKind string) (staticObject, bool) {
	kind := normalizeKind(fallbackKind)
	var id, name string
	switch kind {
	case "set":
		id = stringField(values, "SetName")
		name = stringField(values, "SetDisplayName", "SetName")
	case "unit":
		id = stringField(values, "apiName", "characterId", "character_id", "id")
		name = stringField(values, "name", "display_name", "displayName", "characterName")
	case "trait":
		id = stringField(values, "apiName", "trait_id", "id")
		name = stringField(values, "name", "display_name", "displayName")
	case "item":
		id = stringField(values, "apiName", "nameId", "id")
		name = stringField(values, "name", "display_name", "displayName")
		isAugment, known := values["isAugment"].(bool)
		if (known && isAugment) || (!known && strings.Contains(strings.ToLower(id), "augment")) {
			kind = "augment"
		}
	case "portal":
		id = stringField(values, "nameId", "apiName", "id")
		name = stringField(values, "displayName", "name")
	default:
		id = stringField(values, "apiName", "id", "characterId", "character_id")
		name = stringField(values, "name", "display_name", "displayName", "characterName")
		isAugment, augmentKnown := values["isAugment"].(bool)
		switch {
		case augmentKnown && isAugment:
			kind = "augment"
		case augmentKnown:
			kind = "item"
		case strings.Contains(strings.ToLower(id), "augment"):
			kind = "augment"
		case strings.Contains(strings.ToLower(id), "champion") || values["cost"] != nil:
			kind = "unit"
		}
	}
	if id == "" {
		return staticObject{}, false
	}
	payload, _ := json.Marshal(values)
	object := staticObject{kind: kind, id: id, name: name, payload: payload}
	if kind == "unit" {
		if rawCost, ok := values["cost"]; ok {
			cost := int32(number(rawCost))
			purchasable := cost > 0
			object.cost, object.purchasable = &cost, &purchasable
		}
	}
	return object, true
}

func cdragonChildKind(key, fallbackKind string) string {
	switch strings.ToLower(key) {
	case "champions":
		return "tftchampions"
	case "items":
		return "tftitems"
	case "traits":
		return "tfttraits"
	default:
		return fallbackKind
	}
}

func cdragonAssetURL(icon, patch string) string {
	lower := strings.ToLower(strings.TrimSpace(icon))
	const prefix = "/lol-game-data/assets"
	if !strings.HasPrefix(lower, prefix) {
		return ""
	}
	suffix := strings.TrimPrefix(lower, prefix)
	if suffix == "" || suffix == "/" {
		return ""
	}
	return fmt.Sprintf("https://raw.communitydragon.org/%s/plugins/rcp-be-lol-game-data/global/default%s", patch, suffix)
}

func dedupeStaticObjects(objects []staticObject) []staticObject {
	byKey := make(map[string]staticObject, len(objects))
	for _, object := range objects {
		byKey[object.kind+"\x00"+object.id] = object
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]staticObject, 0, len(keys))
	for _, key := range keys {
		out = append(out, byKey[key])
	}
	return out
}

func mergeParsedDocuments(source string, documents []parsedDocument) ([]staticObject, []string) {
	authoritativeItemKinds := map[string]string{}
	if source == "cdragon" {
		for _, doc := range documents {
			if doc.kind != "rich" {
				continue
			}
			for _, object := range doc.objects {
				if object.kind == "item" || object.kind == "augment" {
					authoritativeItemKinds[object.id] = object.kind
				}
			}
		}
	}

	byKey := map[string]staticObject{}
	assetSet := map[string]bool{}
	for _, doc := range documents {
		for _, object := range doc.objects {
			if source == "cdragon" && doc.kind != "rich" && normalizeKind(doc.kind) == "item" {
				if kind, ok := authoritativeItemKinds[object.id]; ok {
					object.kind = kind
				}
			}
			key := object.kind + "\x00" + object.id
			if previous, ok := byKey[key]; ok {
				if object.name == "" {
					object.name = previous.name
				}
				if object.cost == nil {
					object.cost = previous.cost
				}
				if object.purchasable == nil {
					object.purchasable = previous.purchasable
				}
			}
			byKey[key] = object
		}
		for _, asset := range doc.assets {
			assetSet[asset] = true
		}
	}

	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	objects := make([]staticObject, 0, len(keys))
	for _, key := range keys {
		objects = append(objects, byKey[key])
	}
	assets := make([]string, 0, len(assetSet))
	for asset := range assetSet {
		assets = append(assets, asset)
	}
	sort.Strings(assets)
	return objects, assets
}

func requiresPurchasableUnitCatalog(source, kind string) bool {
	return (source == "ddragon" && normalizeKind(kind) == "unit") || (source == "cdragon" && kind == "rich")
}

func normalizeKind(kind string) string {
	switch kind {
	case "tft-champion", "tftchampions":
		return "unit"
	case "tft-item", "tftitems":
		return "item"
	case "tft-trait", "tfttraits":
		return "trait"
	case "tft-augments":
		return "augment"
	case "tftsets":
		return "set"
	case "tftregionportals":
		return "portal"
	}
	return kind
}
func documentsSHA(version string, docs []document) string {
	h := sha256.New()
	_, _ = h.Write([]byte(version))
	_, _ = h.Write([]byte{0})
	for _, doc := range docs {
		_, _ = h.Write([]byte(doc.name))
		_, _ = h.Write(doc.body)
	}
	return hex.EncodeToString(h.Sum(nil))
}
func patchOf(build string) string {
	parts := strings.Split(build, ".")
	if len(parts) < 2 {
		return build
	}
	return parts[0] + "." + parts[1]
}
func mapLocale(locale string, ddragon bool) string {
	if ddragon {
		if locale == "en_us" {
			return "en_US"
		}
		if locale == "zh_cn" {
			return "zh_CN"
		}
	}
	if locale == "en_us" {
		return "default"
	}
	return locale
}
func stringField(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}
func number(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}
func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
func getJSON(ctx context.Context, client *http.Client, url string, target any) error {
	body, _, _, err := get(ctx, client, url)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}

type HTTPStatusError struct {
	URL        string
	Status     string
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("GET %s: %s", e.URL, e.Status)
}

func get(ctx context.Context, client *http.Client, url string) ([]byte, string, string, error) {
	return getWithLimit(ctx, client, url, maxDocumentBody)
}

func getWithLimit(ctx context.Context, client *http.Client, url string, maxBody int64) ([]byte, string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, "", "", &HTTPStatusError{URL: url, Status: resp.Status, StatusCode: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, "", "", err
	}
	if int64(len(body)) > maxBody {
		return nil, "", "", fmt.Errorf("GET %s: response exceeds %d bytes", url, maxBody)
	}
	return body, resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"), nil
}
func writeAtomic(path string, body []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".static-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(body); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	return nil
}

type DownloadResult struct {
	Processed      int
	Fetched        int
	Completed      int64
	Skipped        int64
	Failed         int64
	Total          int64
	Remaining      int64
	NextEligibleAt time.Time
	HasMore        bool
}

type DownloadInput struct {
	SnapshotIDs []int64
	BatchSize   int
	Concurrency int
}

type ExhaustedAssetsError struct {
	Count int64
}

var errAssetDownloadDeferred = errors.New("TFT static asset download deferred for retry")

func (e *ExhaustedAssetsError) Error() string {
	return fmt.Sprintf("%d TFT static asset jobs exhausted their retries", e.Count)
}

func DownloadAssets(ctx context.Context, q AssetQuerier, root string, client *http.Client, owner string, limit int) (DownloadResult, error) {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	jobs, err := q.ClaimTFTStaticAssetJobs(ctx, &owner, 660, int32(limit))
	if err != nil {
		return DownloadResult{}, err
	}
	ownerRef := &owner
	defer func() { _ = q.ReleaseTFTStaticAssetLeases(context.WithoutCancel(ctx), ownerRef) }()
	for _, job := range jobs {
		body, _, _, err := getWithLimit(ctx, client, job.SourceUrl, maxAssetBody)
		if err != nil {
			if isOptionalMissingPNG(job.SourceUrl, err) {
				path := filepath.Join(root, filepath.FromSlash(job.RelativePath))
				if writeErr := writeAtomic(path, transparentPNG, 0o640); writeErr != nil {
					return DownloadResult{}, writeErr
				}
				digest := sha256.Sum256(transparentPNG)
				message := err.Error()
				rows, skipErr := q.SkipTFTStaticAssetJob(ctx, sqlcgen.SkipTFTStaticAssetJobParams{
					LastError: &message, TargetSnapshotID: job.SnapshotID, TargetAssetKey: job.AssetKey,
					LeaseOwnerFilter: ownerRef, Sha256: hex.EncodeToString(digest[:]),
				})
				if updateErr := requireAssetLeaseUpdate("skip TFT static asset job", rows, skipErr); updateErr != nil {
					return DownloadResult{}, updateErr
				}
				continue
			}
			message := err.Error()
			rows, failErr := q.FailTFTStaticAssetJob(ctx, sqlcgen.FailTFTStaticAssetJobParams{LastError: &message, TargetSnapshotID: job.SnapshotID, TargetAssetKey: job.AssetKey, LeaseOwnerFilter: ownerRef})
			if updateErr := requireAssetLeaseUpdate("fail TFT static asset job", rows, failErr); updateErr != nil {
				return DownloadResult{}, errors.Join(err, updateErr)
			}
			return DownloadResult{}, err
		}
		path := filepath.Join(root, filepath.FromSlash(job.RelativePath))
		if err := writeAtomic(path, body, 0o640); err != nil {
			message := err.Error()
			rows, failErr := q.FailTFTStaticAssetJob(ctx, sqlcgen.FailTFTStaticAssetJobParams{LastError: &message, TargetSnapshotID: job.SnapshotID, TargetAssetKey: job.AssetKey, LeaseOwnerFilter: ownerRef})
			if updateErr := requireAssetLeaseUpdate("fail TFT static asset job", rows, failErr); updateErr != nil {
				return DownloadResult{}, errors.Join(err, updateErr)
			}
			return DownloadResult{}, err
		}
		digest := sha256.Sum256(body)
		rows, err := q.CompleteTFTStaticAssetJob(ctx, sqlcgen.CompleteTFTStaticAssetJobParams{TargetSnapshotID: job.SnapshotID, TargetAssetKey: job.AssetKey, LeaseOwnerFilter: ownerRef, Sha256: hex.EncodeToString(digest[:])})
		if err := requireAssetLeaseUpdate("complete TFT static asset job", rows, err); err != nil {
			return DownloadResult{}, err
		}
	}
	queue, err := q.GetTFTStaticAssetQueueState(ctx)
	if err != nil {
		return DownloadResult{}, err
	}
	result := DownloadResult{Processed: len(jobs), Remaining: queue.Remaining, HasMore: queue.Remaining > 0}
	if queue.NextEligibleAt.Valid {
		result.NextEligibleAt = queue.NextEligibleAt.Time
	}
	return result, nil
}

func DownloadAssetsForSnapshots(ctx context.Context, q ScopedAssetQuerier, root string, client *http.Client, owner string, input DownloadInput) (DownloadResult, error) {
	input.SnapshotIDs = uniqueSnapshotIDs(input.SnapshotIDs)
	if len(input.SnapshotIDs) == 0 {
		return DownloadResult{}, errors.New("TFT static snapshot IDs are required")
	}
	if input.BatchSize < 1 || input.BatchSize > 1000 {
		return DownloadResult{}, fmt.Errorf("TFT static download batch must be 1..1000, got %d", input.BatchSize)
	}
	if input.Concurrency < 1 || input.Concurrency > 64 {
		return DownloadResult{}, fmt.Errorf("TFT static download concurrency must be 1..64, got %d", input.Concurrency)
	}
	if input.BatchSize > input.Concurrency*8 {
		return DownloadResult{}, fmt.Errorf("TFT static download batch %d exceeds safe limit %d for concurrency %d", input.BatchSize, input.Concurrency*8, input.Concurrency)
	}
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	ownerRef := &owner
	jobs, err := q.ClaimTFTStaticAssetGroups(ctx, sqlcgen.ClaimTFTStaticAssetGroupsParams{
		SnapshotIds: input.SnapshotIDs,
		RowLimit:    int32(input.BatchSize), LeaseOwner: ownerRef, LeaseSeconds: 660,
	})
	if err != nil {
		return DownloadResult{}, err
	}
	defer func() { _ = q.ReleaseTFTStaticAssetLeases(context.WithoutCancel(ctx), ownerRef) }()

	groups := groupClaimedAssets(jobs)
	var processed, fetched atomic.Int64
	var downloads errgroup.Group
	downloads.SetLimit(input.Concurrency)
	for _, jobsForURL := range groups {
		jobsForURL := jobsForURL
		downloads.Go(func() error {
			fetched.Add(1)
			if err := downloadAssetGroup(ctx, q, root, client, ownerRef, input.SnapshotIDs, jobsForURL); err != nil {
				if errors.Is(err, errAssetDownloadDeferred) {
					return nil
				}
				return err
			}
			processed.Add(int64(len(jobsForURL)))
			return nil
		})
	}
	if err := downloads.Wait(); err != nil {
		return DownloadResult{}, err
	}
	queue, err := q.GetTFTStaticAssetQueueStateForSnapshots(ctx, input.SnapshotIDs)
	if err != nil {
		return DownloadResult{}, err
	}
	if queue.Exhausted > 0 {
		return DownloadResult{}, &ExhaustedAssetsError{Count: queue.Exhausted}
	}
	result := DownloadResult{
		Processed: int(processed.Load()), Fetched: int(fetched.Load()),
		Completed: queue.Completed, Skipped: queue.Skipped, Failed: queue.Failed,
		Total: queue.Total, Remaining: queue.Remaining, HasMore: queue.Remaining > 0,
	}
	if queue.NextEligibleAt.Valid {
		result.NextEligibleAt = queue.NextEligibleAt.Time
	}
	return result, nil
}

func uniqueSnapshotIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func groupClaimedAssets(jobs []sqlcgen.ClaimTFTStaticAssetGroupsRow) [][]sqlcgen.ClaimTFTStaticAssetGroupsRow {
	byKey := make(map[string][]sqlcgen.ClaimTFTStaticAssetGroupsRow, len(jobs))
	keys := make([]string, 0, len(jobs))
	for _, job := range jobs {
		if _, exists := byKey[job.AssetKey]; !exists {
			keys = append(keys, job.AssetKey)
		}
		byKey[job.AssetKey] = append(byKey[job.AssetKey], job)
	}
	sort.Strings(keys)
	out := make([][]sqlcgen.ClaimTFTStaticAssetGroupsRow, 0, len(keys))
	for _, key := range keys {
		out = append(out, byKey[key])
	}
	return out
}

func downloadAssetGroup(ctx context.Context, q ScopedAssetQuerier, root string, client *http.Client, owner *string, snapshotIDs []int64, jobs []sqlcgen.ClaimTFTStaticAssetGroupsRow) error {
	if len(jobs) == 0 {
		return nil
	}
	assetKey, sourceURL := jobs[0].AssetKey, jobs[0].SourceUrl
	for _, job := range jobs[1:] {
		if job.AssetKey != assetKey || job.SourceUrl != sourceURL {
			return failAssetGroup(ctx, q, snapshotIDs, assetKey, owner, errors.New("TFT static asset group has inconsistent source URLs"), len(jobs))
		}
	}
	body, _, _, err := getWithLimit(ctx, client, sourceURL, maxAssetBody)
	if err != nil {
		if isOptionalMissingPNG(sourceURL, err) {
			if writeErr := writeAssetGroup(root, jobs, transparentPNG); writeErr != nil {
				return failAssetGroup(ctx, q, snapshotIDs, assetKey, owner, writeErr, len(jobs))
			}
			digest := sha256.Sum256(transparentPNG)
			message := err.Error()
			rows, skipErr := q.SkipTFTStaticAssetGroup(ctx, sqlcgen.SkipTFTStaticAssetGroupParams{
				LastError: &message, SnapshotIds: snapshotIDs, AssetKey: assetKey,
				LeaseOwner: owner, Sha256: hex.EncodeToString(digest[:]),
			})
			return requireAssetGroupLeaseUpdate("skip TFT static asset group", rows, int64(len(jobs)), skipErr)
		}
		return failAssetGroup(ctx, q, snapshotIDs, assetKey, owner, err, len(jobs))
	}
	if err := writeAssetGroup(root, jobs, body); err != nil {
		return failAssetGroup(ctx, q, snapshotIDs, assetKey, owner, err, len(jobs))
	}
	digest := sha256.Sum256(body)
	rows, err := q.CompleteTFTStaticAssetGroup(ctx, sqlcgen.CompleteTFTStaticAssetGroupParams{
		SnapshotIds: snapshotIDs, AssetKey: assetKey, LeaseOwner: owner,
		Sha256: hex.EncodeToString(digest[:]),
	})
	return requireAssetGroupLeaseUpdate("complete TFT static asset group", rows, int64(len(jobs)), err)
}

func writeAssetGroup(root string, jobs []sqlcgen.ClaimTFTStaticAssetGroupsRow, body []byte) error {
	for _, job := range jobs {
		if err := writeAtomic(filepath.Join(root, filepath.FromSlash(job.RelativePath)), body, 0o640); err != nil {
			return err
		}
	}
	return nil
}

func failAssetGroup(ctx context.Context, q ScopedAssetQuerier, snapshotIDs []int64, assetKey string, owner *string, cause error, expected int) error {
	message := cause.Error()
	rows, err := q.FailTFTStaticAssetGroup(ctx, sqlcgen.FailTFTStaticAssetGroupParams{
		LastError: &message, SnapshotIds: snapshotIDs, AssetKey: assetKey, LeaseOwner: owner,
	})
	if updateErr := requireAssetGroupLeaseUpdate("fail TFT static asset group", rows, int64(expected), err); updateErr != nil {
		return errors.Join(cause, updateErr)
	}
	return fmt.Errorf("%w: %v", errAssetDownloadDeferred, cause)
}

func requireAssetGroupLeaseUpdate(action string, rows, expected int64, err error) error {
	if err != nil {
		return err
	}
	if rows != expected {
		return fmt.Errorf("%s: lease was lost for %d of %d jobs", action, expected-rows, expected)
	}
	return nil
}

func isOptionalMissingPNG(sourceURL string, err error) bool {
	var status *HTTPStatusError
	if !errors.As(err, &status) || (status.StatusCode != http.StatusForbidden && status.StatusCode != http.StatusNotFound) {
		return false
	}
	path := strings.ToLower(strings.Split(sourceURL, "?")[0])
	return strings.Contains(path, "/img/tft-queue-type/") && strings.HasSuffix(path, ".png")
}

var transparentPNG = func() []byte {
	body, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		panic(err)
	}
	return body
}()

func requireAssetLeaseUpdate(action string, rows int64, err error) error {
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("%s: lease was lost", action)
	}
	return nil
}
