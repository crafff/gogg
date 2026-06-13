package crawl

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/crafff/gogg/internal/config"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

func newDaemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the crawler as a daemon using schedule from config.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cleanup := setupLogger("daemon")
			defer cleanup()

			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			if len(cfg.Schedule) == 0 {
				return fmt.Errorf("no schedule entries found in config.yaml")
			}

			c := cron.New()
			for _, entry := range cfg.Schedule {
				e := entry // capture
				_, err := c.AddFunc(e.Cron, func() {
					slog.Info("daemon: triggering scheduled run", "profile", e.Profile, "cron", e.Cron)
					if err := runProfile(cfg, e.Profile); err != nil {
						slog.Error("daemon: run failed", "profile", e.Profile, "err", err)
					}
				})
				if err != nil {
					return fmt.Errorf("invalid cron expression %q: %w", e.Cron, err)
				}
				slog.Info("daemon: scheduled", "profile", e.Profile, "cron", e.Cron)
			}

			c.Start()
			slog.Info("daemon started, waiting for scheduled runs (Ctrl+C to stop)")

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			<-quit

			slog.Info("daemon stopping...")
			c.Stop()
			return nil
		},
	}
}

func runProfile(cfg *config.Config, profileName string) error {
	profile, err := cfg.Profile(profileName)
	if err != nil {
		return err
	}
	if err := profile.Validate(); err != nil {
		return err
	}

	ctx := context.Background()
	store, err := newStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	regionCfg, err := cfg.RegionByName(profile.Region)
	if err != nil {
		return err
	}
	riot := newRiotClientForRegion(cfg, regionCfg)
	runner := buildRunner(store, riot, profile.Execution)
	return runner.Run(ctx, &profileName, profile, 0)
}
