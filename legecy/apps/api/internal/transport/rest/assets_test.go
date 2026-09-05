package rest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGameAssetsHandler(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "16.14")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"version":"16.14"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/game-assets/16.14/manifest.json", nil)
	rec := httptest.NewRecorder()
	GameAssetsHandler(root).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60" {
		t.Fatalf("cache-control = %q", got)
	}
}

func TestGameAssetsHandlerCachesProfileIcon(t *testing.T) {
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/latest/plugins/rcp-be-lol-game-data/global/default/v1/profile-icons/6205.jpg" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		fmt.Fprint(w, "jpeg")
	}))
	defer upstream.Close()

	handler := gameAssetsHandler(t.TempDir(), upstream.URL, upstream.Client())
	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/game-assets/profile-icons/6205.jpg", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "jpeg" {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	}
	if requests != 1 {
		t.Fatalf("upstream requests=%d, want 1", requests)
	}
}

func TestGameAssetsHandlerRejectsProfileIconWrites(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/game-assets/profile-icons/6205.jpg", nil)
	rec := httptest.NewRecorder()
	GameAssetsHandler(t.TempDir()).ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", rec.Code)
	}
}
