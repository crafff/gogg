// asset-sync prepares one CommunityDragon patch without starting the crawler.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/crafff/gogg/packages/cdragonassets"
	"github.com/crafff/gogg/packages/riotapi"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "asset-sync: %v\n", err)
		os.Exit(1)
	}
}
func run() error {
	version := flag.String("version", "", "CommunityDragon patch, for example 16.14 (default: latest)")
	root := flag.String("root", "data/game-assets", "published asset directory")
	locales := flag.String("locales", "en_us,zh_cn", "comma-separated CommunityDragon locales")
	positions := flag.Bool("positions", true, "download position icons")
	timeout := flag.Duration("timeout", 10*time.Minute, "overall sync timeout")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	resolvedVersion := strings.TrimSpace(*version)
	if resolvedVersion == "" {
		entries, err := riotapi.NewClient("", "", "").GetAllVersions(ctx)
		if err != nil {
			return fmt.Errorf("detect latest version: %w", err)
		}
		var latest riotapi.VersionEntry
		for _, entry := range entries {
			if entry.PatchStartAt.After(latest.PatchStartAt) {
				latest = entry
			}
		}
		if latest.Version == "" {
			return fmt.Errorf("detect latest version: CommunityDragon returned no patches")
		}
		resolvedVersion = latest.Version
	}
	manifest, err := cdragonassets.Sync(ctx, cdragonassets.Options{Root: *root, Version: resolvedVersion, Locales: strings.Split(*locales, ","), Positions: *positions})
	if err != nil {
		return err
	}
	fmt.Printf("version=%s champions=%d locales=%s root=%s\n", manifest.Version, len(manifest.Champions), strings.Join(manifest.Locales, ","), *root)
	return nil
}
