package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// memCache is a hand-rolled in-memory Cache for tests. Avoids needing
// a real Redis (or testcontainers) to exercise GetOrLoad's semantics.
// Concurrency safe so the singleflight test is real.
type memCache struct {
	mu     sync.Mutex
	data   map[string][]byte
	getErr error
}

func newMemCache() *memCache {
	return &memCache{data: make(map[string][]byte)}
}

func (m *memCache) GetJSON(_ context.Context, key string, dst any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getErr != nil {
		return m.getErr
	}
	b, ok := m.data[key]
	if !ok {
		return ErrMiss
	}
	return unmarshal(b, dst)
}

func (m *memCache) SetJSON(_ context.Context, key string, value any, _ time.Duration) error {
	b, err := marshal(value)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = b
	return nil
}

func (m *memCache) Delete(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.data, k)
	}
	return nil
}

func (m *memCache) Ping(_ context.Context) error { return nil }

func marshal(v any) ([]byte, error)   { return jsonMarshal(v) }
func unmarshal(b []byte, v any) error { return jsonUnmarshal(b, v) }

func TestGetOrLoad_missThenHit(t *testing.T) {
	c := newMemCache()
	var calls int32
	loader := func(_ context.Context) (string, error) {
		atomic.AddInt32(&calls, 1)
		return "loaded", nil
	}

	val, hit, err := GetOrLoad(context.Background(), c, "k", time.Minute, loader)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if hit {
		t.Error("first call should be a miss")
	}
	if val != "loaded" {
		t.Errorf("val = %q", val)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("loader called %d times", calls)
	}

	val2, hit2, err := GetOrLoad(context.Background(), c, "k", time.Minute, loader)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !hit2 {
		t.Error("second call should be a hit")
	}
	if val2 != "loaded" {
		t.Errorf("val2 = %q", val2)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("loader called twice when it should have hit cache; calls = %d", calls)
	}
}

func TestGetOrLoad_loaderErrorIsNotCached(t *testing.T) {
	c := newMemCache()
	want := errors.New("boom")
	loader := func(_ context.Context) (string, error) { return "", want }

	_, _, err := GetOrLoad(context.Background(), c, "k", time.Minute, loader)
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want errors.Is(_, want)", err)
	}
	// Cache should be untouched after a loader failure.
	if _, ok := c.data["k"]; ok {
		t.Error("loader error wrote a value to the cache")
	}
}

func TestGetOrLoad_realRedisFlavour_genericTypePreserved(t *testing.T) {
	type Result struct {
		Items []int
		Total int
	}
	c := newMemCache()
	loader := func(_ context.Context) (Result, error) {
		return Result{Items: []int{1, 2, 3}, Total: 100}, nil
	}

	got, _, err := GetOrLoad(context.Background(), c, "k", time.Minute, loader)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Total != 100 || len(got.Items) != 3 {
		t.Errorf("got = %+v", got)
	}

	hit, _, err := GetOrLoad(context.Background(), c, "k", time.Minute, loader)
	if err != nil {
		t.Fatalf("hit err = %v", err)
	}
	if hit.Total != 100 || hit.Items[2] != 3 {
		t.Errorf("hit = %+v (round-trip JSON didn't preserve struct)", hit)
	}
}

func TestGetOrLoad_redisGetErrorFallsThroughToLoader(t *testing.T) {
	c := newMemCache()
	c.getErr = errors.New("network blip")

	want := "fell-through"
	loader := func(_ context.Context) (string, error) { return want, nil }

	got, _, err := GetOrLoad(context.Background(), c, "k", time.Minute, loader)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != want {
		t.Errorf("got = %q, want %q (loader should run when GET fails)", got, want)
	}
}
