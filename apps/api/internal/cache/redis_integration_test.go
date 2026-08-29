//go:build integration

package cache

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestAllowFixedWindowsDoesNotPartiallyConsumeCounters(t *testing.T) {
	if os.Getenv("GOGG_INTTEST") == "" {
		t.Skip("set GOGG_INTTEST=1 to run Redis integration tests")
	}
	redisURL := os.Getenv("GOGG_TEST_REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	client, err := NewRedis(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	prefix := fmt.Sprintf("gogg-test:fixed-windows:%d", time.Now().UnixNano())
	identityA, identityB, ip := prefix+":identity-a", prefix+":identity-b", prefix+":ip"
	t.Cleanup(func() { _ = client.Delete(context.Background(), identityA, identityB, ip) })
	rules := func(identity string) ([]string, []int, []time.Duration) {
		return []string{identity, ip}, []int{1, 1}, []time.Duration{time.Minute, time.Minute}
	}

	keys, limits, windows := rules(identityA)
	denied, _, err := client.AllowFixedWindows(t.Context(), keys, limits, windows)
	if err != nil || denied != -1 {
		t.Fatalf("first request: denied=%d err=%v", denied, err)
	}

	denied, _, err = client.AllowFixedWindows(t.Context(), keys, limits, windows)
	if err != nil || denied != 0 {
		t.Fatalf("repeated identity: denied=%d err=%v", denied, err)
	}
	if count := client.client.Get(t.Context(), ip).Val(); count != "1" {
		t.Fatalf("identity denial consumed IP counter: count=%q", count)
	}

	keys, limits, windows = rules(identityB)
	denied, _, err = client.AllowFixedWindows(t.Context(), keys, limits, windows)
	if err != nil || denied != 1 {
		t.Fatalf("exhausted IP: denied=%d err=%v", denied, err)
	}
	if exists := client.client.Exists(t.Context(), identityB).Val(); exists != 0 {
		t.Fatalf("IP denial consumed new identity counter: exists=%d", exists)
	}
}
