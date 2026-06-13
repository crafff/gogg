// Package v1 hosts the REST compatibility layer the legacy frontend
// hits at /api/v1/*. The shape of every response is byte-equal to the
// pre-refactor /api/* shape so the old web app keeps working through
// Phase D's cutover. Drop the entire package after the new frontend
// goes live (per ADR-0003).
package v1

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/crafff/gogg/apps/api/internal/transport/middleware"
)

// CatalogService is the narrow service surface the v1 handlers call.
// Defined here for the same reason catalog.Querier is defined inside
// the catalog package: consumer-defined interfaces simplify mocking
// and keep packages decoupled.
type CatalogService interface {
	ListVersionsWithData(ctx context.Context) ([]string, error)
	ListRegionsWithData(ctx context.Context) ([]string, error)
}

// Routes returns the chi sub-router mounted at /api/v1.
func Routes(catalog CatalogService, rkn RankingsService) chi.Router {
	r := chi.NewRouter()
	r.Get("/versions", versionsHandler(catalog))
	r.Get("/regions", regionsHandler(catalog))
	r.Get("/rankings/champions", rankingsHandler(rkn))
	return r
}

func versionsHandler(s CatalogService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		versions, err := s.ListVersionsWithData(r.Context())
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "failed to load versions")
			return
		}
		// Defense in depth: service already coalesces nil → [], but
		// the handler re-checks so parity holds even if a future
		// alternative implementation forgets.
		if versions == nil {
			versions = []string{}
		}
		respondJSON(w, http.StatusOK, map[string]any{"versions": versions})
	}
}

func regionsHandler(s CatalogService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		regions, err := s.ListRegionsWithData(r.Context())
		if err != nil {
			respondError(w, r, http.StatusInternalServerError, "failed to load regions")
			return
		}
		if regions == nil {
			regions = []string{}
		}
		respondJSON(w, http.StatusOK, map[string]any{"regions": regions})
	}
}

// respondError logs the underlying detail via the request-scoped logger
// (so operators see it) and returns a generic message to the client
// (so we don't leak internals). Replaces the legacy pattern of
// returning err.Error() in the HTTP body verbatim.
func respondError(w http.ResponseWriter, r *http.Request, code int, msg string) {
	middleware.LoggerFromContext(r.Context()).Error("rest_error", "code", code, "msg", msg)
	respondJSON(w, code, map[string]string{"error": msg})
}

func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
