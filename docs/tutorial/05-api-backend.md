# Chapter 05 · API backend

> Goal: by the end of this chapter you can trace any HTTP request through chi → middleware → service → sqlc → Postgres → response; you understand the GraphQL surface and how it shares the same service layer as REST; you've added a console log to a handler and watched it fire.

This chapter assumes Chapters 02 and 03 — you have the dev stack up, the API binary boots, and you understand the database. We're now inside the `apps/api/` tree.

## The 3-layer architecture

```
                  ┌────────────────────────────┐
                  │   transport/               │  ← knows about HTTP
                  │   - rest/      (chi router)│
                  │   - graphql/   (gqlgen)    │
                  │   - middleware/            │
                  └─────────────┬──────────────┘
                                │ calls
                                ▼
                  ┌────────────────────────────┐
                  │   service/                 │  ← business logic
                  │   - catalog/               │
                  │   - rankings/              │
                  │   - user/                  │
                  │   - champion/, summoner/  (Phase E)
                  └─────────────┬──────────────┘
                                │ calls
                                ▼
                  ┌────────────────────────────┐
                  │   packages/sqlc/gen/       │  ← knows about SQL
                  │   - typed query functions  │
                  └────────────────────────────┘
```

The cardinal rule:

- **Transport** never calls SQL. It validates input, calls a service, formats output.
- **Service** never imports `net/http`. It owns business decisions.
- **sqlc** is the only SQL boundary.

Why bother? Because the same service can be invoked from REST *and* GraphQL — they're two different transports over the same logic. Phase B chunk 5 proved this when `championRankings` GraphQL and `/api/v1/rankings/champions` REST started returning the same numbers without code duplication.

## Walk through one request, top to bottom

Pick the simplest endpoint: `GET /api/v1/versions`.

### 5a · Entry point

```bash
cat apps/api/cmd/api/main.go | head -120
```

Two important functions:

- `run()` builds dependencies (config, logger, pgxpool, Redis), constructs the router via `buildRouter()`, starts the HTTP server, waits on SIGINT.
- `buildRouter(cfg, logger, pool, redisClient)` constructs the chi router with all the routes and middleware. Read it slowly:

```go
r := chi.NewRouter()
r.Use(middleware.Recoverer)
r.Use(middleware.RequestID)
r.Use(middleware.Logger(logger))
r.Use(metrics.Middleware)
r.Use(corsMiddleware)
// ...
r.Get("/healthz", rest.LivenessHandler())
r.Get("/readyz", rest.ReadinessHandler(pingers...))
r.Mount("/api/v1", v1.Routes(catalogSvc, rankingsSvc))
r.Handle("/graphql", gqlserver.NewHandler(gqlRoot))
r.Mount("/", restauth.Routes(userService, authCfg))
```

`r.Use(...)` registers middleware (runs on every request). `r.Get / r.Mount / r.Handle` register routes.

### 5b · Middleware chain

Middleware wraps a handler with cross-cutting behavior. In order:

1. `Recoverer` — turns a panic into a 500 instead of crashing the process.
2. `RequestID` — generates / propagates `X-Request-ID` for log correlation.
3. `Logger` — emits a structured log line per request (path, status, latency).
4. `metrics.Middleware` — increments Prometheus counters, observes latency histograms.
5. `corsMiddleware` — same-origin policy.

The handler runs after every `Use()`-registered piece has wrapped it.

🛠️ **Exercise**: hit `curl -s http://localhost:8080/healthz -i` and look at the `X-Request-Id` response header. The same ID should appear in the API's stdout log lines for that request.

### 5c · Hit the mount

`r.Mount("/api/v1", v1.Routes(catalogSvc, rankingsSvc))` says: anything matching `/api/v1/*` is delegated to whatever `v1.Routes(...)` returns.

```bash
cat apps/api/internal/transport/rest/v1/v1.go
```

You'll see:

