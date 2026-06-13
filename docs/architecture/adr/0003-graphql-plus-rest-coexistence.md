# ADR-0003: GraphQL as the primary API, REST for the operational surface

- Status: Accepted
- Date: 2026-06-12
- Deciders: GOGG maintainers

## Context

The pre-refactor API is REST: `GET /api/rankings/champions`,
`GET /api/versions`, `GET /api/regions`. The refactor introduces
three new feature surfaces — champion detail, summoner search,
user system — and each has wildly different query shapes. The
champion detail page wants ~12 related slices in one round trip
(stats, runes, items, matchups, builds). The summoner page is
hierarchical (player → matches → participants → bans). The
rankings page is flat and filtered.

This is the textbook GraphQL fit: client-driven shape selection,
strong typing, schema as a contract.

But REST does some things GraphQL is bad at:

- Health checks (`/healthz`, `/readyz`)
- Prometheus metrics (`/metrics`)
- OAuth redirect callbacks (browser GET to a fixed URL)
- File / blob endpoints
- Cache-friendly resource-style URLs that fit naturally into
  CDN rules
- A graceful path for the legacy frontend during the transition

## Decision

**GraphQL is the primary public API; REST is reserved for the
operational surface and the legacy compatibility layer.**

```
/graphql                 — single endpoint for client queries (gqlgen)
/healthz, /readyz        — k8s probes
/metrics                 — Prometheus scrape
/oauth/callback/{provider} — Discord / Google / RSO redirect target
/auth/refresh, /auth/logout — short-lived REST for token rotation
/api/v1/rankings/champions — legacy compat (drop in Phase D)
/api/v1/versions           — legacy compat (drop in Phase D)
/api/v1/regions            — legacy compat (drop in Phase D)
```

## Rationale

- **Right tool per shape.** GraphQL solves the n+1 fan-out
  problem for the champion-detail and summoner pages, where a
  REST design would either over-fetch or require multiple round
  trips.
- **Schema as documentation.** GraphQL introspection, plus
  `graphql-codegen` on the frontend, means the React app gets
  typed query hooks generated from the server schema. No
  parallel REST OpenAPI to maintain.
- **REST is fine for non-query surfaces.** Health probes, OAuth
  redirects, and metrics scraping all have well-known REST
  shapes. Forcing them through GraphQL would be a worse fit
  (POST-only, JSON envelope, no native streaming).
- **Legacy compat without rewriting twice.** The existing web
  page hits `/api/rankings/champions`. Keeping that REST route
  alive (calling the same `service.RankingsService` underneath)
  lets the legacy frontend stay in service while the new
  frontend is built. Drop the REST route in Phase D when the
  new web app cuts over.

## Consequences

### Positive

- One contract, two transports (GraphQL + REST compat), one
  service layer — no logic duplication.
- Frontend types stay in sync with the server because they're
  generated from the same SDL.
- OAuth and ops endpoints don't pay GraphQL's complexity tax.

### Negative

- Two transports to test. Mitigation: integration tests target
  the service layer directly; transport-level tests are thin
  smoke tests that confirm wiring.
- GraphQL caching is hard at the HTTP layer (POST + body). For
  V1 we cache server-side (Redis, behind the resolver) instead
  of trying to teach the CDN about GraphQL operations.
  Persisted queries become an option in Phase F if hit rates
  matter.
- Two error envelopes (GraphQL `errors[]` vs REST status codes).
  Mitigation: both flow from the same typed error in
  `packages/domain/errors.go`; the transport maps it.

## Alternatives considered

### REST everywhere (status quo + new endpoints)
Rejected. Champion detail and summoner pages would need
either over-fetching, multiple parallel calls, or a custom
batching layer that ends up being a worse GraphQL.

### GraphQL everywhere (including health/metrics)
Rejected. K8s probes expect HTTP 200 on a fixed path. Prometheus
expects `/metrics` with a fixed wire format. OAuth providers
redirect to a fixed URL. Forcing these through `/graphql` adds
mapping logic for no benefit.

### tRPC / gRPC-Web
Rejected. tRPC ties the frontend to TypeScript backends; we are
on Go. gRPC-Web works but adds proto plumbing for the public
edge that gqlgen handles natively with SDL.
