package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func NewRouter(app *App) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	mux.HandleFunc("/api/rankings/champions", func(w http.ResponseWriter, r *http.Request) {
		// fmt.Printf("Received request: %s %s\n", r.Method, r.URL.String())
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		limit := intQuery(r, "limit", -1, -1, 500)
		minGames := intQuery(r, "minGames", 20, 1, 20000)
		queueID := intQuery(r, "queueId", 420, 0, 9999)
		position := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("position")))
		positionThreshold := floatQuery(r, "positionThreshold", 5.0, 0, 100)
		version := strings.TrimSpace(r.URL.Query().Get("version"))
		region := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("region")))
		tierGroup := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("tier")))

		if version == "latest" {
			latestVersion, err := app.repos.versionStore.GetLatestVersion()
			if err != nil {
				http.Error(w, "failed to get latest version: "+err.Error(), http.StatusInternalServerError)
				return
			}
			version = latestVersion
		}

		query := ChampionRankingQuery{
			QueueID:   queueID,
			Version:   version,
			Region:    region,
			TierGroup: tierGroup,
			MinGames:  minGames,
			Limit:     limit,
		}

		var items []ChampionRankingItem
		var totalMatches int
		var err error

		if position != "" {
			query.Position = position
			items, totalMatches, err = app.repos.rankingStore.GetRankingsByPosition(r.Context(), query)
		} else {
			query.PositionThreshold = positionThreshold
			items, totalMatches, err = app.repos.rankingStore.GetOverallRankings(r.Context(), query)
		}

		if err != nil {
			http.Error(w, "failed to get champion rankings: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if items == nil {
			items = []ChampionRankingItem{}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"items": items,
			"meta": map[string]any{
				"queueId":           queueID,
				"version":           version,
				"region":            region,
				"tier":              tierGroup,
				"position":          position,
				"minGames":          minGames,
				"limit":             limit,
				"positionThreshold": positionThreshold,
				"totalMatches":      totalMatches,
			},
		})
	})

	mux.HandleFunc("/api/versions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		versions, err := app.repos.versionStore.GetVersionsWithData(r.Context())
		if err != nil {
			http.Error(w, "failed to get versions: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if versions == nil {
			versions = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
	})

	mux.HandleFunc("/api/regions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		regions, err := app.repos.rankingStore.GetRegionsWithData(r.Context())
		if err != nil {
			http.Error(w, "failed to get regions: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if regions == nil {
			regions = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"regions": regions})
	})

	if distHandler := buildDistHandler(app.webDistDir); distHandler != nil {
		mux.Handle("/", distHandler)
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}

			writeJSON(w, http.StatusOK, map[string]string{
				"service": "gogg-server",
				"status":  "running",
				"message": "frontend is not built yet (run: cd web && npm install && npm run build)",
			})
		})
	}

	return withCORS(mux)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func intQuery(r *http.Request, key string, fallback, min, max int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}

	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func floatQuery(r *http.Request, key string, fallback, min, max float64) float64 {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}

	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// 跨域访问（Cross-Origin Resource Sharing, CORS）支持
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func buildDistHandler(distDir string) http.Handler {
	indexPath := filepath.Join(distDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return nil
	}

	absDist, err := filepath.Abs(distDir)
	if err != nil {
		return nil
	}

	fs := http.FileServer(http.Dir(distDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			http.NotFound(w, r)
			return
		}

		// Clean using path (URL paths always use '/') then convert
		// to the OS-specific separator before joining with distDir.
		// path.Clean strips ../ components so the URL "/../etc/passwd"
		// becomes "/etc/passwd", but as belt-and-braces we also
		// verify the joined path doesn't escape the served dir.
		cleaned := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
		target := filepath.Join(distDir, filepath.FromSlash(cleaned))
		absTarget, err := filepath.Abs(target)
		if err != nil || !strings.HasPrefix(absTarget+string(filepath.Separator), absDist+string(filepath.Separator)) {
			http.NotFound(w, r)
			return
		}
		if info, err := os.Stat(absTarget); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		if err := serveIndex(w, r, indexPath); err != nil {
			http.Error(w, "frontend not available", http.StatusInternalServerError)
		}
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, indexPath string) error {
	h, err := os.Open(indexPath)
	if err != nil {
		return err
	}
	defer h.Close()

	stat, err := h.Stat()
	if err != nil {
		return err
	}

	http.ServeContent(w, r, "index.html", stat.ModTime(), h)
	return nil
}
