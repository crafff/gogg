package main

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestTransientDatabaseErrorClassification(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "network", err: &net.DNSError{Err: "network unreachable", Name: "db", IsTemporary: true}, want: true},
		{name: "sql state", err: &pgconn.PgError{Code: "23505", Message: "duplicate key"}, want: false},
		{name: "cancelled", err: context.Canceled, want: false},
		{name: "ordinary", err: errors.New("bad configuration"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransientDatabaseError(tt.err); got != tt.want {
				t.Fatalf("isTransientDatabaseError()=%v want %v", got, tt.want)
			}
		})
	}
}

func TestWaitForRetryCanBeCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	err := waitForRetry(ctx, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("retry wait did not stop promptly")
	}
}
