// Package testutil provides helpers for integration tests that need a real database.
package testutil

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/crafff/gogg/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultDSN = "postgres://gogg:goggpass@localhost:55433/gogg?sslmode=disable"

// NewTestStore creates a Store backed by a freshly-migrated temporary schema.
// The schema is created under the same database as TEST_DATABASE_DSN (or the
// default DSN), isolated from the public schema. It is dropped automatically
// when the test ends.
func NewTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}
	ctx := context.Background()
	schema := fmt.Sprintf("goggtest_%d", time.Now().UnixNano())

	// Create the schema using the base DSN.
	adminPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("testutil: connect: %v", err)
	}
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		adminPool.Close()
		t.Fatalf("testutil: create schema %s: %v", schema, err)
	}
	adminPool.Close()

	store, err := storage.New(ctx, withSearchPath(dsn, schema), 5, 1, 60)
	if err != nil {
		dropSchema(t, dsn, schema)
		t.Fatalf("testutil: new store: %v", err)
	}
	if err := store.InitSchema(ctx); err != nil {
		store.Close()
		dropSchema(t, dsn, schema)
		t.Fatalf("testutil: init schema: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
		dropSchema(t, dsn, schema)
	})
	return store
}

func withSearchPath(dsn, schema string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		panic("testutil: invalid DSN: " + err.Error())
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}

// NewPersistentStore creates a Store with its own schema but does NOT register
// cleanup — the schema survives after the test. Use the inttest-cleanup program
// to drop it manually.
func NewPersistentStore(t *testing.T, schema string) *storage.Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("testutil: connect: %v", err)
	}
	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+schema); err != nil {
		pool.Close()
		t.Fatalf("testutil: create schema %s: %v", schema, err)
	}
	pool.Close()

	store, err := storage.New(ctx, withSearchPath(dsn, schema), 5, 1, 60)
	if err != nil {
		t.Fatalf("testutil: new store: %v", err)
	}
	if err := store.InitSchema(ctx); err != nil {
		store.Close()
		t.Fatalf("testutil: init schema: %v", err)
	}

	t.Cleanup(store.Close)
	return store
}

func dropSchema(t *testing.T, dsn, schema string) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Logf("testutil: cleanup connect failed: %v", err)
		return
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
		t.Logf("testutil: drop schema %s: %v", schema, err)
	}
}