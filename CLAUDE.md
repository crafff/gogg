# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when
working with code in this repository.

## Project overview

**GOGG** is a League of Legends champion stats / summoner search
website, mid-refactor from a hobby MVP into a production
service. The plan is captured in `/home/zrt/.claude/plans/radiant-wobbling-pizza.md` and
the load-bearing decisions live under `docs/architecture/adr/`.

V1 scope: rankings, champion detail, summoner search, user
system (Discord/Google OAuth + Riot RSO when approved).
Two regions: KR and NA1. Bilingual UI (zh-CN + en-US).
Cloud-agnostic deploy (AWS or domestic Chinese cloud).

## Repository layout

```
apps/
  api/        — gogg-api binary (chi + gqlgen GraphQL BFF + REST compat + auth)
  worker/     — gogg-worker binary (Temporal worker hosting crawl/enrich workflows)
  web/        — React 18 + Vite + TS, feature-grouped UI
packages/
  domain/     — shared Go enums (Champion, Tier, Region) and error codes
  sqlc/       — SQL migrations + queries + generated bindings
  riotapi/    — Riot API client lifted from internal/riotapi
  proto/      — reserved for future gRPC contracts
deploy/
  docker/     — Dockerfiles + nginx.conf
  compose/    — local dev stack (docker-compose.dev.yml)
  k8s/        — Kustomize base + dev/staging/prod overlays
  terraform/  — cloud-agnostic IaC (modules/aws, modules/aliyun)
  observability/ — Prometheus + Grafana + Alertmanager
  secrets/    — SOPS-encrypted env files
docs/
  architecture/ — C4 + ADRs
  runbooks/   — on-call procedures
  api/        — GraphQL schema docs + OpenAPI for the REST compat layer
```

The legacy code (`internal/server/`, `internal/crawler/`,
`internal/storage/`, top-level `main.go`, `cmd/crawl/`) is still
present and serving traffic. Phase B replaces it; Phase C
migrates the crawler to Temporal. Do not delete legacy paths
until the replacement is wired in and a release has switched
over.

## Phases

- ✅ **A · Foundation** — monorepo scaffold, Docker, CI, SOPS, ADRs
- ✅ **B · Backend rewrite** — `refactor/phase-b-backend`, ready to merge
  - ✅ chunk 1: api skeleton, config, middleware (Recover/RequestID/Logger/CORS), healthz/readyz
  - ✅ chunk 2: catalog service + `/api/v1/{versions,regions}` parity
  - ✅ chunk 3: rankings service + `/api/v1/rankings/champions` parity (10/11 byte-equal, 1 ADR-0003 divergence)
  - ✅ chunk 4: Prometheus `/metrics` + Redis cache wrapping rankings + `/readyz` includes Redis
  - ✅ chunk 5: gqlgen schema + resolvers for `versions` / `regions` / `championRankings`; sanitizing error presenter (ADR-0003); `/graphql` + `/graphql/playground`
  - ✅ chunk 6: HS256 JWT issuer + Discord/Google OAuth providers + `/oauth/start/{p}` + `/oauth/callback/{p}` + `/auth/refresh` + `/auth/logout` + optional Bearer middleware
