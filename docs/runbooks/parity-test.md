# Parity test: legacy `/api/*` vs new `/api/v1/*`

During Phase B the legacy `internal/server/` HTTP API and the new
`apps/api` GraphQL+REST stack run **side by side**. Every endpoint the
new stack adds must return JSON that is **byte-equal** to the legacy
shape so the existing frontend keeps working without changes.

This runbook covers the manual parity smoke. CI's
`migrations parity (legacy ↔ packages/sqlc)` job covers SQL-file
drift; this runbook covers HTTP response drift.

## Prerequisites

```bash
make dev                          # postgres + redis + temporal must be up
make migrate-up                   # schema at v12, dirty=f
```

The DB can be empty (parity still holds, both sides return `[]`).
For a richer comparison, run a crawler against the dev DB beforehand:

```bash
# inserts real data; takes minutes to hours depending on tier
go run . crawl run --profile daily_kr
```

## Run both binaries

```bash
# Terminal 1: legacy on the default port
go build -o /tmp/gogg-legacy .
PORT=8080 /tmp/gogg-legacy serve

# Terminal 2: new on an alternate port (so they don't clash)
go build -o /tmp/gogg-api ./apps/api/cmd/api
GOGG_API_PORT=8081 /tmp/gogg-api
```

## Diff each endpoint

```bash
diff <(curl -s http://localhost:8080/api/versions) \
     <(curl -s http://localhost:8081/api/v1/versions) \
  && echo "VERSIONS PARITY OK"

diff <(curl -s http://localhost:8080/api/regions) \
     <(curl -s http://localhost:8081/api/v1/regions) \
  && echo "REGIONS PARITY OK"
```

Both diffs must be **empty**. A non-empty diff is a regression — open
an issue and roll back the offending commit on `refactor/phase-b-backend`
until it is reconciled.

## Endpoint coverage tracker

| Legacy | New | Parity verified |
|---|---|---|
| `GET /api/versions` | `GET /api/v1/versions` | ✅ Phase B chunk 2 |
| `GET /api/regions` | `GET /api/v1/regions` | ✅ Phase B chunk 2 |
| `GET /api/rankings/champions` | `GET /api/v1/rankings/champions` | ⏳ Phase B chunk 3 (rankings) |
| `GET /healthz` | `GET /healthz` | ✅ Phase B chunk 1 |
| `GET /readyz` | `GET /readyz` | ✅ Phase B chunk 1 (extends to add DB ping) |

Update this table in the same PR that adds each new endpoint.

## When parity legitimately diverges

Two cases are allowed to diverge:

1. **Error envelopes** — legacy returns `err.Error()` verbatim, new
   stack returns a sanitized message. New is more secure; the legacy
   behaviour was flagged in ADR-0003. We do NOT mirror that.
2. **Headers** — new stack adds `X-Request-Id` and may add others
   over time. Legacy did not. The body parity is the contract; the
   header set is allowed to grow.
