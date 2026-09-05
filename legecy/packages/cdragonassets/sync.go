// Package cdragonassets prepares versioned, frontend-ready CommunityDragon assets.
package cdragonassets

import (
	"context"
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
)

const DefaultBaseURL = "https://raw.communitydragon.org"

const (
	manifestSchemaVersion = 4
	maxProfileIconBytes   = 4 << 20
)

type Options struct {
	Root      string
	Version   string
	Locales   []string
	BaseURL   string
	Client    *http.Client
	Positions bool
	// PreserveLatest publishes the versioned assets without changing latest.json.
	// This is useful when backfilling assets for an older patch.
	PreserveLatest bool
}

type Champion struct {
	ID    int               `json:"id"`
	Names map[string]string `json:"names"`
	Image string            `json:"image"`
}

type Asset struct {
	ID    int               `json:"id"`
	Names map[string]string `json:"names"`
	Image string            `json:"image"`
}

type Manifest struct {
	SchemaVersion  int                 `json:"schemaVersion"`
	Version        string              `json:"version"`
	PreparedAt     time.Time           `json:"preparedAt"`
	Locales        []string            `json:"locales"`
	Champions      map[string]Champion `json:"champions"`
	Positions      map[string]string   `json:"positions,omitempty"`
	Items          map[string]Asset    `json:"items"`
	SummonerSpells map[string]Asset    `json:"summonerSpells"`
	Perks          map[string]Asset    `json:"perks"`
}

type championSummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type assetSummary struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	IconPath string `json:"iconPath"`
}

type perkStylesPayload struct {
	Styles []assetSummary `json:"styles"`
}

