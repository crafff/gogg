# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when
working with code in this repository.

## Project overview

**GOGG** is a League of Legends champion stats / summoner search
website implemented as a production-oriented modular monolith.

V1 scope: rankings, champion detail, summoner search, user system
(Discord/Google OAuth + Riot RSO when approved). The product targets
KR and NA1 first, with a bilingual zh-CN + en-US UI and cloud-agnostic
deployment.

The former single-binary MVP has been archived at
`/home/zrt/apps/gogg-legacy-archive-2026-06-23.tar.gz` and removed from
the project tree. Do not recreate dependencies on top-level `internal/`,
`cmd/crawl`, root `main.go`, or old `web/`.

## Repository layout

```
apps/
  api/        — gogg-api binary: chi + gqlgen GraphQL BFF + REST compat + auth
  worker/     — gogg-worker binary: Temporal worker for crawl/enrich workflows
  web/        — React 18 + Vite + TypeScript feature-grouped UI
packages/
  domain/     — shared Go enums and error codes
  sqlc/       — SQL migrations, queries, generated bindings
  riotapi/    — Riot API client
  proto/      — reserved for future gRPC contracts
config/
  *.example.yaml — tracked example configs; local *.yaml files are gitignored
deploy/
  docker/     — Dockerfiles + nginx.conf
  compose/    — local dev stack
  k8s/        — Kustomize base + overlays
  terraform/  — cloud-agnostic IaC
  observability/ — Prometheus + Grafana + Alertmanager
  secrets/    — SOPS-encrypted config files
docs/
  architecture/ — C4 + ADRs
  runbooks/     — on-call procedures
  tutorial/     — codebase walkthrough
```

## Commands

```bash
make dev          # postgres + redis + temporal + mailhog
make dev-down     # stop the dev stack
make dev-reset    # stop + drop volumes

make run-api      # uses deploy/secrets/dev.enc.yaml or config/dev.yaml
make run-worker   # same unified config source as the API
make run-web      # apps/web vite dev server

make build-api
make build-worker
make build-web
make lint
make test
make ci
make gen
make migrate-up
make migrate-new name=add_user_favorites
make check-no-legacy
```

Local plaintext config belongs in `config/dev.yaml`, copied from
`config/dev.example.yaml`. SOPS-managed config belongs in
`deploy/secrets/dev.enc.yaml`. Worker config is one unified document:
Temporal, logging, database, Riot regions, schedules, and run profiles.

## Architectural rules

1. **Service layer owns business logic.** Transports
   (`apps/api/internal/transport/graphql`,
   `apps/api/internal/transport/rest`) call into service packages. No
   SQL in resolvers, no HTTP-aware code in services.
2. **Data access goes through sqlc.** Hand-written `pgx` queries are
   allowed only for truly dynamic cases; leave a `// sqlc-skip: <reason>`
   comment when doing so.
3. **Secrets never land in git.** Use `deploy/secrets/*.enc.yaml` via
   SOPS or local ignored `config/*.yaml`.
4. **Migrations are forward-only in spirit.** Down migrations are for
   local dev; production rolls forward.
5. **No archived-code dependencies.** New code must not import top-level
   `internal/*` or `cmd/crawl`. Run `make check-no-legacy` before
   handing off backend or worker changes.

## Where to look next

- Architecture decisions: `docs/architecture/adr/000{1,2,3}-*.md`
- Developer workflow: `docs/contributing.md`
- Local secrets workflow: `deploy/secrets/README.md`
- Dev stack details: `deploy/compose/docker-compose.dev.yml`
