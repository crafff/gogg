# Chapter 01 · Overview

> Goal: by the end of this chapter you can describe, in one paragraph, what GOGG does, which processes run in production, and how data flows between them.

## What problem does GOGG solve

If you play League of Legends competitively, you want to know:

1. **Champion rankings** — which champion has the best win rate this patch, in this region, at this rank, in this position?
2. **Champion detail** — what items / runes / matchups should I build into?
3. **Summoner search** — how is some specific player performing? What's their recent match history?

Existing tools (op.gg, u.gg) answer those questions but are tuned for global audiences. GOGG aims at the **Korean (KR) + North American (NA1) regions** with a **bilingual zh-CN + en-US UI**, designed to be cloud-agnostic so it can be deployed to AWS or a domestic Chinese cloud without major rework.

V1 ships:

- **Champion rankings** (✅ done, Phase D)
- **Champion detail page** (🚧 Phase E)
- **Summoner search** (🚧 Phase E)
- **User accounts** (🚧 Phase E — Discord/Google OAuth first; Riot RSO once approved)

This tutorial covers the state after Phase D ships, which is everything except the three V1 features.

## The shape of the system

GOGG runs three long-lived processes:

```
              ┌──────────────────────────────────┐
              │  apps/web  (React SPA, in Nginx) │
              │  served as static files          │
              └──────────────┬───────────────────┘
                             │ /graphql + /api/v1
                             ▼
              ┌──────────────────────────────────┐
              │  apps/api  (gogg-api binary)     │
              │  - chi HTTP router               │
              │  - gqlgen GraphQL BFF            │
              │  - REST compat at /api/v1        │
              │  - JWT issuer + OAuth callbacks  │
              └─────┬────────────────┬───────────┘
                    │                │
                    │ pgx            │ go-redis
                    ▼                ▼
              ┌──────────┐     ┌──────────┐
              │ Postgres │     │  Redis   │
              └────┬─────┘     └──────────┘
                   │
                   │ same DB
                   │
              ┌────▼─────────────────────────────┐
              │  apps/worker  (gogg-worker)      │
              │  - Temporal worker               │
              │  - hosts CrawlRegionWorkflow     │
              │  - hosts 8 phase activities      │
              └─────┬────────────────────────────┘
                    │ HTTPS
                    ▼
              ┌──────────────────────────────────┐
              │  Riot Games API                  │
              │  (regional + platform endpoints) │
              └──────────────────────────────────┘
```

**Read this slowly:**

