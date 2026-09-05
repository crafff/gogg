// rankings-rollup rebuilds the narrow PostgreSQL rollups used by the API's
// champion rankings read path.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crafff/gogg/apps/worker/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "rankings-rollup: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dsnDefault := os.Getenv("GOGG_DATABASE_DSN")
	dsn := flag.String("database-dsn", dsnDefault, "PostgreSQL DSN (or set GOGG_DATABASE_DSN)")
	timeout := flag.Duration("timeout", 30*time.Minute, "maximum rebuild duration")
	flag.Parse()
	if *dsn == "" {
		return fmt.Errorf("--database-dsn or GOGG_DATABASE_DSN is required")
	}
	if *timeout <= 0 {
		return fmt.Errorf("--timeout must be positive")
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, *timeout)
	defer cancel()

	store, err := storage.New(ctx, *dsn, 2, 0, 0)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.InitSchema(ctx); err != nil {
		return err
	}

	result, err := store.RebuildRankingsRollups(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("refreshed_at=%s data_through=%s source_completed_matches=%d eligible_matches=%d excluded_metadata=%d excluded_tier=%d excluded_duration=%d excluded_participant_shape=%d excluded_participant_facts=%d excluded_ban_shape=%d champion_position_rows=%d ban_rows=%d match_count_rows=%d duration=%s\n",
		result.RefreshedAt.Format(time.RFC3339), formatTime(result.DataThrough),
		result.SourceCompletedMatches, result.EligibleMatches,
		result.ExcludedMetadataMatches, result.ExcludedTierMatches,
		result.ExcludedDurationMatches, result.ExcludedParticipantShapeMatches,
		result.ExcludedParticipantFactsMatches, result.ExcludedBanShapeMatches,
		result.ChampionPositionRows,
		result.BanRows, result.MatchCountRows, result.Duration.Round(time.Millisecond))
	return nil
}

func formatTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}
