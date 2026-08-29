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
	timeout := flag.Duration("timeout", 30*time.Minute, "maximum rebuild duration")
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
		var r storage.ChampionDetailRollupRefresh
		r, err = store.RebuildChampionDetailRollups(ctx)
		if err == nil {
			fmt.Printf("source_matches=%d aggregate_rows=%d duration=%s\n", r.SourceMatches, r.AggregateRows, r.Duration.Round(time.Millisecond))
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
