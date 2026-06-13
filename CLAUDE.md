# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**GOGG** is a League of Legends champion rankings app. It consists of:
- **Go backend** — REST API serving champion ranking stats from PostgreSQL
- **React frontend** — TypeScript/Vite UI for browsing and filtering rankings
- **Crawler** (`playground/crawler-test/`) — standalone tool that fetches match data from the Riot API and populates the database

## Commands

### Backend
```bash
go build .          # Build the binary
./gogg              # Run the server (needs DATABASE_DSN and optionally PORT, WEB_DIST_DIR)
go test ./...       # Run tests
```

### Frontend
```bash
cd web
npm install
npm run dev         # Dev server with Vite proxy to localhost:8080/api
npm run build       # TypeScript check + Vite build → web/dist/
npm run type-check  # Type-check only
```

### Crawler
```bash
cd playground/crawler-test
go run ./cmd/crawler   # Run crawler (reads config.yaml)
```

### Database (Docker)
The default DSN is `postgres://gogg:goggpass@localhost:55433/gogg`. The crawler's `internal/storage/schema.go` defines and creates all tables on startup.

## Architecture

### Backend (`internal/server/`)

- **app.go** — `App` struct wires together the HTTP server, PostgreSQL pool (`pgxpool`), and repositories (`RankingStore`, `VersionStore`). Handles graceful shutdown.
- **config.go** — Reads `PORT`, `DATABASE_DSN`, `WEB_DIST_DIR` from environment.
- **router.go** — `http.ServeMux` routing + CORS middleware. Key routes: `GET /api/rankings/champions`, `/healthz`, `/readyz`, static file serving for the built frontend.
- **rankings.go** — `RankingStore` with two query paths:
  - `GetOverallRankings` — CTE-based query that aggregates champions across all positions; supports a `PositionThreshold` (e.g. 5%) for multi-position heroes and returns positions as a PostgreSQL array.
  - `GetRankingsByPosition` — simpler single-position aggregation.
  - `ChampionRankingQuery` carries all filter params: `QueueID` (default 420), `GameVersion` ("latest" resolves via `VersionStore`), `Position`, `MinGames`, `Limit`, `PositionThreshold`.
- **versions.go** — `VersionStore.GetLatestVersion()` queries the `game_versions` table.

### Frontend (`web/src/`)

Routing is handled by React Router DOM; the app currently renders a single `<Rankings>` page.

- **types/index.ts** — Shared types (`Position`, `RankingsFilters`, `RankingsResponse`, `ChampionRankingItem`).
- **services/api.ts** — `fetchChampionRankings(filters)` calls `/api/rankings/champions` with `URLSearchParams`.
- **hooks/useRankings.ts** — Fetching hook that manages `loading`/`error`/`items` state and request cancellation.
- **pages/Rankings/Rankings.tsx** — Complex component: infinite scroll via `IntersectionObserver`, position filter switching with fade animations (180 ms out / 220 ms in), viewport-height locking during transitions. Constants: `PAGE_SIZE=40`, `FIXED_MIN_GAMES=20`.
- **components/UI/** — `FiltersPanel`, `RankingsTable`, `StateMessages` (loading/error/empty).

### Crawler (`playground/crawler-test/`)

Task-queue + worker-pool architecture:

- **internal/riot/** — Riot API HTTP client with rate limiting (`golang.org/x/time`). Separate `api_*.go` files per endpoint (account, league, match, match_detail, version).
- **internal/crawler/** — `TaskHandler` registry (`router.go`). Task types: `AccountByRiotID`, `ChallengeLeaguesByQueue`, `Version`, `MatchByPUUID`, `MatchDetailByMatchID`. Workers consume from a buffered channel.
- **internal/process/** — Result processors that transform API responses into DB writes.
- **internal/storage/** — Repository pattern over `pgxpool`. `schema.go` creates tables on startup: `players`, `player_rank_current`, `game_versions`, `player_match_sync_state`, `matches`, `match_participants`, `bans`.
- **config.yaml** — Riot API key, database DSN, `worker_count` (default 5), `queue_size` (default 100).