func Sync(ctx context.Context, opts Options) (Manifest, error) {
	if strings.TrimSpace(opts.Root) == "" || strings.TrimSpace(opts.Version) == "" {
		return Manifest{}, errors.New("asset root and version are required")
	}
	if len(opts.Locales) == 0 {
		opts.Locales = []string{"en_us", "zh_cn"}
	}
	for i := range opts.Locales {
		opts.Locales[i] = strings.ToLower(strings.TrimSpace(opts.Locales[i]))
	}
	sort.Strings(opts.Locales)
	if opts.BaseURL == "" {
		opts.BaseURL = DefaultBaseURL
	}
	opts.BaseURL = strings.TrimRight(opts.BaseURL, "/")
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 30 * time.Second}
	}

	finalDir := filepath.Join(opts.Root, opts.Version)
	if manifest, err := readManifest(filepath.Join(finalDir, "manifest.json")); err == nil && manifest.SchemaVersion >= manifestSchemaVersion {
		if !opts.PreserveLatest {
			if err := writeJSONAtomic(filepath.Join(opts.Root, "latest.json"), manifest); err != nil {
				return Manifest{}, err
			}
		}
		return manifest, nil
	}
	if err := os.MkdirAll(opts.Root, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("create asset root: %w", err)
	}
	stage, err := os.MkdirTemp(opts.Root, ".staging-"+opts.Version+"-")
	if err != nil {
		return Manifest{}, fmt.Errorf("create staging directory: %w", err)
	}
	defer os.RemoveAll(stage)

	manifest := Manifest{SchemaVersion: manifestSchemaVersion, Version: opts.Version, PreparedAt: time.Now().UTC(), Locales: opts.Locales, Champions: map[string]Champion{}, Items: map[string]Asset{}, SummonerSpells: map[string]Asset{}, Perks: map[string]Asset{}}
	for _, locale := range opts.Locales {
		sourceLocale := locale
		if locale == "en_us" {
			sourceLocale = "default"
		}
		url := fmt.Sprintf("%s/%s/plugins/rcp-be-lol-game-data/global/%s/v1/champion-summary.json", opts.BaseURL, opts.Version, sourceLocale)
		var summaries []championSummary
		if err := getJSON(ctx, opts.Client, url, &summaries); err != nil {
			return Manifest{}, fmt.Errorf("fetch champions for %s: %w", locale, err)
		}
		for _, summary := range summaries {
			if summary.ID <= 0 || summary.Name == "" { // summary contains a synthetic id=-1 entry.
				continue
			}
			key := strconv.Itoa(summary.ID)
			champion := manifest.Champions[key]
			champion.ID = summary.ID
			if champion.Names == nil {
				champion.Names = map[string]string{}
			}
			champion.Names[locale] = summary.Name
			champion.Image = "champions/" + key + ".png"
			manifest.Champions[key] = champion
		}
	}
	for _, spec := range []struct {
		name, endpoint, dir string
		target              map[string]Asset
	}{
		{"item", "items.json", "items", manifest.Items},
		{"summoner spell", "summoner-spells.json", "spells", manifest.SummonerSpells},
		{"perk", "perks.json", "perks", manifest.Perks},
	} {
		if err := os.MkdirAll(filepath.Join(stage, spec.dir), 0o755); err != nil {
			return Manifest{}, err
		}
		for _, locale := range opts.Locales {
			sourceLocale := locale
			if locale == "en_us" {
				sourceLocale = "default"
			}
			url := fmt.Sprintf("%s/%s/plugins/rcp-be-lol-game-data/global/%s/v1/%s", opts.BaseURL, opts.Version, sourceLocale, spec.endpoint)
			var entries []assetSummary
			if err := getJSON(ctx, opts.Client, url, &entries); err != nil {
				return Manifest{}, fmt.Errorf("fetch %s assets for %s: %w", spec.name, locale, err)
			}
			for _, entry := range entries {
				// Match inventories can contain transformed and quest-reward items
				// (for example Seraph's Embrace or upgraded support items) that are
				// intentionally not sold directly and therefore have inStore=false.
				// They still need local icons for match-history rendering.
				if entry.ID <= 0 || entry.ID > 2147483647 || entry.Name == "" || entry.IconPath == "" {
					continue
				}
				key := strconv.Itoa(entry.ID)
				a, existed := spec.target[key]
				a.ID = entry.ID
				if a.Names == nil {
					a.Names = map[string]string{}
				}
				a.Names[locale] = entry.Name
				a.Image = spec.dir + "/" + key + ".png"
				spec.target[key] = a
				if locale == opts.Locales[0] && !existed {
					iconURL := cdragonAssetURL(opts.BaseURL, opts.Version, entry.IconPath)
					if err := download(ctx, opts.Client, iconURL, filepath.Join(stage, a.Image)); err != nil {
						return Manifest{}, fmt.Errorf("fetch %s %d image: %w", spec.name, entry.ID, err)
					}
				}
			}
		}
	}
	// perks.json contains individual rune choices but not the five rune-tree
	// icons (8000, 8100, ...). Match history stores the secondary tree ID, so
	// merge perkstyles.json into the same local perks namespace.
	for _, locale := range opts.Locales {
		sourceLocale := locale
		if locale == "en_us" {
			sourceLocale = "default"
		}
		url := fmt.Sprintf("%s/%s/plugins/rcp-be-lol-game-data/global/%s/v1/perkstyles.json", opts.BaseURL, opts.Version, sourceLocale)
		var payload perkStylesPayload
		if err := getJSON(ctx, opts.Client, url, &payload); err != nil {
			return Manifest{}, fmt.Errorf("fetch perk styles for %s: %w", locale, err)
		}
		for _, entry := range payload.Styles {
			if entry.ID <= 0 || entry.Name == "" || entry.IconPath == "" {
				continue
			}
			key := strconv.Itoa(entry.ID)
			a, existed := manifest.Perks[key]
			a.ID = entry.ID
			if a.Names == nil {
				a.Names = map[string]string{}
			}
			a.Names[locale] = entry.Name
			a.Image = "perks/" + key + ".png"
			manifest.Perks[key] = a
			if locale == opts.Locales[0] && !existed {
				iconURL := cdragonAssetURL(opts.BaseURL, opts.Version, entry.IconPath)
				if err := download(ctx, opts.Client, iconURL, filepath.Join(stage, a.Image)); err != nil {
					return Manifest{}, fmt.Errorf("fetch perk style %d image: %w", entry.ID, err)
				}
			}
		}
	}
	if len(manifest.Champions) == 0 {
		return Manifest{}, errors.New("CommunityDragon returned no champions")
	}
	if err := os.MkdirAll(filepath.Join(stage, "champions"), 0o755); err != nil {
		return Manifest{}, err
	}
	for id, champion := range manifest.Champions {
		if len(champion.Names) != len(opts.Locales) {
			return Manifest{}, fmt.Errorf("champion %s missing a configured locale", id)
		}
		url := fmt.Sprintf("%s/%s/plugins/rcp-be-lol-game-data/global/default/v1/champion-icons/%s.png", opts.BaseURL, opts.Version, id)
		if err := download(ctx, opts.Client, url, filepath.Join(stage, champion.Image)); err != nil {
			return Manifest{}, fmt.Errorf("fetch champion %s image: %w", id, err)
		}
	}
	if opts.Positions {
		manifest.Positions = map[string]string{}
		if err := os.MkdirAll(filepath.Join(stage, "positions"), 0o755); err != nil {
			return Manifest{}, err
		}
		for _, position := range []string{"top", "jungle", "middle", "bottom", "utility"} {
			rel := "positions/" + position + ".png"
			url := fmt.Sprintf("%s/%s/plugins/rcp-fe-lol-clash/global/default/assets/images/position-selector/positions/icon-position-%s.png", opts.BaseURL, opts.Version, position)
			if err := download(ctx, opts.Client, url, filepath.Join(stage, rel)); err != nil {
				return Manifest{}, fmt.Errorf("fetch position %s: %w", position, err)
			}
			manifest.Positions[position] = rel
		}
	}
	if err := writeJSON(filepath.Join(stage, "manifest.json"), manifest); err != nil {
		return Manifest{}, err
	}
	// MkdirTemp creates the staging root with mode 0700. Published assets are
	// commonly mounted into an API or web container running as a different
	// non-root UID, so the version directory itself must be traversable.
	if err := os.Chmod(stage, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("make published asset directory readable: %w", err)
	}
	backup := finalDir + ".previous"
	_ = os.RemoveAll(backup)
	if _, err := os.Stat(finalDir); err == nil {
		if err := os.Rename(finalDir, backup); err != nil {
			return Manifest{}, err
		}
	}
	if err := os.Rename(stage, finalDir); err != nil {
		_ = os.Rename(backup, finalDir)
		if existing, readErr := readManifest(filepath.Join(finalDir, "manifest.json")); readErr == nil {
			return existing, nil
		}
		return Manifest{}, fmt.Errorf("publish assets: %w", err)
	}
	_ = os.RemoveAll(backup)
	if !opts.PreserveLatest {
		if err := writeJSONAtomic(filepath.Join(opts.Root, "latest.json"), manifest); err != nil {
			return Manifest{}, err
		}
	}
	return manifest, nil
}

