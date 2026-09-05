# Chapter 07 · End-to-end trace — a winRate's life

> Goal: by the end of this chapter you can describe, with file paths and function names, every layer that touches a single `winRate: 53.2%` value on its way from Riot's API to a row in your browser. This is the "aha moment" chapter — everything from chapters 03–06 connects here.

We'll trace this question:

> When the rankings page shows "Lux · Mid · 53.2% WR · 1247 games", where did each of those numbers come from, and what happens between then and now if a user clicks the Mid → Top chip?

You won't run code in this chapter. You'll **read** along with the trace, with a paused-in-your-head model of each file.

## Step 1 — The hourly tick

Outside our codebase, Riot's match history APIs got an update. A new ranked game finished. Riot now knows about it.

Inside our codebase, a Temporal **Schedule** ticks. Look:

```bash
cat apps/worker/internal/schedule/schedule.go | head -50
```

`BuildPlan(cfg)` reads `internal/config/config.go`'s `cfg.Schedule[]`, sees an entry like:

```yaml
schedule:
  - profile: daily_kr
    cron: "0 3 * * *"
```

`Upsert(ctx, c, plans)` registers a Temporal Schedule `gogg-crawl-daily_kr` on the `crawl-kr` task queue, with the cron expression `"0 3 * * *"`. At 03:00 KR time, Temporal fires.

When Temporal fires the Schedule, it enqueues a `CrawlRegionWorkflow` start request onto the `crawl-kr` task queue with `Input: { ProfileName: "daily_kr" }`.

## Step 2 — The worker picks it up

The worker (started by `make run-worker`, see `apps/worker/cmd/worker/main.go`) is polling the `crawl-kr` task queue. It dequeues the task and begins executing the workflow.

The first thing `CrawlRegionWorkflow` does is `createRun(ctx, profile)`. That schedules a `CreateRun` activity:

```bash
cat apps/worker/internal/activity/crawl/run.go | head -40
```

The activity inserts a row into the `runs` table:

```sql
INSERT INTO runs (status, profile, mode, target_tiers, started_at, ...)
VALUES ('running', 'daily_kr', 'incremental', ARRAY['CHALLENGER',...], now(), ...)
RETURNING id;
```

