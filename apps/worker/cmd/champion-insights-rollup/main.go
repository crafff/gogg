// champion-insights-rollup rebuilds the additive histograms consumed by the
// champion win-factor API. It is intentionally independent from region crawl
// workflows so one regional completion cannot trigger repeated global scans.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/storage"
)

func main() {
	dsn := flag.String("database-dsn", os.Getenv("GOGG_DATABASE_DSN"), "PostgreSQL DSN")
	timeout := flag.Duration("timeout", 2*time.Hour, "maximum rebuild duration")
	flag.Parse()
	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "database DSN is required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	store, err := storage.New(ctx, *dsn, 2, 0, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()
	if err = store.InitSchema(ctx); err == nil {
		var r storage.ChampionWinFactorRollupRefresh
		r, err = store.RebuildChampionWinFactorRollups(ctx)
		if err == nil {
			fmt.Printf("publication_id=%d revision=%s reused=%t source_matches=%d eligible_matches=%d source_participants=%d cohorts=%d factors=%d buckets=%d duration=%s\n",
				r.PublicationID, r.Revision, r.Reused, r.SourceMatches, r.EligibleMatches,
				r.SourceParticipants, r.CohortRows, r.FactorRows, r.BucketRows,
				r.Duration.Round(time.Millisecond))
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