- ✅ **C · Crawler → Temporal** — `refactor/phase-c-crawler`, ready to merge
  - ✅ chunk 1: `apps/worker` skeleton — viper config, slog→Temporal logger adapter, `PingWorkflow`/`PingActivity` on `smoke` task queue; verified end-to-end against compose Temporal (event history complete, result returned)
  - ✅ chunk 2: `apps/worker/internal/runtime` loads legacy `internal/config` + `internal/storage` + per-region `*riotapi.Client`. Activities `ResolveProfile` / `CreateRun` / `Phase0VersionSync` / `PinRunVersion` / `Phase1RankSnapshot` / `CompleteRun` / `FailRun` registered on `*crawl.Activities`. `CrawlRegionWorkflow{ProfileName | Profile}` orchestrates them with phase-specific RetryPolicy + heartbeat budgets. Verified KR end-to-end: CDragon → 226 game_versions upserted, latest 16.12 pinned on `runs.version`, Phase 1 401 from expired dev Riot key triggered Temporal's exponential retry (5s → 10s → 20s) exactly as `phase1Opts` specifies; cancellation triggered disconnected `FailRun` and `runs.status = 'failed'` stamped correctly
  - ✅ chunk 3: `Phase2MatchIDCollection` / `Phase3MatchDetails` / `Phase35OnDemandRank` / `Phase4AvgTierCalc` Activities — thin wrappers calling legacy `phase{2,3,35,4}.Phase.Run` with a synthetic `*crawler.RunState` (assembled via `ResumeRunState`). Workflow gained `runPhase2` dispatcher: `execution=sequential` runs one Activity with all `TargetTiers` inline (legacy-parity); `execution=pipeline` fans out one Activity per tier via `workflow.Future` then barriers into sequential Phase 3/3.5/4 (parallelism gain over legacy `PipelineStrategy` which was actually per-tier serial). Phase 2-4 ActivityOptions intentionally drop `HeartbeatTimeout` because the legacy inner loops don't heartbeat per batch yet. Verified via Temporal `testsuite` workflow tests (3/3 pass): pipeline mode schedules N Phase2 calls for N tiers, sequential mode dispatches a single bulk Phase2, and FailRun fires on workflow failure with the configured 5-attempt exponential backoff exactly matching `phase1Opts`.
  - ✅ chunk 4: `Phase5Timeline` + `Phase55ItemClassify` Activities complete the 8-phase chain. `apps/worker/internal/schedule` reads legacy `cfg.Schedule[]` entries, resolves each profile name to its region, and idempotently upserts a Temporal Schedule per row (`gogg-crawl-{profile}` on `crawl-{region}` task queue). Replaces `robfig/cron` in legacy `gogg crawl daemon` — both `cmd/crawl/{run,daemon}` are marked `[DEPRECATED]` in their cobra help text and `internal/crawler` package doc; they stay one release cycle as the rollback escape hatch per plan §3. Verified end-to-end against compose Temporal: schedules created on first boot (`schedule_created` log), `schedule_updated` on subsequent boots; `temporal schedule list` shows correct `NextRunTime` matching the cron expressions. Workflow test suite extended (5/5 pass) — full-chain sequential walks every Activity in order; schedule pkg tests (4/4 pass) cover profile resolution, queue selection, and rejection of malformed entries.
- **D · Frontend rewrite** — `refactor/phase-d-frontend` (in progress)
  - ✅ chunk 1: `apps/web` toolchain — vite 5 + react 18 + tailwind 3 + tanstack-query 5 + radix + i18next + zustand + react-router 6; tsconfig (strict + noUncheckedIndexedAccess) with path aliases `@app`/`@features`/`@shared`; smoke App.tsx renders Tailwind classes (`bg-gogg-ink`, `bg-gogg-gold/10`) through custom theme tokens. `npm run dev` + `npm run build` clean (dist: 5.5 kB CSS gzipped 1.7 kB, 143 kB JS gzipped 46 kB). Makefile adds `run-web` + `build-web`. Legacy `web/` stays untouched until D ships.
  - ✅ chunk 2: vite 5 → vite 8 + vitest 2 → vitest 4 + plugin-react 6 (esbuild 0.28 — `npm audit` clean, no high CVEs); i18n bootstrap with i18next + react-i18next + browser LanguageDetector, namespaces `common` + `rankings`, zh-CN default + en-US, type-safe via `CustomTypeOptions` module augmentation; design tokens layered on tailwind — brand primitives (`gogg-gold`/`gogg-ink`) plus semantic `surface-*` / `fg-*` / `border-*` / `accent-*` plus per-tier `tier-*` palette + 3 tier gradients + `animate-skeleton` keyframe; `cn()` helper (clsx + tailwind-merge); base components `Button`/`Tag`/`Skeleton`/`Select` under `@shared/ui` (cva variants split into `*.variants.ts` so react-refresh boundaries stay clean, Select wraps Radix primitives); `LanguageSwitcher` under `@shared/i18n`; vitest.config.ts (jsdom + setupFiles) + `src/test/setup.ts` (jest-dom matchers + cleanup hook); 4 component test files / 6 cases pass; `npm run dev` + `lint` + `type-check` + `build` clean (dist: 9.83 kB CSS gzipped 2.99 kB, 303 kB JS gzipped 99 kB).
  - chunk 3: GraphQL codegen wired against gqlgen schema; TanStack Query hooks generated
  - chunk 4: Rankings page rewrite (hooks split + presenter component)
  - chunk 5: Router skeleton + placeholder pages for champion-detail / summoner / login / me
  - chunk 6: Vitest hook tests + Playwright golden path
