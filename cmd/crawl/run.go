package crawl

import (
	"context"
	"fmt"
	"strings"

	"github.com/crafff/gogg/internal/config"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var (
		profileName string
		tiers       string
		modeStr     string
		version     string
		execution   string
		region      string
		resume      int
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start or resume a crawler run [DEPRECATED]",
		Long: "DEPRECATED as of Phase C chunk 4 (2026-06-15). The new entry point " +
			"is a CrawlRegionWorkflow execution on gogg-worker — see " +
			"docs/runbooks/crawler-stuck.md or `temporal workflow start " +
			"--task-queue crawl-<region> --type CrawlRegionWorkflow ...`. " +
			"This command is kept for one release cycle as the rollback " +
			"escape hatch and will be removed in the version after Phase C ships.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()

			cfg, store, err := loadDeps(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			// Resolve profile.
			var profile *config.RunProfile
			if profileName != "" {
				profile, err = cfg.Profile(profileName)
				if err != nil {
					return err
				}
			} else {
				if tiers == "" && resume == 0 {
					return fmt.Errorf("provide --profile or --tiers (or --resume to continue an interrupted run)")
				}
				profile = &config.RunProfile{}
			}

			// Apply CLI flag overrides.
			if tiers != "" {
				profile.TargetTiers = strings.Split(tiers, ",")
			}
			if modeStr != "" {
				profile.Mode = config.Mode(modeStr)
			}
			if version != "" {
				profile.Version = version
			}
			if execution != "" {
				profile.Execution = config.Execution(execution)
			}
			if region != "" {
				profile.Region = region
			}

			// When resuming, fill missing region/execution from the stored run.
			// When not resuming, auto-fill region from config.
			if resume != 0 {
				if profile.Region == "" || profile.Execution == "" {
					found, err := store.GetRunByID(ctx, resume)
					if err != nil {
						return err
					}
					if found == nil {
						return fmt.Errorf("run %d not found", resume)
					}
					if profile.Region == "" {
						profile.Region = found.Region
					}
					if profile.Execution == "" {
						profile.Execution = config.Execution(found.Execution)
					}
				}
			} else if profile.Region == "" {
				regions := cfg.Regions
				if len(regions) == 0 {
					profile.Region = "KR"
				} else if len(regions) == 1 {
					profile.Region = regions[0].Name
				} else {
					return fmt.Errorf("--region is required when config has multiple regions")
				}
			}

			// Skip validation when resuming – the runner restores profile from the stored run.
			if resume == 0 {
				if err := profile.Validate(); err != nil {
					return err
				}
			}

			cleanup := setupLogger(profile.Region)
			defer cleanup()

			regionCfg, err := cfg.RegionByName(profile.Region)
			if err != nil {
				return err
			}
			riot := newRiotClientForRegion(cfg, regionCfg)
			runner := buildRunner(store, riot, profile.Execution)

			var pName *string
			if profileName != "" {
				pName = &profileName
			}
			return runner.Run(ctx, pName, profile, resume)
		},
	}

	cmd.Flags().StringVar(&profileName, "profile", "", "named run profile from config.yaml")
	cmd.Flags().StringVar(&tiers, "tiers", "", "comma-separated target tiers (e.g. CHALLENGER,GRANDMASTER)")
	cmd.Flags().StringVar(&modeStr, "mode", "", "incremental or historical")
	cmd.Flags().StringVar(&version, "version", "", "game version for historical mode (e.g. 14.10)")
	cmd.Flags().StringVar(&execution, "execution", "", "pipeline or sequential")
	cmd.Flags().StringVar(&region, "region", "", "region name (e.g. KR, NA1, EUW1)")
	cmd.Flags().IntVar(&resume, "resume", 0, "resume a specific run ID (0 = auto-detect)")

	return cmd
}
