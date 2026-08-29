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
		case strings.HasSuffix(r.URL.Path, "/default/v1/items.json"), strings.HasSuffix(r.URL.Path, "/zh_cn/v1/items.json"):
			fmt.Fprint(w, `[{"id":1001,"name":"Boots","iconPath":"/lol-game-data/assets/v1/items/1001.png","inStore":true},{"id":3040,"name":"Seraph's Embrace","iconPath":"/lol-game-data/assets/v1/items/3040.png","inStore":false}]`)
		case strings.HasSuffix(r.URL.Path, "/default/v1/summoner-spells.json"), strings.HasSuffix(r.URL.Path, "/zh_cn/v1/summoner-spells.json"):
			fmt.Fprint(w, `[{"id":4,"name":"Flash","iconPath":"/lol-game-data/assets/v1/summoner-spells/4.png"}]`)
		case strings.HasSuffix(r.URL.Path, "/default/v1/perks.json"), strings.HasSuffix(r.URL.Path, "/zh_cn/v1/perks.json"):
			fmt.Fprint(w, `[{"id":8005,"name":"Press","iconPath":"/lol-game-data/assets/v1/perks/8005.png"}]`)
		case strings.HasSuffix(r.URL.Path, "/default/v1/perkstyles.json"), strings.HasSuffix(r.URL.Path, "/zh_cn/v1/perkstyles.json"):
			fmt.Fprint(w, `{"styles":[{"id":8000,"name":"Precision","iconPath":"/lol-game-data/assets/v1/perk-images/Styles/7201_Precision.png"}]}`)
		case strings.HasSuffix(r.URL.Path, ".png"):
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
	if manifest.SchemaVersion != manifestSchemaVersion {
		t.Fatalf("schema version = %d, want %d", manifest.SchemaVersion, manifestSchemaVersion)
	}
	if got := manifest.Champions["1"].Names["zh_cn"]; got != "安妮" {
		t.Fatalf("zh name = %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "16.14", "champions", "1.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "16.14", "perks", "8000.png")); err != nil {
		t.Fatal(err)
	}
	if _, ok := manifest.Items["3040"]; !ok {
		t.Fatal("transformed non-store item is missing from manifest")
	}
	if _, err := os.Stat(filepath.Join(root, "16.14", "items", "3040.png")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, "16.14"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("published directory mode = %o", info.Mode().Perm())
	}
	firstRequests := requests
	if _, err := Sync(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if requests != firstRequests {
		t.Fatalf("idempotent sync made %d more requests", requests-firstRequests)
	}

	opts.Version = "16.13"
	opts.PreserveLatest = true
	if _, err := Sync(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	latest, err := readManifest(filepath.Join(root, "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if latest.Version != "16.14" {
		t.Fatalf("preserved latest version = %q", latest.Version)
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

func TestCacheProfileIconDownloadsOnce(t *testing.T) {
	t.Parallel()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/latest/plugins/rcp-be-lol-game-data/global/default/v1/profile-icons/6205.jpg" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		fmt.Fprint(w, "jpeg")
	}))
	defer server.Close()

	root := t.TempDir()
	path, err := CacheProfileIcon(context.Background(), root, 6205, server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "jpeg" {
		t.Fatalf("cached icon=%q err=%v", got, err)
	}
	if _, err := CacheProfileIcon(context.Background(), root, 6205, server.URL, server.Client()); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests=%d, want 1", requests)
	}
}
