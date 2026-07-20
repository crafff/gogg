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

type Options struct {
	Root      string
	Version   string
	Locales   []string
	BaseURL   string
	Client    *http.Client
	Positions bool
}

type Champion struct {
	ID    int               `json:"id"`
	Names map[string]string `json:"names"`
	Image string            `json:"image"`
}

type Manifest struct {
	Version    string              `json:"version"`
	PreparedAt time.Time           `json:"preparedAt"`
	Locales    []string            `json:"locales"`
	Champions  map[string]Champion `json:"champions"`
	Positions  map[string]string   `json:"positions,omitempty"`
}

type championSummary struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
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
	if manifest, err := readManifest(filepath.Join(finalDir, "manifest.json")); err == nil {
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

	manifest := Manifest{Version: opts.Version, PreparedAt: time.Now().UTC(), Locales: opts.Locales, Champions: map[string]Champion{}}
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
	if err := os.Rename(stage, finalDir); err != nil {
		if existing, readErr := readManifest(filepath.Join(finalDir, "manifest.json")); readErr == nil {
			return existing, nil
		}
		return Manifest{}, fmt.Errorf("publish assets: %w", err)
	}
	if err := writeJSONAtomic(filepath.Join(opts.Root, "latest.json"), manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
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
