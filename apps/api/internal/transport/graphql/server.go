// Package graphql wires up the gqlgen executable schema into HTTP
// handlers the chi router can mount. The schema definition lives in
// schema/*.graphql; resolvers in resolver/; gqlgen-generated runtime
// in generated/.
//
// Two handlers are exposed:
//
//	NewHandler:           POST /graphql — the query endpoint
//	NewPlaygroundHandler: GET  /graphql/playground — GraphiQL UI for
//	                      development. Disabled in production via the
//	                      caller's routing logic (main.go gates on env).
package graphql

import (
	"context"
	"errors"
	"net/http"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/crafff/gogg/apps/api/internal/transport/graphql/domainerr"
	gqlgenerated "github.com/crafff/gogg/apps/api/internal/transport/graphql/generated"
	"github.com/crafff/gogg/apps/api/internal/transport/graphql/resolver"
	"github.com/crafff/gogg/apps/api/internal/transport/middleware"
)

// NewHandler returns the GraphQL HTTP handler for chi. POST + GET +
// OPTIONS transports are wired (GET is handy for cache-friendly query
// caching by clients; OPTIONS is needed by browsers for CORS preflight
// before POST). WebSocket is intentionally omitted — V1 has no
// subscriptions, and pulling it in adds gorilla/websocket as a
// transitive dep we don't otherwise need.
//
// Persisted-query and parsed-query caches are LRU(1000); query depth
// is capped at 15 to keep an adversarial query from melting the DB.
func NewHandler(r *resolver.Resolver) http.Handler {
	srv := handler.New(gqlgenerated.NewExecutableSchema(gqlgenerated.Config{Resolvers: r}))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	srv.Use(extension.Introspection{})
	srv.Use(extension.FixedComplexityLimit(300))
	srv.Use(extension.AutomaticPersistedQuery{Cache: lru.New[string](100)})

	srv.SetErrorPresenter(sanitizingErrorPresenter)
	return srv
}

// sanitizingErrorPresenter is the gqlgen ErrorPresenter. It enforces
// ADR-0003: callers see a stable, client-safe message; operators see
// the original error in the request-scoped slog. The legacy REST
// stack used to echo err.Error() back over the wire — that pattern
// leaked SQL strings (e.g. "no rows in result set"), and we don't
// reproduce it here.
//
// A *domainerr.Error is treated as already-safe and passes through.
// Anything else collapses to a generic "internal server error" plus
// the gqlgen field path.
func sanitizingErrorPresenter(ctx context.Context, err error) *gqlerror.Error {
	logger := middleware.LoggerFromContext(ctx)
	gerr := graphql.DefaultErrorPresenter(ctx, err)

	if de, ok := errors.AsType[*domainerr.Error](err); ok {
		gerr.Message = de.Public
		if gerr.Extensions == nil {
			gerr.Extensions = map[string]any{}
		}
		gerr.Extensions["code"] = de.Code
		logger.Warn("graphql_domain_error", "code", de.Code, "msg", de.Public, "err", err)
		return gerr
	}

	logger.Error("graphql_error", "err", err, "path", gerr.Path)
	gerr.Message = "internal server error"
	return gerr
}

// NewPlaygroundHandler returns the GraphiQL UI that points at /graphql.
// Mount it only in non-prod environments — the page itself is harmless
// (it's static HTML + JS) but it advertises the GraphQL endpoint and
// invites exploration we don't need on the public surface.
func NewPlaygroundHandler(graphqlPath string) http.Handler {
	return playground.Handler("GOGG GraphQL Playground", graphqlPath)
}
