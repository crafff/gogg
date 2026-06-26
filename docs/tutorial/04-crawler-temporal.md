# Chapter 04 · Crawler + Temporal

> Goal: by the end of this chapter you understand what each of the 8 crawl phases does, how Temporal owns the workflow state, why a retry-after-Phase-1-401 happens automatically, and how the schedule is registered.

This chapter is the densest one. We're covering two things at once: the *domain* (Riot's API + what we extract from it) and the *infrastructure* (Temporal). Take it slow.

## Riot's API in 60 seconds

Riot's HTTP API has two flavors of endpoint:

| Routing | Used for | Examples |
|---|---|---|
| **Platform** routing (region-specific host) | "live" or rank data: leaderboards, summoner info | `https://kr.api.riotgames.com/...`, `https://na1.api.riotgames.com/...` |
| **Regional** routing (continent-grouped host) | "static" or aggregate data: match details, match history | `https://asia.api.riotgames.com/...` (covers KR, JP), `https://americas.api.riotgames.com/...` (covers NA, BR, LAN/LAS) |

Both are HTTPS, both require an API key in the `X-Riot-Token` header. Both rate-limit aggressively — a development key gets ~100 requests / 2 minutes.

GOGG only deploys to KR + NA1 in V1. The `packages/riotapi/` client owns the HTTP DTOs, rate limiter, and platform/regional Riot calls. The worker builds one client per configured platform region and passes the matching client into each activity.

## What the 8 phases do

A "crawl run" is one execution of `CrawlRegionWorkflow` for one region. It's labeled with a profile from the crawler config (`daily_kr`, `daily_na1`, etc.), and walks through 8 phases in order:

| # | Name | What it asks Riot | What it writes |
|---|---|---|---|
| 0 | VersionSync | "what patches exist?" (CDragon, not Riot directly) | `game_versions` |
| 1 | RankSnapshot | "top players in CHALLENGER / GRANDMASTER / MASTER" | `players`, `player_rank_snapshots` |
| 2 | MatchIDCollection | "what matches did each of those players play recently?" | `matches` (just the IDs + queue + region) |
| 3 | MatchDetails | "give me the participants of these matches" | `match_participants`, `matches` (fills version, avg_tier, fetch_status='done') |
| 3.5 | OnDemandRank | "what's the current rank of these participants?" (for low-tier players not in Phase 1's snapshot) | `player_rank_snapshots` (source='phase3.5') |
| 4 | AvgTierCalc | (DB-only, no Riot call) — for each match, compute `avg_tier` from participants | `matches.avg_tier` |
| 5 | Timeline | "give me the per-minute timeline for these matches" | `match_timelines` |
| 5.5 | ItemClassify | (DB-only) classify each item event from the timeline (boots, starter items) | `match_boots`, `match_starter_items` |

Phases 5 and 5.5 are controlled by the unified worker config
(`config/dev.yaml` locally or `deploy/secrets/dev.enc.yaml` through
SOPS). The Phase E "champion detail" feature flips them on.

## Why Temporal?

The old stack ran all 8 phases as a single Go `pipeline.Run()` function. If the process crashed mid-phase, the run was lost; if a phase failed, recovery meant manually reading a `runs` table and figuring out which match IDs were already in `matches`. Retry logic was bespoke per phase.

Temporal hands you these for free:

- **Persistent state**: the workflow's current step + intermediate results are stored in Temporal's database, surviving worker crashes.
- **Retry policies**: each Activity declares its retry shape (initial interval, multiplier, max attempts). Failures retry automatically.
- **Cancellation + heartbeats**: long-running activities can heartbeat progress; cancellation propagates cleanly.
- **Visibility**: the Temporal Web UI shows every step's input + output + duration.
- **Scheduling**: cron-style schedules with idempotent upserts; Temporal owns "what should run when."

The cost is a new piece of infra (Temporal server + its own Postgres). Locally that's just another container in `make dev`.

## Temporal vocabulary

You need exactly four concepts:

| Term | Meaning | Lives in |
|---|---|---|
| **Workflow** | A deterministic Go function describing the orchestration. *Cannot* do I/O directly. | `apps/worker/internal/workflow/` |
| **Activity** | A Go function that *does* I/O. Idempotent-ish. Called by workflows. | `apps/worker/internal/activity/` |
| **Task queue** | A named channel where workflow / activity tasks land for workers to poll. We use `crawl-{region}` so KR vs NA1 can scale independently. | code constants |
| **Schedule** | A cron expression + workflow start request. Temporal's replacement for `cron` + `at`. | `apps/worker/internal/schedule/` |

## Current code map

The crawler code is now owned by the worker stack under `apps/worker`.
Shared Riot API code lives under `packages/riotapi`; `apps/` and
`packages/` must not import archived top-level legacy packages.

| Package | Role |
|---|---|
| `apps/worker/cmd/worker` | process entry point; loads worker config, builds runtime, registers Temporal workers |
| `apps/worker/internal/config` | the single worker config model: Temporal, logging, Riot regions, database DSN, schedules, run profiles |
| `apps/worker/internal/crawlerconfig` | shared crawler schema types used by the unified config and `RunState` adapter |
| `packages/riotapi` | Riot/CDragon HTTP client, DTOs, rate limiting |
| `apps/worker/internal/storage` | worker-owned DB repository used by crawl phases and run bookkeeping |
| `apps/worker/internal/crawler` | copied phase algorithms and `RunState` helpers used by activities |
| `apps/worker/internal/activity/crawl` | Temporal Activity wrappers and phase-specific inputs/outputs |
| `apps/worker/internal/workflow/crawl` | deterministic `CrawlRegionWorkflow` orchestration |
| `apps/worker/internal/schedule` | converts crawler config schedules into Temporal Schedules |

The no-legacy-import guard is:

```bash
make check-no-legacy
```

It fails if any code under `apps/` or `packages/` imports the old `internal/crawler`, `internal/storage`, `internal/config`, `internal/riotapi`, `internal/server`, or `cmd/crawl` packages.

The mental model:

```
   Schedule fires            Workflow runs (deterministic)
        ▼                          ▼
   "start CrawlRegion(KR)"    "first call CreateRun,
                               then Phase0VersionSync,
                               then PinRunVersion,
                               then Phase1RankSnapshot,
                               ..."
                                    │
                                    ▼ each step
                              Activity executed (I/O allowed)
                                    │
                                    ▼ result
                              Workflow continues with that result
```

The workflow is *replayable*. If a worker dies after Phase 1 succeeded but before Phase 2 was scheduled, Temporal replays the workflow on a fresh worker: CreateRun returns the same Run ID from history (no DB row created twice), Phase0/Phase1 return their cached outputs, and execution resumes at "schedule Phase 2." That replay is why workflows must be deterministic — no `time.Now()`, no `rand`, no maps with iteration-order dependencies, no goroutines outside `workflow.Go`.

## Read the code, smallest-to-biggest

### 4a · The boot file

```bash
cat apps/worker/cmd/worker/main.go | head -80
```

`main()` does, in order:

1. Load config (viper).
2. Build a structured logger.
3. Build the `Runtime` from that same config (worker storage + Riot clients per region).
4. Dial the Temporal server.
5. For each region, register a worker on `crawl-{region}`.
6. Register `CrawlRegionWorkflow` + all activities.
7. Build the schedule plan from config and upsert into Temporal.
8. Wait for `SIGINT`, then graceful shutdown.

`Runtime` is the dependency-injection bag: parsed config, worker storage, and per-region Riot clients.

### 4b · One activity, end-to-end

Phase 0 is the simplest:

```bash
cat apps/worker/internal/activity/crawl/phase0.go
```

Things to notice:

- The function is a method on `*Activities`. That's the registration pattern for Temporal: `worker.RegisterActivity(&Activities{...})`.
- It calls `activity.RecordHeartbeat(ctx, ...)` periodically. That's how Temporal knows "still alive, don't time out."
- It returns a typed output (`Phase0Output` with `ResolvedVersion`, `UpsertedCount`). That output becomes available to subsequent activities via the workflow's history.

Now look at Phase 1 — slightly more complex:

```bash
cat apps/worker/internal/activity/crawl/phase1.go
```

- It loops over (queue, tier) pairs.
- For each combination, it calls the Riot client.
- It writes `players` + `player_rank_snapshots`.
- If Riot returns 401, the function returns an error. Temporal sees the error and retries per the workflow's `phase1Opts` policy.

### 4c · The workflow itself

```bash
cat apps/worker/internal/workflow/crawl/workflow.go | head -100
```

This is the orchestration. Read the variable declarations first:

- `bookkeepingOpts` — short timeouts, fast retries (for `CreateRun`, `PinRunVersion`, `CompleteRun`).
- `phase0Opts` — modest timeouts (CDragon is fast).
- `phase1Opts` — longer timeouts, exponential backoff `5s → 10s → 20s → 40s → 80s`, max 5 attempts.
- `phase2Opts` through `phase55Opts` — phase-specific budgets.

Then the function body:

```go
func CrawlRegionWorkflow(ctx workflow.Context, in CrawlRegionInput) (CrawlRegionOutput, error) {
    profile := in.Profile
    if in.ProfileName != "" {
        profile, err = resolveProfile(ctx, in.ProfileName)
        ...
    }
    runID, err := createRun(ctx, profile)
    ...
    p0, err := runPhase0(ctx)
    ...
    err = pinRunVersion(ctx, runID, p0.ResolvedVersion)
    ...
    // and so on
}
```

If any step returns an error, the workflow runs `FailRun` (via a disconnected context so cancellation doesn't bleed in), then returns. The `runs` table row is stamped `status='failed'`.

🛠️ **Exercise**: find the `runPhase2` dispatcher and read it. There are two modes:

- `execution: "sequential"` — one activity call with all tiers inline.
- `execution: "pipeline"` — process each configured target tier in order. For one tier, run Phase 2 through Phase 5.5 before moving to the next tier.

Pipeline mode is tier-first, not phase-first: `CHALLENGER` can become usable after its Phase 2→5.5 chain completes, without waiting for `GRANDMASTER` or `MASTER` to finish Phase 2. Later phases still consume pending rows by region + version, so this is practical tier prioritisation rather than strict per-tier match isolation.

### 4d · The schedule registration

```bash
cat apps/worker/internal/schedule/schedule.go | head -60
```

`BuildPlan(cfg)` reads the unified config's `Schedule[]` array, resolves each profile name to its region, and constructs a `Plan` per cron entry. `Upsert(ctx, c, plans)` calls `sc.Create` per plan; if the schedule already exists, the SDK returns `temporal.ErrScheduleAlreadyRunning`, and we fall through to `h.Update(...)` so the cron expression / start payload can be edited in the worker config without manual cleanup.

The schedule ID convention: `gogg-crawl-{profile}`. That's the string you'll see in the Temporal UI.

## Watch it run

You need a valid Riot API key in `deploy/secrets/dev.enc.yaml` (request one at <https://developer.riotgames.com>; the key is a 38-char `RGAPI-...` string). With sops + age set up:

```bash
sops -d deploy/secrets/dev.enc.yaml | grep RIOT_API_KEY
```

That should print your key. If you get a placeholder, edit:

```bash
sops deploy/secrets/dev.enc.yaml   # opens an editor on the decrypted content
# add: riot_api_key: RGAPI-your-key-here
# save & quit
```

Now start the worker:

```bash
make run-worker
```

In the logs, look for `schedule_created` or `schedule_updated`. Open the Temporal UI at <http://localhost:8233>, click **Schedules** in the left nav — you should see `gogg-crawl-daily_kr`.

To trigger a workflow immediately (without waiting for the cron to fire), use the Temporal CLI from your host. The host loopback trick is in `docs/manual-verification.md`:

```bash
docker exec gogg-dev-temporal tctl --address temporal:7233 workflow start \
    --taskqueue crawl-na1 \
    --workflow_type CrawlRegionWorkflow \
    --workflow_id manual-na1-$(date +%s) \
    --input '{"profile_name":"daily_na"}'
```

Switch to the **Workflows** tab in the UI. You'll see your run starting. Click into it. You can:

- See each activity's input + output in the event history.
- Watch the timer between attempts when a retry happens.
- Cancel it mid-run with the "Terminate" button.

🛠️ **Exercise**: with a valid key, let the workflow finish Phase 0 + Phase 1, then look at the `runs` and `player_rank_snapshots` tables:

```bash
psql "$DEV_DSN" -c 'SELECT id, status, profile, started_at, current_phase FROM runs ORDER BY id DESC LIMIT 5;'
psql "$DEV_DSN" -c 'SELECT count(*) FROM player_rank_snapshots WHERE source = \'phase1\';'
```

You should see the new run + several hundred rank snapshots.

🛠️ **Exercise (no key required)**: trigger the workflow with `ProfileName: "daily_kr"`. Phase 1 will fail with HTTP 401 (no auth). Watch the Temporal UI: you'll see 5 attempt rows with timestamps 5s, 10s, 20s, 40s apart. After the fifth attempt, the workflow runs `FailRun` and ends. `runs.status` will be `'failed'`.

That single observation — the workflow keeping going through retries without you writing a single line of `time.Sleep` or `if err { retry }` — is why Temporal exists.

## The synthetic RunState adapter

Open `apps/worker/internal/activity/crawl/synthstate.go`:

```bash
cat apps/worker/internal/activity/crawl/synthstate.go
```

The copied phase algorithms in `apps/worker/internal/crawler/phaseN/phase.go` still expect a `*crawler.RunState`. The workflow owns run lifecycle now, so activities do not use the old runner. Instead, `synthState()` constructs a small `*crawler.RunState` on the fly from the activity inputs, calls the phase algorithm, and discards the synthetic state.

This is an adapter pattern: the worker owns the code and dependencies, but it preserves the proven phase algorithms while Temporal owns orchestration, retries, and cancellation.

## Determinism gotchas

If you ever write a new workflow (Phase E adds `EnrichSummonerWorkflow`), the determinism rules:

- ❌ `time.Now()` → ✅ `workflow.Now(ctx)`
- ❌ `rand` → ✅ `workflow.SideEffect(ctx, func() any { return rand.Int() })`
- ❌ `go func() { ... }` → ✅ `workflow.Go(ctx, func(ctx workflow.Context) { ... })`
- ❌ unsorted `range mapVar` → ✅ sort the keys first
- ❌ direct DB/HTTP calls → ✅ wrap in an Activity and call it

Violating any of these means a replay will diverge from history, and Temporal will refuse to continue. The error message ("non-determinism in workflow") points at the offending step.

## Up next

[Chapter 05 — API backend](./05-api-backend.md) covers the other binary: `gogg-api`. You'll learn how an HTTP request walks through chi middleware → service → sqlc → Postgres, and what the gqlgen GraphQL surface adds on top.