It returns `Run ID = 42` (let's say). The workflow records this in its history.

## Step 3 — Phase 0: version sync

The workflow calls `Phase0VersionSync`. The activity:

```bash
cat apps/worker/internal/activity/crawl/phase0.go | head -60
```

1. Hits CDragon's JSON manifest (`https://raw.communitydragon.org/...`) to get the current patch list. CDragon is a community project that mirrors Riot's static data; Riot's own endpoint for this is finicky.
2. Loops through versions, upserting each into `game_versions`:
   ```sql
   INSERT INTO game_versions (version, fetched_at, is_latest)
   VALUES ($1, $2, $3)
   ON CONFLICT (version) DO UPDATE SET fetched_at = EXCLUDED.fetched_at;
   ```
3. Flips `is_latest = true` on the newest patch and `false` on everything older.
4. Returns `Phase0Output{ResolvedVersion: "14.20.1", LatestVersion: "14.20.1", UpsertedCount: 226}`.

The workflow then calls `PinRunVersion(runID=42, version="14.20.1")` which writes `version` into the `runs` row so the audit log shows which patch this run was scoped to.

## Step 4 — Phase 1: rank snapshot

The workflow calls `Phase1RankSnapshot`. The activity:

```bash
cat apps/worker/internal/activity/crawl/phase1.go | head -80
```

For each tier in `(CHALLENGER, GRANDMASTER, MASTER)`:

1. Calls Riot's league API: `GET https://kr.api.riotgames.com/lol/league/v4/challengerleagues/by-queue/RANKED_SOLO_5x5`. With auth header `X-Riot-Token: <RIOT_API_KEY>`.
2. The response is a JSON like `{"tier":"CHALLENGER","entries":[{"summonerId":"...","leaguePoints":1234,"wins":89,"losses":76},...]}`.
3. For each entry, the activity asks Riot for the puuid:
   `GET https://kr.api.riotgames.com/lol/summoner/v4/summoners/{summonerId}`.
4. Upserts the puuid + game name + tag line into `players`.
5. Inserts a row into `player_rank_snapshots` with `source='phase1', queue='RANKED_SOLO_5x5', tier='CHALLENGER', league_points=1234, wins=89, losses=76, run_id=42`.

Heartbeating every 25 players so Temporal knows the activity isn't stuck.

Returns `Phase1Output{TierCounts: {"CHALLENGER": 301, "GRANDMASTER": 752, "MASTER": 2003}}`.

If Riot returns HTTP 401 (expired key) at any point, the activity returns an error. The workflow's `phase1Opts` retry policy kicks in: wait 5s, retry. Wait 10s, retry. Wait 20s. Wait 40s. Wait 80s. Five attempts in total; then the workflow runs `FailRun` and stamps `runs.status='failed'`.

## Step 5 — Phase 2: match IDs

The workflow's post-Phase1 dispatcher picks sequential or pipeline mode based on `profile.Execution`. Say it's pipeline:

```go
for _, tier := range profile.TargetTiers {
    runPhase2(ctx, runID, profile, version, startedAt, []string{tier})
    runLaterPhases(ctx, runID, profile, version, startedAt)
}
```

Pipeline mode is tier-first. `CHALLENGER` runs Phase 2 through Phase 5.5 before `GRANDMASTER` starts Phase 2, so high-rank data becomes usable earlier. The Phase 2 activity for one tier:

```bash
cat apps/worker/internal/activity/crawl/phase2.go | head -50
```

1. For each player in `player_rank_snapshots` (filtered to this tier + run_id=42):
   `GET https://asia.api.riotgames.com/lol/match/v5/matches/by-puuid/{puuid}/ids?queue=420&count=20`.

   Note the *regional* host (`asia.` not `kr.`). That's the host that knows match-data.

2. The response is `["KR_<match_id>", "KR_<match_id>", ...]` — a list of match IDs.

3. Each new match ID is upserted into `matches`:
   ```sql
   INSERT INTO matches (match_id, region, queue_id, fetch_status, fetched_at)
   VALUES ($1, 'KR', 420, 'pending', now())
   ON CONFLICT (match_id) DO NOTHING;
   ```

Returns `Phase2Output{NewMatches: 5237, DuplicateMatches: 1819}`.

## Step 6 — Phase 3: match details

In pipeline mode, Phase 3 runs immediately after the current tier's Phase 2. It still operates across pending matches scoped by region + version, because Riot match data cannot be fetched directly by tier:

```bash
cat apps/worker/internal/activity/crawl/phase3.go | head -40
```

1. Selects matches with `fetch_status='pending' AND region='KR' AND version='<resolved version>'`.
2. For each match: `GET https://asia.api.riotgames.com/lol/match/v5/matches/{match_id}`.
3. The response is detailed: who picked what, who won, KDA, items, etc.
4. For each of the 10 participants:
   ```sql
   INSERT INTO match_participants
     (match_id, puuid, champion_id, team_position, win, kills, deaths, assists, ...)
   VALUES (...);
   ```
   So `lux_match_42` gets a row with `champion_id=99`, `team_position='MIDDLE'`, `win=true`, `kills=8`, `deaths=2`, `assists=15`.
5. Updates the `matches` row with `version='14.20.1'`, `fetch_status='done'`.

This is the table that powers the rankings query.

## Step 7 — Phases 3.5 → 5.5

The remaining phases enrich the data:

- **3.5 (OnDemandRank)**: for participants who weren't in the original Phase 1 snapshot (e.g. they're in DIAMOND, not MASTER+), look up their current rank.
- **4 (AvgTierCalc)**: a DB-only pass. For each match: `UPDATE matches SET avg_tier = (SELECT mode of participants' tiers) WHERE match_id = ...`. This is the "what tier was this match?" tag that the rankings page filters on.
- **5 (Timeline)**: optional. Pulls the per-minute match timeline (used for Phase E champion detail).
- **5.5 (ItemClassify)**: optional. Classifies item events into boots, starter items, etc.

Finally `CompleteRun(runID=42)` flips `runs.status='completed'`, `runs.ended_at=now()`.

## Step 8 — The crawler is done. Now the user opens the page

Meanwhile, you (the user) open <http://localhost:5173> in your browser. The vite dev server returns `index.html`:

```bash
cat apps/web/index.html
```

That references `/src/main.tsx`. Vite serves it, transforming TypeScript on the fly. `main.tsx` mounts `<App />` → `RouterProvider` matches `/` against the routes table → `Navigate to /rankings` → matches `/rankings` → lazy-loads `RankingsPage`.

`RankingsPage` mounts and calls:

```ts
const versionsQuery = useVersionsQuery();    // codegen'd hook
const regionsQuery = useRegionsQuery();
const rankings = useRankingsQuery({ filters, limit, enabled });
```

Each of those is a TanStack Query subscription. TanStack Query checks its in-memory cache. First load: cache miss for all three. It fires the network calls.

## Step 9 — The frontend posts to `/graphql`

