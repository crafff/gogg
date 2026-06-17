# GOGG

League of Legends champion stats and summoner search website.
Two regions (KR + NA1), bilingual UI (zh-CN + en-US), cloud-agnostic
deploy (AWS or domestic Chinese cloud). The full refactoring plan
lives at [`docs/architecture/adr/`](./docs/architecture/adr/).

## Current state

| Phase | Status | Branch |
|---|---|---|
| A · Foundation (monorepo, Docker, CI, SOPS, ADRs) | ✅ shipped | — |
| B · Backend rewrite (chi + gqlgen + sqlc + JWT/OAuth) | ✅ shipped | — |
| C · Crawler → Temporal | ✅ shipped | — |
| D · Frontend rewrite (Tailwind + TanStack + Router) | ✅ shipped | — |
| E · New features (champion detail, summoner, user system) | 🚧 in progress | `refactor/phase-e-features` |
| F · Production hardening (k8s, Terraform, runbooks) | ⏳ next | — |

The legacy binary (`./gogg`) and legacy frontend (`web/`) are still
present as the rollback path; they will be removed one release cycle
after Phase E ships.

## Repository layout

```
apps/
  api/        gogg-api binary — chi + gqlgen GraphQL BFF + REST compat + auth
  worker/     gogg-worker binary — Temporal worker hosting crawl/enrich workflows
  web/        React 18 + Vite + Tailwind + TanStack Query + React Router 6
packages/
  domain/     shared Go enums (Champion, Tier, Region) + error codes
  sqlc/       SQL migrations, queries, generated bindings
  riotapi/    Riot API client (lifted from internal/riotapi)
  proto/      reserved for future gRPC contracts
deploy/
  docker/     Dockerfiles + nginx.conf
  compose/    local dev stack (docker-compose.dev.yml)
  k8s/        Kustomize base + dev/staging/prod overlays
  terraform/  cloud-agnostic IaC (modules/aws, modules/aliyun)
  observability/  Prometheus + Grafana + Alertmanager
  secrets/    SOPS-encrypted env files (age-encrypted)
docs/
  architecture/  C4 diagrams + ADRs
  runbooks/      on-call procedures
  api/           GraphQL schema docs + OpenAPI for REST compat
internal/, cmd/, main.go, web/   the legacy stack — kept until E ships
```

## Prerequisites

- **Go 1.26.4+** (toolchain pinned in `go.mod`)
- **Node 22.12+** (vite 8 peer)
- **Docker + Compose v2** (the dev stack is containerized)
- **sops + age** for decrypting `deploy/secrets/dev.enc.yaml`
- **golangci-lint v1.62+**, **sqlc v1.27+**, **golang-migrate v4.18+** (for `make gen` / `make migrate-*`)
- **lefthook** (`make hooks`) for pre-commit gates — optional locally, enforced in CI

## Quick start

```bash
# 1. Bring up postgres + redis + temporal + temporal-ui + mailhog
make dev

# 2. Apply database migrations
make migrate-up

# 3. Run the three binaries in three terminals
make run-api      # apps/api — http://localhost:8080
make run-worker   # apps/worker — Temporal worker on the crawl-{region} task queues
make run-web      # apps/web — http://localhost:5173 (proxies /api + /graphql to :8080)
```

`make run-api` and `make run-worker` decrypt `deploy/secrets/dev.enc.yaml`
via sops if the file exists; otherwise they fall back to env-only
config. The vite dev server in `make run-web` proxies `/api` and
`/graphql` to `:8080`, so no CORS gymnastics in development.

Open `http://localhost:5173` for the rankings page. Other routes
(`/champion/:id`, `/summoner/:region/:name`, `/login`, `/me`) are
placeholder pages until Phase E populates them.

## Common workflows

```bash
# Quality gates
make lint            # golangci-lint + apps/web eslint
make test            # go test ./... + apps/web vitest
make ci              # vet + lint + test (CI parity)

# Integration tests (require the dev stack running)
make test-int        # tagged `integration` Go tests

# E2E tests (require Playwright browser deps)
make test-e2e-install   # one-time: installs chromium
make test-e2e           # Playwright golden path against apps/web

# Code generation
make gen-sqlc        # regenerate packages/sqlc/gen
make gen-gql         # regenerate apps/api gqlgen resolvers
make gen-web         # regenerate apps/web/src/shared/api/generated
make gen             # all three at once

# Migrations
make migrate-up                                  # apply pending
make migrate-down                                # roll back one
make migrate-new name=add_user_favorites         # scaffold new

# Build
make build-api       # → bin/gogg-api
make build-worker    # → bin/gogg-worker
make build-web       # apps/web/dist
```

For the granular vitest / playwright / tsc workflows, run from
`apps/web/`:

```bash
cd apps/web
npm run dev          # vite dev server
npm run codegen      # graphql-codegen against apps/api schema
npm run type-check   # tsc -b --noEmit
npm run lint         # eslint
npm test             # vitest run
npm run test:watch   # vitest watch
npm run test:e2e     # playwright (chromium)
npm run build        # type-check + vite production build
```

## Verifying your setup

Once everything builds, walk through
[`docs/manual-verification.md`](./docs/manual-verification.md) — it
covers the smoke checks for each binary, the GraphQL + REST surface,
the worker's Temporal workflows, and the apps/web UI flows, plus the
test suites you should run end-to-end.

## Legacy stack (rollback path)

```bash
# Build the legacy single binary (everything in main.go + internal/server)
go build .

# Run the API
./gogg serve                          # HTTP API on :8080

# Run the crawler (cobra subcommand, marked [DEPRECATED] but functional)
./gogg crawl run --profile daily_kr

# Legacy frontend (web/) — separate Vite dev server, proxies to ./gogg serve
cd web && npm install && npm run dev
```

The legacy stack reads the unencrypted `config.yaml` (gitignored).
Don't add features here — bug fixes only, and only if they need to
ship before Phase E lands. Mirror any change into `apps/api/` in the
same PR.

## Documentation

- [`CLAUDE.md`](./CLAUDE.md) — load-bearing project context (read first)
- [`docs/manual-verification.md`](./docs/manual-verification.md) — step-by-step manual smoke + test guide
- [`docs/contributing.md`](./docs/contributing.md) — developer workflow + PR checklist
- [`docs/architecture/adr/`](./docs/architecture/adr/) — architectural decision records
- [`docs/runbooks/`](./docs/runbooks/) — on-call procedures
- [`deploy/secrets/README.md`](./deploy/secrets/README.md) — SOPS + age workflow
- [`deploy/compose/docker-compose.dev.yml`](./deploy/compose/docker-compose.dev.yml) — local dev stack details

## License

UNLICENSED — private project. Do not redistribute.