- **E · New features** — champion detail, summoner search, user system
- **F · Production hardening** — k8s manifests, Terraform, observability, runbooks

The legacy `internal/server/` package is `Deprecated` as of chunk 4;
do not add features there. Bug fixes for security/correctness only,
and mirror them into `apps/api/internal/*` in the same PR.

## Commands

### Local dev stack
```bash
make dev          # postgres + redis + temporal + mailhog
make dev-down     # stop
make dev-reset    # stop + drop volumes (data loss)
```

### Legacy backend (still functional during Phase A–B)
```bash
go build .                          # build the legacy ./gogg binary
./gogg serve                        # HTTP API on :8080
./gogg crawl run --profile daily_kr # crawler with the existing config.yaml
go test ./...
```

### Legacy frontend
```bash
cd web
npm install
npm run dev         # Vite proxy → localhost:8080/api
npm run build       # type-check + build → web/dist/
npm run type-check
```

### New monorepo targets (skeleton — fully populated as Phase B+ lands)
```bash
make build-api    # apps/api/cmd/api → bin/gogg-api
make build-worker # apps/worker/cmd/worker → bin/gogg-worker
make lint         # golangci-lint + web lint
make test         # go test + web test
make ci           # vet + lint + test (CI parity)
make gen          # sqlc + gqlgen + graphql-codegen
make migrate-up   # apply migrations from packages/sqlc/migrations/
make migrate-new name=add_user_favorites
```

## Architectural rules (enforced by review)

1. **Service layer owns business logic.** Transports
   (`apps/api/internal/transport/graphql`,
   `apps/api/internal/transport/rest`) call into the same
   `internal/service/*` packages. No SQL in resolvers, no
   HTTP-aware code in services.
2. **Data access goes through sqlc.** Hand-written `pgx`
   queries are allowed only for the truly-dynamic case (e.g.
   filter-built WHERE clauses); when you write one, leave a
   `// sqlc-skip: <reason>` comment.
3. **Secrets never land in git.** Use `deploy/secrets/*.enc.yaml`
   via SOPS. CI verifies via gitleaks.
4. **Migrations are forward-only in spirit.** Every migration
   ships with a corresponding `.down.sql`, but down migrations
   are for local dev only; production rolls forward.
5. **The legacy stack is sacred until its replacement ships.**
   Modifying `internal/server/` or `internal/crawler/` during
   the refactor is only OK if the change is being mirrored into
   the new module, or it's an outright bug fix that can't wait.

## Where to look next

- The refactoring plan: `/home/zrt/.claude/plans/radiant-wobbling-pizza.md`
- Why these decisions: `docs/architecture/adr/000{1,2,3}-*.md`
- How to contribute: `docs/contributing.md`
- Local secrets workflow: `deploy/secrets/README.md`
- Dev stack details: `deploy/compose/docker-compose.dev.yml`
