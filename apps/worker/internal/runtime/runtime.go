// Package runtime builds the long-lived resources the crawl Activities
// share: a database pool, a per-region Riot API client, and a handle
// onto the legacy crawler config (for profile lookup by name from
// inside a Workflow).
//
// Activities receive a pointer to *Runtime via their receiver struct;
// they MUST treat its fields as read-only after Build returns.
package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	legacycfg "github.com/crafff/gogg/internal/config"
	"github.com/crafff/gogg/internal/riotapi"
	"github.com/crafff/gogg/internal/storage"
)

// Runtime is the worker process's shared state. Constructed once in
// main, registered on every Activity struct.
type Runtime struct {
	Cfg   *legacycfg.Config
	Store *storage.Store
	// Riot maps an upper-cased region name (KR, NA1, …) to its Client.
	// Each client has its own RateLimiter so per-region budgets stay
	// isolated even when one worker process serves multiple regions.
	Riot map[string]*riotapi.Client
}

// Build loads the legacy crawler config + opens the DB pool + spins up
// one Riot client per configured region. Closes any partially-built
// resources on failure so callers never see a half-initialised Runtime.
func Build(ctx context.Context, configPath string) (*Runtime, error) {
	cfg, err := legacycfg.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load crawler config %s: %w", configPath, err)
	}

	store, err := storage.New(ctx, cfg.Database.DSN,
		cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns,
		cfg.Database.ConnMaxLifetime)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}

	regions, err := resolvedRegions(cfg)
	if err != nil {
		store.Close()
		return nil, err
	}

	clients := make(map[string]*riotapi.Client, len(regions))
	for _, r := range regions {
		key := strings.ToUpper(r.Name)
		clients[key] = riotapi.NewClient(r.APIKey, r.BaseURL, regionalRoutingURL(r.BaseURL))
		slog.Info("riot_client_built", "region", key, "platform", r.BaseURL)
	}

	return &Runtime{Cfg: cfg, Store: store, Riot: clients}, nil
}

// Close releases the DB pool. Riot clients hold only an http.Client
// and a rate limiter — no explicit cleanup is required.
func (r *Runtime) Close() {
	if r.Store != nil {
		r.Store.Close()
	}
}

// RiotForRegion returns the Riot client for the given region, normalising
// the key. Errors when the region wasn't configured at startup so
// the workflow surfaces a clear failure instead of a nil dereference.
func (r *Runtime) RiotForRegion(region string) (*riotapi.Client, error) {
	key := strings.ToUpper(region)
	c, ok := r.Riot[key]
	if !ok {
		known := make([]string, 0, len(r.Riot))
		for k := range r.Riot {
			known = append(known, k)
		}
		return nil, fmt.Errorf("no riot client for region %q (configured: %v)", region, known)
	}
	return c, nil
}

// resolvedRegions mirrors internal/config.resolvedRegions (unexported)
// so the worker doesn't have to expose that helper publicly. Keeps the
// legacy package untouched while Phase B's main.go and Phase C's worker
// both rely on the same resolution rules.
func resolvedRegions(cfg *legacycfg.Config) ([]legacycfg.RegionConfig, error) {
	if len(cfg.Regions) > 0 {
		out := make([]legacycfg.RegionConfig, len(cfg.Regions))
		for i, r := range cfg.Regions {
			if r.APIKey == "" {
				r.APIKey = cfg.Riot.APIKey
			}
			out[i] = r
		}
		return out, nil
	}
	if cfg.Riot.BaseURL == "" {
		return nil, fmt.Errorf("no regions configured and riot.base_url is unset")
	}
	return []legacycfg.RegionConfig{{
		Name:    "KR",
		APIKey:  cfg.Riot.APIKey,
		BaseURL: cfg.Riot.BaseURL,
	}}, nil
}

// regionalRoutingURL maps a platform base URL to its regional routing
// URL. Duplicated from cmd/crawl/root.go on purpose — the legacy
// package will be deleted at the end of Phase C; pulling the function
// into a shared helper now would force a cmd/crawl edit during this
// chunk, against the "legacy is sacred until its replacement ships"
// rule. We'll fold this back into packages/riotapi when phase C
// chunk 4 wraps up.
func regionalRoutingURL(platformURL string) string {
	p := strings.ToLower(platformURL)
	switch {
	case strings.Contains(p, "kr"), strings.Contains(p, "jp1"):
		return "https://asia.api.riotgames.com"
	case strings.Contains(p, "euw1"), strings.Contains(p, "eun1"),
		strings.Contains(p, "tr1"), strings.Contains(p, "ru"):
		return "https://europe.api.riotgames.com"
	case strings.Contains(p, "br1"), strings.Contains(p, "la1"),
		strings.Contains(p, "la2"), strings.Contains(p, "na1"):
		return "https://americas.api.riotgames.com"
	case strings.Contains(p, "oc1"):
		return "https://sea.api.riotgames.com"
	default:
		return "https://asia.api.riotgames.com"
	}
}
