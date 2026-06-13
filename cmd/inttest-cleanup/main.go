// inttest-cleanup drops the crawl_inttest schema created by TestFullPipelineReduced.
//
// Usage:
//
//	go run ./cmd/inttest-cleanup/
//
// Reads TEST_DATABASE_DSN (or falls back to the default local DSN).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	schema     = "crawl_inttest"
	defaultDSN = "postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable"
)

func main() {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Check schema exists before dropping.
	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)`, schema,
	).Scan(&exists); err != nil {
		fmt.Fprintf(os.Stderr, "check schema: %v\n", err)
		os.Exit(1)
	}

	if !exists {
		fmt.Printf("schema %q does not exist, nothing to do\n", schema)
		return
	}

	// Count rows in key tables so the user can see what they're deleting.
	for _, tbl := range []string{"matches", "match_participants", "player_rank_snapshots", "runs"} {
		var n int
		if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", schema, tbl)).Scan(&n); err != nil {
			fmt.Fprintf(os.Stderr, "  %s.%s count: %v\n", schema, tbl, err)
			continue
		}
		fmt.Printf("  %s.%-28s %d rows\n", schema, tbl, n)
	}

	fmt.Printf("\nDropping schema %q ... ", schema)
	if _, err := pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
		fmt.Fprintf(os.Stderr, "failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("done.")
}
