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
| E · New features (champion detail, summoner, user system) | 🚧 in progress | — |
| F · Production hardening (k8s, Terraform, runbooks) | ⏳ next | — |

The previous single-binary MVP has been archived outside the project
tree at `/home/zrt/apps/gogg-legacy-archive-2026-06-23.tar.gz`. The
repository now keeps only the `apps/` + `packages/` architecture.

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
config/
  *.example.yaml tracked example configs; local *.yaml files are gitignored
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

# 3. Create local plaintext config if you are not using SOPS
cp config/dev.example.yaml config/dev.yaml
# edit config/dev.yaml and set riot.api_key before starting the worker

# 4. Run the three processes in three terminals
make run-api      # apps/api — http://localhost:8080
make run-worker   # apps/worker — Temporal worker on the crawl-{region} task queues
make run-web      # apps/web — http://localhost:5173 (proxies /api + /graphql to :8080)
```

`make run-api` and `make run-worker` decrypt `deploy/secrets/dev.enc.yaml`
via sops if the file exists; otherwise they use `config/dev.yaml`. The
vite dev server in `make run-web` proxies `/api` and `/graphql` to
`:8080`, so no CORS gymnastics in development.

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

## Learning the codebase

If you're new to the repo, work through
[`docs/tutorial/`](./docs/tutorial/README.md) in order. It's a 13-chapter
hand-held walkthrough split into three parts:

- **Part I — Understanding GOGG** (chapters 01–08): from "what is this?"
  to "I can trace a single `winRate` value from Riot's API into a row in
  the browser." Assumes no Go / React / GraphQL / Temporal background.
- **Part II — Transferable knowledge** (chapters 10–13): Go essentials
  + React/TypeScript essentials + a meta-skill chapter on reading any
  unfamiliar codebase + six annotated line-by-line code tours.
- **Part III — Going further** (chapter 09): Phase E + F roadmap, ADR
  pointers, contribution workflow.

## Documentation

- [`CLAUDE.md`](./CLAUDE.md) — load-bearing project context (read first)
- [`docs/tutorial/`](./docs/tutorial/README.md) — 9-chapter hand-held codebase walkthrough
- [`docs/manual-verification.md`](./docs/manual-verification.md) — step-by-step manual smoke + test guide
- [`docs/contributing.md`](./docs/contributing.md) — developer workflow + PR checklist
- [`docs/architecture/adr/`](./docs/architecture/adr/) — architectural decision records
- [`docs/runbooks/`](./docs/runbooks/) — on-call procedures
- [`deploy/secrets/README.md`](./deploy/secrets/README.md) — SOPS + age workflow
- [`deploy/compose/docker-compose.dev.yml`](./deploy/compose/docker-compose.dev.yml) — local dev stack details

## License

UNLICENSED — private project. Do not redistribute.
