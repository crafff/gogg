package cdragonassets

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncPublishesCompleteVersionAndIsIdempotent(t *testing.T) {
	t.Parallel()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch {
		case strings.HasSuffix(r.URL.Path, "/default/v1/champion-summary.json"):
			fmt.Fprint(w, `[{"id":-1,"name":"None"},{"id":1,"name":"Annie"}]`)
		case strings.HasSuffix(r.URL.Path, "/zh_cn/v1/champion-summary.json"):
			fmt.Fprint(w, `[{"id":1,"name":"安妮"}]`)
		case strings.HasSuffix(r.URL.Path, "/champion-icons/1.png"):
			fmt.Fprint(w, "png")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	opts := Options{Root: root, Version: "16.14", Locales: []string{"zh_cn", "en_us"}, BaseURL: server.URL}
	manifest, err := Sync(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := manifest.Champions["1"].Names["zh_cn"]; got != "安妮" {
		t.Fatalf("zh name = %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "16.14", "champions", "1.png")); err != nil {
		t.Fatal(err)
	}
	firstRequests := requests
	if _, err := Sync(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if requests != firstRequests {
		t.Fatalf("idempotent sync made %d more requests", requests-firstRequests)
	}
}

func TestSyncDoesNotPublishPartialVersion(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "champion-summary.json") {
			fmt.Fprint(w, `[{"id":1,"name":"Annie"}]`)
			return
		}
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()
	root := t.TempDir()
	_, err := Sync(context.Background(), Options{Root: root, Version: "16.14", Locales: []string{"en_us"}, BaseURL: server.URL})
	if err == nil {
		t.Fatal("expected error")
	}
	if _, statErr := os.Stat(filepath.Join(root, "16.14")); !os.IsNotExist(statErr) {
		t.Fatalf("partial version was published: %v", statErr)
	}
}
