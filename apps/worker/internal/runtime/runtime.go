// Package runtime builds the long-lived resources the crawl Activities
// share: a database pool, a per-region Riot API client, and a handle
// onto the crawler config for profile lookup by name from inside a
// Workflow.
//
// Activities receive a pointer to *Runtime via their receiver struct;
// they MUST treat its fields as read-only after Build returns.
package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/crafff/gogg/apps/worker/internal/config"
	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
)

// Runtime is the worker process's shared state. Constructed once in
// main, registered on every Activity struct.
type Runtime struct {
	Cfg   *config.Config
	Store *storage.Store
	// Riot maps an upper-cased region name (KR, NA1, …) to its Client.
	// Each client has its own RateLimiter so per-region budgets stay
	// isolated even when one worker process serves multiple regions.
	Riot map[string]*riotapi.Client
}

// Build loads the crawler config, opens the DB pool, and spins up
// one Riot client per configured region. Closes any partially-built
// resources on failure so callers never see a half-initialised Runtime.
func Build(ctx context.Context, cfg config.Config) (*Runtime, error) {
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

	return &Runtime{Cfg: &cfg, Store: store, Riot: clients}, nil
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

// resolvedRegions returns the effective region list. Empty per-region
// API keys inherit the global Riot API key, and a single KR region is
// synthesized for older single-region config files.
func resolvedRegions(cfg config.Config) ([]config.RegionConfig, error) {
	regions := cfg.ResolvedRegions()
	if len(regions) == 1 && regions[0].BaseURL == "" {
		return nil, fmt.Errorf("no regions configured and riot.base_url is unset")
	}
	return regions, nil
}

// regionalRoutingURL maps a platform base URL to its regional routing
// URL for account and match-history APIs.
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
