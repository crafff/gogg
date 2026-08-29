package staticdata

import (
	"context"
	"crypto/sha256"
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
	"time"

	"github.com/jackc/pgx/v5"

	sqlcgen "github.com/crafff/gogg/packages/sqlc/gen"
)

const parserVersion = "tft-static-v1"

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
	FailTFTStaticAssetJob(context.Context, sqlcgen.FailTFTStaticAssetJobParams) (int64, error)
	ReleaseTFTStaticAssetLeases(context.Context, *string) error
	GetTFTStaticAssetQueueState(context.Context) (sqlcgen.GetTFTStaticAssetQueueStateRow, error)
}

type Options struct {
	Root, DDragonBaseURL, CDragonBaseURL string
	Locales                              []string
	Client                               *http.Client
}

type SyncResult struct {
	Build, Patch string
	SnapshotIDs  []int64
	AssetJobs    int
}

type document struct {
	name, kind, url string
	body            []byte
}

func Sync(ctx context.Context, q Querier, opts Options) (SyncResult, error) {
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
	var versions []string
	if err := getJSON(ctx, opts.Client, strings.TrimRight(opts.DDragonBaseURL, "/")+"/api/versions.json", &versions); err != nil {
		return SyncResult{}, fmt.Errorf("probe Data Dragon versions: %w", err)
	}
	if len(versions) == 0 {
		return SyncResult{}, errors.New("Data Dragon versions list is empty")
	}
	build, patch := versions[0], patchOf(versions[0])
	result := SyncResult{Build: build, Patch: patch}
	for _, localeValue := range opts.Locales {
		locale := strings.ToLower(localeValue)
		id, jobs, err := syncDDragon(ctx, q, opts, build, patch, locale)
		if err != nil {
			return SyncResult{}, err
		}
		if id > 0 {
			result.SnapshotIDs = append(result.SnapshotIDs, id)
			result.AssetJobs += jobs
		}
		id, jobs, err = syncCDragon(ctx, q, opts, build, patch, locale)
		if err != nil {
			return SyncResult{}, err
		}
		if id > 0 {
			result.SnapshotIDs = append(result.SnapshotIDs, id)
			result.AssetJobs += jobs
		}
	}
	return result, nil
}

func syncDDragon(ctx context.Context, q Querier, opts Options, build, patch, locale string) (int64, int, error) {
	if latest, err := q.GetLatestTFTStaticSnapshotAnyStatus(ctx, "ddragon", locale); err == nil && latest.Build == build && latest.Status == "published" {
		return latest.ID, 0, nil
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
	revision := documentsSHA(docs)
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
	etag, modified := head(ctx, opts.Client, richURL)
	if latestErr == nil && latest.Status == "published" && latest.Patch == patch && ((etag != "" && value(latest.Etag) == etag) || (etag == "" && modified != "" && value(latest.LastModified) == modified)) {
		return latest.ID, 0, nil
	}
	if latestErr != nil && !errors.Is(latestErr, pgx.ErrNoRows) {
		return 0, 0, latestErr
	}
	richBody, responseETag, responseModified, err := get(ctx, opts.Client, richURL)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch CommunityDragon rich %s: %w", locale, err)
	}
	if responseETag != "" {
		etag = responseETag
	}
	if responseModified != "" {
		modified = responseModified
	}
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
	revision := documentsSHA(docs)
	if latestErr == nil && latest.Status == "published" && latest.Patch == patch && latest.Revision == revision {
		return latest.ID, 0, nil
	}
	snapshot, err := q.CreateTFTStaticSnapshot(ctx, sqlcgen.CreateTFTStaticSnapshotParams{Source: "cdragon", Patch: patch, Build: build, Revision: revision, Locale: locale, Etag: optional(etag), LastModified: optional(modified), SourceUrl: richURL, ParserVersion: parserVersion})
	if err != nil {
		return 0, 0, err
	}
	jobs, err := persistDocuments(ctx, q, opts, snapshot.ID, "cdragon", patch, revision, locale, build, docs)
	return snapshot.ID, jobs, err
}

func persistDocuments(ctx context.Context, q Querier, opts Options, snapshotID int64, source, patch, revision, locale, build string, docs []document) (int, error) {
	jobs := 0
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
		for _, object := range objects {
			if err := q.UpsertTFTStaticObject(ctx, sqlcgen.UpsertTFTStaticObjectParams{SnapshotID: snapshotID, ObjectKind: object.kind, ObjectID: object.id, Name: optional(object.name), Purchasable: object.purchasable, Cost: object.cost, Payload: object.payload}); err != nil {
				return 0, err
			}
		}
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
	}
	return jobs, nil
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
	urls := make([]string, 0, len(assets))
	for u := range assets {
		urls = append(urls, u)
	}
	sort.Strings(urls)
	if len(objects) == 0 {
		return nil, nil, errors.New("document contains no identifiable objects")
	}
	if normalizeKind(doc.kind) == "unit" {
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
		id := stringField(typed, "apiName", "id", "characterId")
		if id != "" {
			payload, _ := json.Marshal(typed)
			kind := normalizeKind(fallbackKind)
			if strings.Contains(strings.ToLower(id), "champion") || typed["cost"] != nil {
				kind = "unit"
			}
			object := staticObject{kind: kind, id: id, name: stringField(typed, "name"), payload: payload}
			if kind == "unit" {
				cost := int32(number(typed["cost"]))
				purchasable := cost > 0
				object.cost, object.purchasable = &cost, &purchasable
			}
			*objects = append(*objects, object)
		}
		if icon := stringField(typed, "iconPath", "icon"); strings.HasPrefix(strings.ToLower(icon), "/lol-game-data/assets") {
			p := strings.TrimPrefix(strings.ToLower(icon), "/lol-game-data/assets")
			assets[fmt.Sprintf("https://raw.communitydragon.org/%s/plugins/rcp-be-lol-game-data/global/default%s", patch, p)] = true
		}
		for _, child := range typed {
			switch child.(type) {
			case []any, map[string]any:
				walkCDragon(child, fallbackKind, patch, objects, assets)
			}
		}
	}
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
	}
	return kind
}
func documentsSHA(docs []document) string {
	h := sha256.New()
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
func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func getJSON(ctx context.Context, client *http.Client, url string, target any) error {
	body, _, _, err := get(ctx, client, url)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}
func get(ctx context.Context, client *http.Client, url string) ([]byte, string, string, error) {
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
		return nil, "", "", fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	const maxBody = 128 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, "", "", err
	}
	if len(body) > maxBody {
		return nil, "", "", fmt.Errorf("GET %s: response exceeds %d bytes", url, maxBody)
	}
	return body, resp.Header.Get("ETag"), resp.Header.Get("Last-Modified"), nil
}
func head(ctx context.Context, client *http.Client, url string) (string, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return "", ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", ""
	}
	return resp.Header.Get("ETag"), resp.Header.Get("Last-Modified")
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
	Remaining      int64
	NextEligibleAt time.Time
	HasMore        bool
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
		body, _, _, err := get(ctx, client, job.SourceUrl)
		if err != nil {
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

func requireAssetLeaseUpdate(action string, rows int64, err error) error {
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("%s: lease was lost", action)
	}
	return nil
}
