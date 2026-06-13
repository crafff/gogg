# ADR-0001: Modular monolith over microservices

- Status: Accepted
- Date: 2026-06-12
- Deciders: GOGG maintainers
- Supersedes: —
- Superseded by: —

## Context

GOGG is being rewritten from a hobby MVP into a production
service. The pre-refactor codebase is already a single Go
binary (`./gogg`) with a `serve` subcommand and a `crawl`
subcommand. V1 has two regions (KR + NA1), four pages
(rankings, champion detail, summoner, user profile), no
payments, and no SLA commitments beyond best-effort.

We considered three shapes:

1. **Microservices.** A `rankings-service`, `champion-service`,
   `summoner-service`, `auth-service`, `crawler-service`, each
   independently deployable, talking over gRPC/HTTP.
2. **Modular monolith + worker.** One API binary, one async
   worker binary. Internal modules talk through Go interfaces;
   the only out-of-process boundary is the Temporal queue
   between API and worker.
3. **Strict monolith.** One binary that also runs the crawler
   as in-process goroutines (the current state).

## Decision

**Adopt option 2: a modular monolith (`gogg-api`) plus a
Temporal worker (`gogg-worker`).** The two binaries share the
same Go workspace, the same domain types (`packages/domain`),
the same data layer (`packages/sqlc`), and the same Riot client
(`packages/riotapi`). The only runtime boundary is the Temporal
task queue.

## Rationale

- **Match the scale.** At V1 (two regions, no payments, single
  team), the cost of microservices — service mesh, distributed
  tracing setup, per-service CI, cross-service schema
  versioning, distributed transactions — exceeds their benefit.
  Stripe, Shopify, GitHub, and Basecamp all ran modular
  monoliths well past the GOGG scale we're targeting.
- **Keep the crawler async-able.** The crawler is the one
  workload with genuinely different scaling characteristics
  (long-running, rate-limited by Riot, region-partitioned). A
  separate worker binary lets us scale it per region without
  contaminating the API's resource budget.
- **Preserve optionality.** Internal modules use interfaces
  (`service.RankingsService`, `service.UserService`). The day
  any single module needs independent scaling or deploy
  cadence, we promote it to its own binary by moving its `cmd/`
  and wiring a gRPC transport. The `packages/proto/` directory
  is reserved for that future.
- **Operability wins.** One binary to monitor for the synchronous
  path, one for async. One set of deploy pipelines. One health
  check surface. Fewer pager rotations.

## Consequences

### Positive

- Smaller surface area: ~2 deployable artifacts vs ~6 if we
  fanned out.
- Lower observability bar: per-process traces, no need for
  baggage propagation across many hops.
- Refactors that span modules are atomic commits, not coordinated
  releases.
- Easier local development: `make dev` brings up the whole
  stack with three Docker services (pg, redis, temporal) +
  two Go processes.

### Negative

- Module boundaries are enforced by code review, not by network.
  A careless commit can reach across modules in ways
  microservices would have prevented. Mitigation: ADR-driven
  package ownership, `internal/` boundaries per app, linter
  rules forbidding cross-feature imports.
- Single deploy unit means a regression in one feature can
  delay deploys for another. Mitigation: feature flags
  (introduced in Phase F if needed; deliberately not in V1).
- The Temporal dependency adds operational surface (Temporal
  server, its own PostgreSQL, its UI). Mitigation: vendored in
  docker-compose for dev, self-hosted for prod; we don't pay
  for Temporal Cloud at V1.

## Alternatives considered

### Microservices fanout
Rejected. The synchronous API logic is ~3,000 LOC of Go and a
handful of CTE-heavy SQL queries. Splitting it across processes
buys nothing and costs the entire microservices tax (service
discovery, mTLS, distributed tracing, retries, idempotency,
saga patterns for cross-service writes).

### Strict monolith with in-process crawler
Rejected. The crawler's failure modes (Riot 429s, multi-hour
runs, per-region rate limits) want a workflow engine with
observability and resumability. Cohabiting it with the
request-serving process means a stuck crawl can starve API
requests of file descriptors or DB connections.

### Postgres-as-queue instead of Temporal
Considered. Cheaper to operate (one less service). Rejected
because Temporal's primitives (signals, child workflows,
cron schedules, history replay) match the 8-phase crawl
shape directly; reimplementing a quarter of it on top of
`SELECT ... FOR UPDATE SKIP LOCKED` would burn weeks and never
give us the UI for free.