The codegen'd fetcher in `apps/web/src/shared/api/generated/hooks.ts` posts:

```http
POST /graphql HTTP/1.1
Host: localhost:5173
Content-Type: application/json

{"query": "query ChampionRankings($filter: ChampionRankingsFilter) { championRankings(filter: $filter) { items { championId championName teamPosition games wins losses winRate pickRate banRate kda } totalMatches resolvedVersion } }", "variables": {"filter": {"position": "", "tierGroup": "ALL", "region": "", "version": "latest", "minGames": 20, "limit": 40}}}
```

Vite's proxy (configured in `vite.config.ts`) forwards `/graphql` to `http://localhost:8080/graphql`. The browser sees a 200 OK at `:5173`; the backend sees a request at `:8080`.

## Step 10 — chi receives, middleware wraps

The API binary (`apps/api/cmd/api/main.go`) is running. chi receives the POST. The middleware chain wraps it:

1. `Recoverer` — wraps with panic-recovery.
2. `RequestID` — assigns a request ID, stores in context.
3. `Logger` — emits a structured log line on response.
4. `metrics.Middleware` — starts a latency timer.
5. `corsMiddleware` — allows `localhost:5173`.

Then `r.Handle("/graphql", gqlserver.NewHandler(gqlRoot))` dispatches to the gqlgen handler.

## Step 11 — gqlgen resolves

gqlgen parses the query, sees `championRankings(filter: ...)`, and invokes the resolver:

```bash
cat apps/api/internal/transport/graphql/resolver/rankings.resolvers.go
```

```go
func (r *queryResolver) ChampionRankings(ctx context.Context, filter *model.ChampionRankingsFilter) (*model.RankingsResult, error) {
    result, err := r.rankings.ListChampions(ctx, toServiceFilter(filter))
    if err != nil {
        return nil, err
    }
    return toGraphQLResult(result), nil
}
```

`toServiceFilter` converts the GraphQL input type into the service-layer `ListChampionsFilter` struct. The conversion is mostly 1:1.

## Step 12 — The service consults the cache

```bash
cat apps/api/internal/service/rankings/service.go | head -60
```

```go
func (s *Service) ListChampions(ctx context.Context, filter ListChampionsFilter) (Result, error) {
    key := cacheKey(filter)
    return cache.GetOrLoad(s.cache, ctx, key, 6*time.Hour, func() (Result, error) {
        return s.fetch(ctx, filter)
    })
}
```

Cache lookup → cache miss (first request) → calls `s.fetch`.

## Step 13 — sqlc fires the SQL

`fetch` calls the right sqlc method based on whether `filter.Position` is empty:

```go
if filter.Position == "" {
    rows, err := s.queries.ListOverallRankings(ctx, params)
} else {
    rows, err := s.queries.ListRankingsByPosition(ctx, params)
}
```

The generated `ListOverallRankings` function (in `packages/sqlc/gen/rankings.sql.go`) executes the big CTE query. Postgres:

1. Filters `matches` by queue + version + region + tier (`filtered_matches` CTE) — say 12,500 matches qualify.
2. Joins `match_participants` to those matches → 125,000 participant rows.
3. Groups by `(champion_id, team_position)` → ~150 champion-position pairs.
4. Aggregates `wins / games` per group → champion-position win rate.
5. Rolls up to per-champion totals.
6. Applies `min_games >= 20` floor → drops low-sample champions.
7. Orders by win rate DESC, limit 40.

Lux's `champion_id = 99` has 1,247 games in MIDDLE position. 663 wins → 1247 games → 53.17% winRate. The Postgres-rounded value is `0.5317`. The matching match_participants count and ban count produce pickRate = 0.18, banRate = 0.07.

The row comes back to Go as `sqlcgen.ListOverallRankingsRow{ChampionID: 99, ChampionName: "Lux", TeamPosition: ["MIDDLE"], Games: 1247, Wins: 663, Losses: 584, WinRate: 0.5317, ...}`.

## Step 14 — Through the cache layer back out

`fetch` returns a `Result`. `GetOrLoad` marshals it to JSON and SET in Redis with TTL = 6h. Returns to the resolver.

Subsequent identical requests (same filter) within 6 hours skip all of steps 13 entirely and return from Redis in ~3 ms.

## Step 15 — gqlgen serializes

gqlgen converts the `Result` struct into the GraphQL response shape:

```json
{
  "data": {
    "championRankings": {
      "items": [
        {"championId": 99, "championName": "Lux", "teamPosition": ["MIDDLE"], "games": 1247, "wins": 663, "losses": 584, "winRate": 0.5317, "pickRate": 0.18, "banRate": 0.07, "kda": 2.21},
        ...
      ],
      "totalMatches": 12500,
      "resolvedVersion": "14.20.1"
    }
  }
}
```

