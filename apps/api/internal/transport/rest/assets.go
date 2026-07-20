package rest

import (
	"net/http"
	"strings"
)

// GameAssetsHandler serves the atomically-published CommunityDragon tree.
func GameAssetsHandler(root string) http.Handler {
	files := http.StripPrefix("/game-assets/", http.FileServer(http.Dir(root)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/game-assets/latest.json" || strings.HasSuffix(r.URL.Path, "/manifest.json") {
			w.Header().Set("Cache-Control", "public, max-age=60")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}