```go
func Routes(catalog CatalogService, rkn RankingsService) chi.Router {
    r := chi.NewRouter()
    r.Get("/versions", versionsHandler(catalog))
    r.Get("/regions", regionsHandler(catalog))
    r.Get("/rankings/champions", rankingsHandler(rkn))
    return r
}
```

Note: this function takes **interfaces** (`CatalogService`, `RankingsService`), not concrete types. That's so tests can swap them for mocks. Look how `CatalogService` is defined in the same file — it's an interface declared at the consumer side ("accept interfaces, return concretes" — a Go idiom).

### 5d · The handler

```bash
grep -A 25 'func versionsHandler' apps/api/internal/transport/rest/v1/v1.go
```

It's roughly:

```go
func versionsHandler(svc CatalogService) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        versions, err := svc.ListVersionsWithData(r.Context())
        if err != nil {
            writeError(w, err)
            return
        }
        writeJSON(w, http.StatusOK, versions)
    }
}
```

Three lines of real work:

1. Call the service (passes the request context for cancellation).
2. If err, map to HTTP status + sanitized error body.
3. Else, serialize as JSON.

Notice what's *missing*: no SQL. No `pgx`. No business logic. Transports are the thin glue.

### 5e · The service

```bash
cat apps/api/internal/service/catalog/catalog.go
```

The service is a struct holding the sqlc queries object + the cache, and methods like `ListVersionsWithData(ctx)`. Inside:

```go
func (s *Service) ListVersionsWithData(ctx context.Context) ([]string, error) {
    return s.cache.GetOrLoad(ctx, "catalog:versions", func() ([]string, error) {
        return s.queries.ListVersionsWithData(ctx)
    })
}
```

The cache wrapper (`cache.GetOrLoad`) is the Redis layer. First call: hits Postgres, stores in Redis with a TTL. Subsequent calls within TTL: served from Redis in <10 ms.

### 5f · The query

We covered this in Chapter 03. The Go function that lives in `packages/sqlc/gen/versions.sql.go` runs the `SELECT version FROM matches ...` statement and scans the rows.

## What the middleware actually wraps

Let me pull this together with an exercise.

🛠️ **Exercise**: add a temporary `slog.Info` call inside the catalog service.

```bash
# Find the function
grep -n 'func.*ListVersionsWithData' apps/api/internal/service/catalog/*.go
```

Edit `apps/api/internal/service/catalog/catalog.go` and add a log line:

```go
func (s *Service) ListVersionsWithData(ctx context.Context) ([]string, error) {
    s.logger.Info("listing_versions")    // <-- add this
    return s.cache.GetOrLoad(...)
}
```

