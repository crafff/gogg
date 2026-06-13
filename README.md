# GOGG

League of Legends champion stats and summoner search website.
**This repo is mid-refactor** from a hobby MVP into a production
service; see the [refactoring plan](/home/zrt/.claude/plans/radiant-wobbling-pizza.md)
and the [architecture decisions](./docs/architecture/adr/) for
context.

## What's in here

- `apps/api/` — `gogg-api` GraphQL BFF + REST compat (Go,
  populated in Phase B)
- `apps/worker/` — `gogg-worker` Temporal worker hosting the
  crawler workflows (Phase C)
- `apps/web/` — React 18 + Vite + TS frontend (Phase D)
- `packages/sqlc/`, `packages/domain/`, `packages/riotapi/` —
  shared Go packages
- `deploy/` — Docker, Compose, Kubernetes, Terraform,
  observability, SOPS secrets
- `docs/` — ADRs, runbooks, API docs, contributor guide
- `internal/`, `cmd/`, top-level `main.go`, `web/` — the
  **legacy stack**, kept running until the rewrite catches up;
  do not delete

## Quick start (legacy stack — still the path that actually serves traffic)

```bash
# 1. Start postgres
docker compose -f docker-compose.yml up -d

# 2. Build and run the API + crawler binary
go build -o gogg .
./gogg serve                       # HTTP API on :8080
./gogg crawl run --profile daily_kr # crawler

# 3. Run the frontend
cd web
npm install
npm run dev   # http://localhost:5173, proxies /api to :8080
```

## Quick start (refactored stack — bring up the new monorepo dev env)

```bash
make dev          # postgres + redis + temporal + mailhog
make migrate-up   # apply DB migrations
# apps/api and apps/worker are populated in Phase B/C; Phase A
# only sets up the scaffolding.
```

See [`docs/contributing.md`](./docs/contributing.md) for the
full developer workflow, commit conventions, and PR checklist.

## Top-level make targets

```
make dev / dev-down / dev-reset   — local stack lifecycle
make lint / test / ci             — quality gates
make gen / gen-sqlc / gen-gql     — code generation
make migrate-up / migrate-new     — database migrations
make build / build-api / build-worker — Go binaries
make hooks                        — install pre-commit hooks
```

## License

UNLICENSED — private project. Do not redistribute.
