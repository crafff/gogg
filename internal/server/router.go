package server

import (
	"encoding/json"
	// "fmt"
	"net/http"
	"os"
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
		// fmt.Printf("Query params - queueId: %d, position: '%s', minGames: %d, limit: %d\n", queueID, position, minGames, limit)

		if version == "latest" {
			latestVersion, err := app.repos.versionStore.GetLatestVersion()
			if err != nil {
				http.Error(w, "failed to get latest version: "+err.Error(), http.StatusInternalServerError)
				return
			}
			version = latestVersion
		}

		query := ChampionRankingQuery{
			QueueID:  queueID,
			GameVersion: version,
			MinGames: minGames,
			Limit:    limit,
		}

		var items []ChampionRankingItem
		var err error

		if position != "" {
			query.Position = position
			items, err = app.repos.rankingStore.GetRankingsByPosition(r.Context(), query)
		} else {
			query.PositionThreshold = positionThreshold
			items, err = app.repos.rankingStore.GetOverallRankings(r.Context(), query)
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
				"queueId":     queueID,
				"gameVersion": version, // 返回查询所用的版本号，方便前端展示
				"position":    position,      // 如果未指定则为空字符串
				"minGames":    minGames,
				"limit":       limit,
				"positionThreshold": positionThreshold,
			},
		})
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

	fs := http.FileServer(http.Dir(distDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			http.NotFound(w, r)
			return
		}

		relPath := strings.TrimPrefix(filepath.Clean(r.URL.Path), string(filepath.Separator))
		target := filepath.Join(distDir, relPath)
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
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
