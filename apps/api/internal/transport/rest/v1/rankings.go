package v1

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/crafff/gogg/apps/api/internal/service/rankings"
)

// RankingsService is the narrow surface the rankings handler needs.
// Defined here for the same reason CatalogService is — consumer-side
// interfaces decouple v1 from the concrete service package and let
// tests mock without a real DB.
type RankingsService interface {
	GetOverall(ctx context.Context, f rankings.Filter) (rankings.Result, error)
	GetByPosition(ctx context.Context, f rankings.Filter) (rankings.Result, error)
}

// rankingsHandler returns GET /api/v1/rankings/champions.
//
// Defaults and clamps mirror legacy intQuery / floatQuery exactly so
// the response is byte-equal for any URL the old frontend issues:
//
//	limit              default -1   range [-1, 500]
//	minGames           default 20   range [1, 20000]
//	queueId            default 420  range [0, 9999]
//	position           "" or one of TOP|JUNGLE|MIDDLE|BOTTOM|UTILITY (uppercased)
//	positionThreshold  default 5.0  range [0, 100]
//	version            "" or "latest" or an explicit "X.Y.Z"
//	region             "" or uppercased region code (e.g. KR, NA1)
//	tier               "" or lowercased tier group (e.g. master_plus)
func rankingsHandler(s RankingsService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		filter := rankings.Filter{
			QueueID:           clampInt(intQuery(q, "queueId", 420), 0, 9999),
			Version:           strings.TrimSpace(q.Get("version")),
			Region:            strings.ToUpper(strings.TrimSpace(q.Get("region"))),
			Position:          strings.ToUpper(strings.TrimSpace(q.Get("position"))),
			TierGroup:         strings.ToLower(strings.TrimSpace(q.Get("tier"))),
			MinGames:          clampInt(intQuery(q, "minGames", 20), 1, 20000),
			Limit:             clampInt(intQuery(q, "limit", -1), -1, 500),
			PositionThreshold: clampFloat(floatQuery(q, "positionThreshold", 5.0), 0, 100),
		}

		var (
			res rankings.Result
			err error
		)
		if filter.Position != "" {
			res, err = s.GetByPosition(r.Context(), filter)
		} else {
			res, err = s.GetOverall(r.Context(), filter)
		}
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "failed to get champion rankings")
			return
		}

		items := res.Items
		if items == nil {
			items = []rankings.ChampionRanking{}
		}

		// Echo back the version that actually drove the query so the
		// caller knows what "latest" resolved to. Empty string means
		// the caller didn't ask for "latest"; in that case we send
		// back exactly what they sent us, like legacy did.
		versionEcho := filter.Version
		if res.ResolvedVersion != "" {
			versionEcho = res.ResolvedVersion
		}

		respondJSON(w, http.StatusOK, map[string]any{
			"items": items,
			"meta": map[string]any{
				"queueId":           filter.QueueID,
				"version":           versionEcho,
				"region":            filter.Region,
				"tier":              filter.TierGroup,
				"position":          filter.Position,
				"minGames":          filter.MinGames,
				"limit":             filter.Limit,
				"positionThreshold": filter.PositionThreshold,
				"totalMatches":      res.TotalMatches,
			},
		})
	}
}

func intQuery(q map[string][]string, key string, def int) int {
	raw := strings.TrimSpace(get(q, key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

func floatQuery(q map[string][]string, key string, def float64) float64 {
	raw := strings.TrimSpace(get(q, key))
	if raw == "" {
		return def
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return def
	}
	return v
}

func get(q map[string][]string, key string) string {
	if vs := q[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