- `apps/web` is a static React app. In dev, vite serves it on port `5173`. In prod, Nginx serves the built files.
- `apps/api` is **the** server. It owns the HTTP surface: GraphQL for the frontend, REST `/api/v1` for backwards compat (old web) + scripts, OAuth + JWT for auth, Prometheus metrics. It reads from Postgres + Redis. It never talks to Riot.
- `apps/worker` is the **crawler**. It runs in the background, periodically asking Riot "give me the top players in KR, then give me their matches, then process those matches." It writes everything into the same Postgres that `apps/api` reads from. It uses Temporal as its scheduler + state machine.
- Postgres is the source of truth for everything: ranks, matches, players, users.
- Redis is for caching (the rankings query, computed once per crawl run, served thousands of times) and for sessions/refresh tokens.
- Temporal is "Kubernetes for workflows" — it owns the *state* of the crawler (which phase are we in? did Phase 2 finish for the CHALLENGER tier? when's the next scheduled run?), even across worker restarts.

## What about the legacy stack?

If you grep around, you'll see:

- `main.go` at the repo root
- `internal/server/`, `internal/crawler/`, `internal/storage/`, `internal/riotapi/`, `internal/config/`
- `cmd/crawl/`
- `web/` (a separate React app, not under `apps/`)

That's the **original** single-binary MVP. One process did everything: HTTP server + crawler + DB layer + Riot client. The refactor split it into the three binaries above. The legacy code stays in the tree **as a rollback path** — if anything blows up in production, ops can `./gogg serve` and roll back to the old behavior.

The legacy paths are marked `Deprecated` in their package docs and `[DEPRECATED]` in their CLI help. **Do not add features there**. Bug fixes only, mirrored into `apps/*` in the same PR. The legacy stack will be deleted one release cycle after Phase E ships.

## A worked example: where does "Lux 53.2% win rate" come from?

This is the trace you'll see in [Chapter 07](./07-end-to-end.md) in detail, but here it is at a high level:

1. **Periodic crawl** (`apps/worker`): once a day, Temporal kicks off `CrawlRegionWorkflow` for KR.
2. **Phase 0**: pull the current patch list from CDragon → write to `game_versions` table.
3. **Phase 1**: ask Riot for the top players (CHALLENGER, GRANDMASTER, MASTER) → write to `player_rank_snapshots`.
4. **Phase 2**: for each of those players, ask Riot "what matches did they play?" → store match IDs in `matches`.
5. **Phase 3**: for each match, ask Riot for the details (who picked what, who won) → write to `match_participants`.
6. **Phase 4**: aggregate the participants table per champion → cached aggregate per (patch, region, tier, position).
7. **Phase 5, 5.5**: timeline + item classification (not always loaded).
8. **User loads `/rankings`**: vite serves `apps/web/dist/index.html` → React router lands on `RankingsPage` → `useChampionRankingsQuery` hook → POST `/graphql` with the filter.
9. **GraphQL resolver** in `apps/api`: calls `rankings.Service.ListChampions(filter)` → calls sqlc-generated `ListOverallRankings` → SQL aggregates over `match_participants` for the given filter slice.
10. **Redis cache** in front of the service: subsequent identical queries return in <10 ms.
11. **JSON response**: serializes back through GraphQL → vite proxy → browser.
12. **React renders**: TanStack Query unwraps the JSON, the rankings table shows "Lux · 53.2% WR · ...".

If you remember exactly one paragraph from this chapter, make it that 12-step list.

## The repo at a glance

```
gogg/
├── apps/                    NEW: post-refactor monolith pieces
│   ├── api/                 the HTTP server (gogg-api)
│   ├── worker/              the Temporal worker (gogg-worker)
│   └── web/                 the React SPA
├── packages/                NEW: shared Go libraries
│   ├── domain/              champion/tier/region enums + error codes
│   ├── sqlc/                migrations + queries + generated bindings
│   ├── riotapi/             Riot API client (with rate limiter)
│   └── proto/               reserved for future gRPC
├── deploy/                  NEW: infra-as-code
│   ├── docker/              Dockerfiles
│   ├── compose/             local dev stack (docker-compose.dev.yml)
│   ├── k8s/                 Kubernetes manifests (Phase F)
│   ├── terraform/           AWS + Aliyun modules (Phase F)
│   ├── observability/       Prometheus + Grafana (Phase F)
│   └── secrets/             SOPS-encrypted env files
├── docs/                    NEW: ADRs + runbooks + this tutorial
│   ├── architecture/adr/    architectural decision records
│   ├── runbooks/            on-call procedures
│   ├── tutorial/            ← you are here
│   ├── manual-verification.md
│   └── contributing.md
├── internal/                LEGACY: do not edit
├── cmd/                     LEGACY
├── main.go                  LEGACY entry point
├── web/                     LEGACY frontend
├── go.mod / go.work         Go workspace + module
├── Makefile                 every dev workflow
└── CLAUDE.md                load-bearing project context (read first)
```

## Why so many directories?

Each was a deliberate choice:

| Choice | ADR |
|---|---|
| Modular monolith over microservices | [`docs/architecture/adr/0001-modular-monolith.md`](../architecture/adr/0001-modular-monolith.md) |
| sqlc instead of an ORM | [`docs/architecture/adr/0002-sqlc-over-ent.md`](../architecture/adr/0002-sqlc-over-ent.md) |
| GraphQL + REST dual surface | [`docs/architecture/adr/0003-graphql-plus-rest.md`](../architecture/adr/0003-graphql-plus-rest.md) |

You don't have to read those now — they're rationale records, denser than this tutorial. You'll see them referenced in later chapters when the relevant tradeoff comes up.

## Try this

```bash
# Get a count of every Go file in apps/ vs the legacy internal/
find apps/ -name '*.go' -not -path '*/generated/*' | wc -l
find internal/ -name '*.go' | wc -l
```

The new monolith is roughly the same size as the legacy code. The split isn't about line count — it's about layered responsibilities (we'll see how in Chapter 05).

```bash
# Peek at the three Makefile entry points you'll use most
grep -E '^(run-(api|worker|web)|dev):' Makefile
```

Those are exactly the three processes from the diagram above. Each maps to one binary.

## Up next

[Chapter 02 — Setup](./02-setup.md) walks you through booting the entire stack and watching the rankings page render. Skip ahead if you've already gone through `docs/manual-verification.md`.
