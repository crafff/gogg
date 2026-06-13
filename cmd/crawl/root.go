package crawl

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/crafff/gogg/internal/config"
	"github.com/crafff/gogg/internal/crawler"
	"github.com/crafff/gogg/internal/crawler/phase0"
	"github.com/crafff/gogg/internal/crawler/phase1"
	"github.com/crafff/gogg/internal/crawler/phase2"
	"github.com/crafff/gogg/internal/crawler/phase3"
	"github.com/crafff/gogg/internal/crawler/phase35"
	"github.com/crafff/gogg/internal/crawler/phase5"
	"github.com/crafff/gogg/internal/crawler/phase55"
	"github.com/crafff/gogg/internal/crawler/phase4"
	"github.com/crafff/gogg/internal/riotapi"
	"github.com/crafff/gogg/internal/storage"
	"github.com/spf13/cobra"
)

var configPath string

// NewRootCmd builds the 'crawl' cobra command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "crawl",
		Short: "Riot data crawler",
	}
	root.PersistentFlags().StringVar(&configPath, "config", "config.yaml", "path to config file")

	root.AddCommand(newRunCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newRunsCmd())
	root.AddCommand(newCancelCmd())
	root.AddCommand(newDaemonCmd())

	return root
}

// loadDeps reads config and creates the DB store.
func loadDeps(ctx context.Context) (*config.Config, *storage.Store, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	store, err := newStore(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	return cfg, store, nil
}

func newStore(ctx context.Context, cfg *config.Config) (*storage.Store, error) {
	store, err := storage.New(ctx, cfg.Database.DSN,
		cfg.Database.MaxOpenConns, cfg.Database.MaxIdleConns,
		cfg.Database.ConnMaxLifetime)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}
	if err := store.InitSchema(ctx); err != nil {
		store.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return store, nil
}

// newRiotClientForRegion creates a Riot API client for the given region.
func newRiotClientForRegion(cfg *config.Config, rc config.RegionConfig) *riotapi.Client {
	apiKey := rc.APIKey
	if apiKey == "" {
		apiKey = cfg.Riot.APIKey
	}
	return riotapi.NewClient(apiKey, rc.BaseURL, riotAPIRegionalURL(rc.BaseURL))
}

// buildRunner wires all phases and the chosen execution strategy into a Runner.
func buildRunner(store *storage.Store, riot *riotapi.Client, execution config.Execution) *crawler.Runner {
	phases := []crawler.Phase{
		phase0.New(riot, store),
		phase1.New(riot, store),
		phase2.New(riot, store),
		phase3.New(riot, store),
		phase35.New(riot, store),
		phase4.New(store),
		phase5.New(riot, store),
		phase55.New(store),
	}

	var strategy crawler.ExecutionStrategy
	if execution == config.ExecutionPipeline {
		strategy = crawler.NewPipelineStrategy()
	} else {
		strategy = &crawler.SequentialStrategy{}
	}

	return crawler.NewRunner(store, riot, phases, strategy)
}

// riotAPIRegionalURL maps a platform base URL to its regional routing URL.
func riotAPIRegionalURL(platformURL string) string {
	switch {
	case contains(platformURL, "kr", "jp1"):
		return "https://asia.api.riotgames.com"
	case contains(platformURL, "euw1", "eun1", "tr1", "ru"):
		return "https://europe.api.riotgames.com"
	case contains(platformURL, "br1", "la1", "la2", "na1"):
		return "https://americas.api.riotgames.com"
	case contains(platformURL, "oc1"):
		return "https://sea.api.riotgames.com"
	default:
		return "https://asia.api.riotgames.com"
	}
}

func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(strings.ToLower(s), sub) {
			return true
		}
	}
	return false
}

// setupLogger configures slog to write to stderr and to logs/YYYY-MM-DD-<region>.log.
func setupLogger(region string) func() {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}

	if err := os.MkdirAll("logs", 0o755); err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
		return func() {}
	}

	logPath := fmt.Sprintf("logs/%s-%s.log", time.Now().Format("2006-01-02"), strings.ToUpper(region))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
		return func() {}
	}

	multi := io.MultiWriter(os.Stderr, f)
	slog.SetDefault(slog.New(slog.NewTextHandler(multi, opts)))
	return func() { f.Close() }
}