gqlgen also sets the response Content-Type and HTTP status. The middleware Logger writes a log line, `metrics.Middleware` records the latency, chi returns up the stack.

## Step 16 — The browser receives + TanStack Query unwraps

The browser receives the JSON. The codegen'd `useChampionRankingsQuery` hook's fetcher parses it. TanStack Query stores it in its cache keyed by `["ChampionRankings", { filter }]`.

The hook returns `{ data: <response>, isLoading: false, ... }`. The wrapping `useRankingsQuery` flattens:

```ts
return {
  items: query.data?.championRankings.items ?? [],
  totalMatches: query.data?.championRankings.totalMatches ?? 0,
  resolvedVersion: query.data?.championRankings.resolvedVersion ?? null,
  isLoading: query.isLoading,
  ...
};
```

React re-renders. `RankingsTable` receives `items` (an array of 40 row objects). It maps to `<tr>` elements:

```bash
cat apps/web/src/features/rankings/components/RankingsTable.tsx | head -60
```

```tsx
<td className="px-3 py-2 font-medium text-fg-default">{row.championName}</td>
<PercentCell value={row.winRate} />
```

`PercentCell` formats `0.5317` as `"53.2%"`. The number you see is the cell text node.

## Step 17 — The user clicks a chip

Now the interesting part. The user clicks the "Top" position chip.

1. `RankingsFilters` (`apps/web/src/features/rankings/components/RankingsFilters.tsx`) handles the click → calls `props.onPositionChange("TOP")`.
2. That's wired in `RankingsPage` to `filters.setPosition("TOP")` from `useRankingsFilters`.
3. `setPosition` is wrapped by the `updateSelected` helper, which fires `onBeforeCommit` (passed in by `RankingsPage`).
4. `onBeforeCommit` calls `fade.beginExit()` → fade state machine moves to `"fading-out"`.
5. The page re-renders. The table's opacity transitions to 0 (200ms CSS animation).
6. The 180ms fade-out timer in `useFadeTransition` fires → state moves to `"hidden"`.
7. The `useEffect` watching `fade.phase` fires `filters.commit()` → `committed.position` becomes `"TOP"`.
8. `useRankingsQuery` re-renders. Its `useChampionRankingsQuery` hook has new variables → TanStack Query checks the cache (miss for this new filter) → POSTs to `/graphql` again.
9. Steps 9–15 repeat for the TOP-only slice. New `Result.items` lands.
10. The second `useEffect` watching `isFetching` fires `fade.beginEnter()` → state moves to `"fading-in"`.
11. CSS animates opacity 0 → 1 over 220ms → state moves to `"shown"`.
12. The new table is visible. Lux is still there at MIDDLE? No — `tierGroup="ALL"`, `position="TOP"` filter means only TOP-played champions in the slice. Lux drops off (or stays if she has ≥20 TOP games).

Total elapsed time on your machine, with Redis warm: maybe 300ms including the fade.

## What you should now believe

Every single layer in chapters 03–06 just got exercised:

- **Chapter 03** — the schema (`matches`, `match_participants`), the sqlc query
- **Chapter 04** — Phase 0 → Phase 5.5 wrote the rows
- **Chapter 05** — chi middleware, gqlgen resolver, service, cache, sqlc gen
- **Chapter 06** — vite proxy, codegen hook, TanStack Query, filter state, fade transition

If you can describe this trace from memory without looking at the chapter, you understand the project end-to-end.

## Exercises

1. **Find the rate**: open `psql` while a workflow is running. Periodically run `SELECT count(*) FROM match_participants;`. Watch the count grow during Phase 3. Compute the throughput: rows / second.

2. **Cache hit/miss**: hit `/api/v1/rankings/champions?region=KR&limit=5` twice. The first one is slow (sqlc query against Postgres). The second one is fast (Redis hit). Watch the latency in the API logs.

3. **Add a metric**: in `apps/api/internal/service/rankings/service.go`, add a counter that increments on each cache miss. Hit the endpoint a few times. Check `/metrics` for your new counter name.

4. **Trace the filter**: with DevTools network tab open, change the version filter dropdown. Look at the new `/graphql` POST body. Then look at `apps/web/src/features/rankings/hooks/useRankingsQuery.ts` to see where `filters.version` becomes the request `filter.version`.

## Up next

[Chapter 08 — Auth + secrets](./08-auth-secrets.md) covers JWT, OAuth flow, and how SOPS keeps the Riot API key out of git.