func cdragonAssetURL(baseURL, version, iconPath string) string {
	p := strings.ToLower(strings.TrimSpace(iconPath))
	p = strings.TrimPrefix(p, "/lol-game-data/assets")
	return fmt.Sprintf("%s/%s/plugins/rcp-be-lol-game-data/global/default%s", baseURL, version, p)
}

// CacheProfileIcon downloads one CommunityDragon profile icon into the shared
// local asset tree. Profile icon IDs are account-specific and number in the
// thousands, so eagerly downloading the entire catalog would waste substantial
// disk space. The atomic rename makes concurrent first requests safe.
func CacheProfileIcon(ctx context.Context, root string, id int, baseURL string, client *http.Client) (string, error) {
	if strings.TrimSpace(root) == "" || id <= 0 {
		return "", errors.New("asset root and positive profile icon ID are required")
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	dir := filepath.Join(root, "profile-icons")
	path := filepath.Join(dir, strconv.Itoa(id)+".jpg")
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create profile icon cache: %w", err)
	}
	url := fmt.Sprintf("%s/latest/plugins/rcp-be-lol-game-data/global/default/v1/profile-icons/%d.jpg", baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("profile icon %d: HTTP %d", id, resp.StatusCode)
	}
	tmp, err := os.CreateTemp(dir, ".profile-icon-*.tmp")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	written, copyErr := io.Copy(tmp, io.LimitReader(resp.Body, maxProfileIconBytes+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if written == 0 || written > maxProfileIconBytes {
		return "", fmt.Errorf("profile icon %d has invalid size %d", id, written)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		if info, statErr := os.Stat(path); statErr == nil && info.Size() > 0 {
			return path, nil
		}
		return "", err
	}
	return path, nil
}

func getJSON(ctx context.Context, client *http.Client, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func download(ctx context.Context, client *http.Client, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func readManifest(path string) (Manifest, error) {
	var value Manifest
	f, err := os.Open(path)
	if err != nil {
		return value, err
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(&value)
	return value, err
}
func writeJSON(path string, value any) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	encErr := json.NewEncoder(f).Encode(value)
	closeErr := f.Close()
	if encErr != nil {
		return encErr
	}
	return closeErr
}
func writeJSONAtomic(path string, value any) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(path), ".manifest-*.tmp")
	if err != nil {
		return err
	}
	tmp := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(tmp)
	if err := writeJSON(tmp, value); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
