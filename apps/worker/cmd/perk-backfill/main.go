// perk-backfill repairs historical match_perks rows produced by the old
// 3+2 indexing bug. It refetches Match V5 detail and replaces only perks.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/config"
	"github.com/crafff/gogg/apps/worker/internal/crawler/phase3"
	"github.com/crafff/gogg/apps/worker/internal/runtime"
	"github.com/crafff/gogg/apps/worker/internal/storage"
	"github.com/crafff/gogg/packages/riotapi"
)

type options struct {
	region           string
	batchSize, limit int
	dryRun           bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "perk-backfill: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var opts options
	flag.StringVar(&opts.region, "region", "", "optional region filter, for example KR")
	flag.IntVar(&opts.batchSize, "batch-size", 100, "database keyset batch size")
	flag.IntVar(&opts.limit, "limit", 0, "maximum matches to attempt; 0 means all")
	flag.BoolVar(&opts.dryRun, "dry-run", false, "count pending matches without calling Riot or writing")
	flag.Parse()
	if opts.batchSize < 1 || opts.batchSize > 1000 {
		return fmt.Errorf("batch-size must be between 1 and 1000")
	}
	if opts.limit < 0 {
		return fmt.Errorf("limit must be non-negative")
	}
	opts.region = strings.ToUpper(strings.TrimSpace(opts.region))
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	rt, err := runtime.Build(ctx, cfg)
	if err != nil {
		return fmt.Errorf("build runtime: %w", err)
	}
	defer rt.Close()
	total, err := rt.Store.CountMatchesNeedingPerkBackfill(ctx, opts.region)
	if err != nil {
		return err
	}
	fmt.Printf("pending=%d region=%s dry_run=%t\n", total, displayRegion(opts.region), opts.dryRun)
	if opts.dryRun || total == 0 {
		return nil
	}
	started := time.Now()
	after := ""
	attempted, repaired, failed := 0, 0, 0
	for {
		batchLimit := opts.batchSize
		if opts.limit > 0 && opts.limit-attempted < batchLimit {
			batchLimit = opts.limit - attempted
		}
		if batchLimit <= 0 {
			break
		}
		matches, err := rt.Store.ListMatchesNeedingPerkBackfill(ctx, opts.region, after, batchLimit)
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			break
		}
		for _, match := range matches {
			if err := ctx.Err(); err != nil {
				return err
			}
			after = match.MatchID
			attempted++
			if err := repairMatch(ctx, rt, match); err != nil {
				failed++
				fmt.Fprintf(os.Stderr, "match_failed match_id=%s region=%s error=%q\n", match.MatchID, match.Region, err)
				if riotapi.IsGlobalPermanent(err) {
					return err
				}
			} else {
				repaired++
			}
			if attempted%25 == 0 {
				printProgress(attempted, repaired, failed, total, started)
			}
		}
	}
	printProgress(attempted, repaired, failed, total, started)
	if failed > 0 {
		return fmt.Errorf("%d matches failed; rerun to retry remaining rows", failed)
	}
	return nil
}

func repairMatch(ctx context.Context, rt *runtime.Runtime, match storage.PerkBackfillMatch) error {
	client, err := rt.RiotForRegion(match.Region)
	if err != nil {
		return err
	}
	detail, err := client.GetMatchDetail(ctx, match.MatchID)
	if err != nil {
		return err
	}
	rows, err := perkRows(match.MatchID, detail)
	if err != nil {
		return err
	}
	return rt.Store.ReplaceMatchPerks(ctx, match.MatchID, rows)
}

func perkRows(matchID string, detail *riotapi.MatchDetailDTO) ([]storage.PerkRow, error) {
	if detail == nil {
		return nil, errors.New("nil match detail")
	}
	if detail.Metadata.MatchID != "" && detail.Metadata.MatchID != matchID {
		return nil, fmt.Errorf("response match id %q does not match %q", detail.Metadata.MatchID, matchID)
	}
	if len(detail.Info.Participants) != 10 {
		return nil, fmt.Errorf("got %d participants, want 10", len(detail.Info.Participants))
	}
	rows := make([]storage.PerkRow, 0, 10)
	seen := make(map[string]struct{}, 10)
	for _, participant := range detail.Info.Participants {
		if participant.Puuid == "" {
			return nil, errors.New("participant has empty puuid")
		}
		if _, ok := seen[participant.Puuid]; ok {
			return nil, fmt.Errorf("duplicate participant puuid %q", participant.Puuid)
		}
		seen[participant.Puuid] = struct{}{}
		row := phase3.PerkRowFromDTO(matchID, participant.Puuid, &participant.Perks)
		for i, id := range row.Perk {
			if id <= 0 {
				return nil, fmt.Errorf("participant %q perk%d is missing", participant.Puuid, i)
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func printProgress(attempted, repaired, failed int, total int64, started time.Time) {
	elapsed := time.Since(started)
	rate := float64(attempted) / elapsed.Seconds()
	remaining := time.Duration(0)
	if rate > 0 && int64(attempted) < total {
		remaining = time.Duration(float64(total-int64(attempted))/rate) * time.Second
	}
	fmt.Printf("attempted=%d repaired=%d failed=%d total=%d rate=%.2f_matches_per_second eta=%s\n", attempted, repaired, failed, total, rate, remaining.Round(time.Second))
}
func displayRegion(region string) string {
	if region == "" {
		return "ALL"
	}
	return region
}