(You'll need to add a `logger *slog.Logger` field to the struct and accept it in `New(...)` — search for `New(` in the same file. This is a 1-line change in 2 places.)

Save, then restart `make run-api`. Hit `curl http://localhost:8080/api/v1/versions` and you should see your log line in the API output. Revert when done.

This is the "I can change something in this codebase" milestone. Once you can do that, every other exercise feels small.

## REST vs GraphQL

The same `Service.ListVersionsWithData(...)` is called from both transports. To see the GraphQL side:

```bash
cat apps/api/internal/transport/graphql/schema/catalog.graphql
```

```graphql
extend type Query {
  versions: [String!]!
  regions: [String!]!
}
```

That schema is the public contract. Now the resolver:

```bash
cat apps/api/internal/transport/graphql/resolver/catalog.resolvers.go
```

```go
func (r *queryResolver) Versions(ctx context.Context) ([]string, error) {
    return r.catalog.ListVersionsWithData(ctx)
}
```

Same one-liner as the REST handler — both delegate to the same service. No duplication.

ADR-0003 explains why we keep both: GraphQL is the primary frontend API,
while REST stays as a compatibility layer for scripts, smoke checks, and
clients that do not need a GraphQL client.

🛠️ **Exercise**: open <http://localhost:8080/graphql/playground> and run:

```graphql
query { versions  regions }
```

Watch the API logs — you should see the same log line you added in the previous exercise. **One service call, two transport ways in.**

## The harder query: rankings

`/api/v1/rankings/champions` accepts ~8 query parameters. The handler validates them, packs them into a `ListChampionsFilter` struct, and calls `rankings.Service.ListChampions(ctx, filter)`. The service:

1. Computes a cache key from the filter (`"rankings:hash(filter)"`).
2. `GetOrLoad` against Redis. On miss:
3. Calls the right sqlc query — `ListOverallRankings` if `filter.Position == ""`, else `ListRankingsByPosition`.
4. Post-processes the result (typically empty for now, may attach metadata in future).

The GraphQL resolver does roughly the same wrapping. The filter is a struct on both sides; you'll see the field-by-field translation in `transport/graphql/resolver/rankings.resolvers.go`.

🛠️ **Exercise**: run

```graphql
query {
  championRankings(filter: { region: "KR", queueId: 420, limit: 5 }) {
    items { championName winRate games }
    totalMatches
  }
}
```

Look at `apps/api/internal/transport/graphql/resolver/rankings.resolvers.go`, then `apps/api/internal/service/rankings/service.go`, then `packages/sqlc/gen/rankings.sql.go`. You've now walked the full GraphQL → service → SQL trace.

## Errors and how they sanitize

Look at `apps/api/internal/transport/graphql/error_presenter.go` (or grep `errorPresenter`):

The function sees the error, decides whether it's "safe to expose" (a code like `INVALID_FILTER`) or "internal" (a `pgx` connection error). Safe errors keep their message; internal errors get redacted to `"internal error"`. ADR-0003 codified this; the legacy code didn't have this discipline and leaked SQL error text to the browser.

The same shape exists on the REST side in `writeError`.

## Auth: where it plugs in

Auth is its own subtree:

```bash
ls apps/api/internal/auth/
ls apps/api/internal/auth/provider/
```

- `auth/jwt.go` — issues + validates HS256 JWT pairs (15-min access, 30-day refresh).
- `auth/provider/{discord,google}.go` — OAuth providers.
- `auth/provider/riot_rso.go` — Riot RSO, behind a build tag.

The transport side is `apps/api/internal/transport/rest/auth/auth.go` — mounts:

```
GET  /oauth/start/{provider}    → redirect to provider
GET  /oauth/callback/{provider} → exchange code for tokens, issue JWT
POST /auth/refresh              → rotate refresh token, issue new access
POST /auth/logout               → revoke refresh token
```

We'll dive deep in Chapter 08. For now: the **Bearer middleware** is optional — it attaches a `*User` to the request context if present, doesn't reject if absent. Endpoints that *require* auth check for the user in their handler.

## Cache layer

```bash
cat apps/api/internal/cache/redis.go | head -60
```

The generic `GetOrLoad[T]` function:

1. Check Redis for the key.
2. If hit, deserialize JSON into `T`, return.
3. If miss, call the loader, marshal result to JSON, set with TTL, return.
4. If Redis is unreachable, log and bypass — never return a 500 just because cache is down.

Failure mode: a slow loader + concurrent requests would cause thundering herd. We accept that for V1; the rankings query is fast enough. A `singleflight` decorator can land later.

## Tests

```bash
go test ./apps/api/...
```

Three flavors:

- Service-layer unit tests (`apps/api/internal/service/*/service_test.go`) use a fake sqlc layer (interface-implementing struct).
- Cache tests use a real Redis client against the dev stack.
- Transport tests use `httptest` for request/response handling.

🛠️ **Exercise**: open `apps/api/internal/service/catalog/service_test.go` if it exists, or `rankings/service_test.go`. Note how the fake sqlc implementation is just a struct with the methods filled in to return canned data. This is what the "accept interfaces, return concretes" idiom enables.

## Up next

[Chapter 06 — Frontend](./06-frontend.md) goes into the React app: how vite serves it, how routes are defined, how the codegen'd `useChampionRankingsQuery` hook plugs into the GraphQL endpoint you just learned.
