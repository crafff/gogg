package rest

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/crafff/gogg/packages/cdragonassets"
)

var profileIconPath = regexp.MustCompile(`^/game-assets/profile-icons/([1-9][0-9]*)\.jpg$`)

// GameAssetsHandler serves the atomically-published CommunityDragon tree.
func GameAssetsHandler(root string) http.Handler {
	return gameAssetsHandler(root, cdragonassets.DefaultBaseURL, &http.Client{Timeout: 30 * time.Second})
}

func gameAssetsHandler(root, communityDragonBaseURL string, client *http.Client) http.Handler {
	files := http.StripPrefix("/game-assets/", http.FileServer(http.Dir(root)))
	var downloads singleflight.Group
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if match := profileIconPath.FindStringSubmatch(r.URL.Path); match != nil {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
			id, _ := strconv.Atoi(match[1])
			if _, err, _ := downloads.Do(match[1], func() (any, error) {
				return cdragonassets.CacheProfileIcon(r.Context(), root, id, communityDragonBaseURL, client)
			}); err != nil {
				http.Error(w, fmt.Sprintf("profile icon unavailable: %v", err), http.StatusBadGateway)
				return
			}
		}
		if r.URL.Path == "/game-assets/latest.json" || strings.HasSuffix(r.URL.Path, "/manifest.json") {
			w.Header().Set("Cache-Control", "public, max-age=60")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}
