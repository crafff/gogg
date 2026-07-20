package rest

import (
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
