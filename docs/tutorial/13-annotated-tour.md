# Chapter 13 · Annotated code tour — six classic snippets

> Goal: by the end of this chapter you've seen line-by-line walkthroughs of six load-bearing pieces of the codebase. Each snippet is annotated with what's happening + why. Use these as templates when you write similar code yourself.

The six tours:

1. [main.go DI + graceful shutdown](#tour-1--maingo-the-dependency-graph) — how dependencies are wired and how the binary shuts down cleanly
2. [The Cache interface + singleflight](#tour-2--cache-interface--singleflight) — the read-through pattern with herd protection
3. [Temporal workflow + retry policies](#tour-3--temporal-workflow--phase-aware-retry-policies) — declarative retry per phase, with the orchestration in pure Go
4. [Useful Effect for orchestrating React state](#tour-4--reactgrade-orchestration-via-useeffect) — how RankingsPage coordinates filter / query / animation
5. [Custom hook for a 4-phase state machine](#tour-5--usefadetransition--a-state-machine-as-a-custom-hook) — the fade animation owned by a hook, not the page
6. [TanStack Query + codegen bridging](#tour-6--codegen--tanstack-query-bridging) — how generated hooks become app code

You can read these in any order, but the listed order moves from backend to frontend, simple to complex.

---

## Tour 1 — main.go: the dependency graph

File: `apps/api/cmd/api/main.go`. The first ~120 lines are the dependency-injection wiring of the whole API binary. This is the single file where every dependency is constructed and every shutdown is ordered.

```go
func main() {
    if err := run(); err != nil {
        fmt.Fprintf(os.Stderr, "gogg-api: %v\n", err)
        os.Exit(1)
    }
}
```

> 💡 **The `main` thinness pattern.** Real Go binaries usually have a 5-line `main()` that just calls `run()`. Why? Because `run()` returns `error`, which means it's *testable* — you can write a unit test that calls `run()` with synthetic args and asserts on the result. `main()` can't be tested. Keep the untestable shell tiny.

```go
func run() error {
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("load config: %w", err)
    }
```

> Step 1: load config. The `%w` wraps the underlying error so the call site sees both "load config: " and the original message. `errors.Is/As` continue to work through the wrap.

```go
    logger := newLogger(cfg.Logging)
    slog.SetDefault(logger)
    logger.Info("starting",
        "version", version, "commit", commit, "build_date", buildDate,
        "port", cfg.API.Port, "log_level", cfg.Logging.Level,
    )
```

> Step 2: build the logger. `slog.SetDefault` makes it the package default so any code that grabs `slog.Default()` (libraries we don't control) gets our configured logger.
>
> Notice the **structured logging shape**: the first arg is an event name (`"starting"` — snake_case, noun-form), the rest are key/value pairs. Don't `fmt.Sprintf` the values into the event name — keep them as attrs so queries and dashboards can filter.

```go
    rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()
```

> Step 3: the **root context** for the binary. `signal.NotifyContext` returns a context that's cancelled the moment SIGINT (Ctrl+C) or SIGTERM (kubectl delete pod) arrives. Every downstream context derives from this one. When the signal fires, every in-flight HTTP request, every DB query, every Redis call sees its context cancelled.
>
> `defer stop()` releases the signal handler when `run()` returns. Pair every `NotifyContext` with a `defer stop()`. Otherwise the process keeps holding signal handlers around.

```go
    dbCtx, cancel := context.WithTimeout(rootCtx, 10*time.Second)
    defer cancel()
    pool, err := connectDB(dbCtx, cfg.Database)
    if err != nil {
        return fmt.Errorf("connect db: %w", err)
    }
    defer pool.Close()
```

> Step 4: DB. Note **two patterns** in 5 lines:
>
> 1. **Bounded startup**: `WithTimeout(rootCtx, 10s)` — the DB must connect within 10 seconds or we bail. A naked `connectDB(rootCtx, ...)` could hang for 60s of TCP retries.
> 2. **`defer pool.Close()`**: register cleanup immediately after acquiring a resource. If `run()` returns later (success or error), the pool gets closed. This is how Go does "RAII" — without destructors, you `defer` the cleanup as soon as you've created the thing.

```go
    var redisClient *cache.Redis
    if cfg.Redis.URL != "" {
        redisClient, err = cache.NewRedis(cfg.Redis.URL)
        if err != nil {
            return fmt.Errorf("init redis: %w", err)
        }
        defer func() { _ = redisClient.Close() }()
```

> Step 5: Redis. Same pattern. The conditional (`if cfg.Redis.URL != ""`) is how we make Redis **optional in dev** — if not configured, every request goes straight to PG. In prod it's always there.
>
> The `defer func() { _ = redisClient.Close() }()` instead of `defer redisClient.Close()` is because `.Close()` returns an error, which `defer` would discard silently. The explicit `_ =` documents "we're intentionally ignoring this error" — a linter would flag the bare form.

Skipping ahead — the same pattern repeats for sqlc queries, cache wrapper, services, auth provider, GraphQL root, REST routes, prom registry.

```go
    handler := buildRouter(cfg, logger, pool, redisClient)
    // ...
    srv := &http.Server{
        Addr:    fmt.Sprintf(":%d", cfg.API.Port),
        Handler: handler,
        // ...
    }

    // Start the server in a goroutine; main flow waits for shutdown signal.
    go func() {
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            logger.Error("listen_failed", "err", err)
        }
    }()
    logger.Info("http_server_started", "addr", srv.Addr)

    <-rootCtx.Done()
    logger.Info("shutdown_signal")

    shutdownCtx, sCancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer sCancel()
    if err := srv.Shutdown(shutdownCtx); err != nil {
        logger.Error("shutdown_failed", "err", err)
    }
    logger.Info("http_server_stopped")
    return nil
}
```

> **Graceful shutdown.** The pattern:
>
> 1. Start the server in a goroutine. Don't block `main` on it.
> 2. Block `main` on the root context being done (i.e. on the signal arriving).
> 3. When the signal fires, derive a fresh context with timeout (because rootCtx is already cancelled). 15 seconds is "drain time" — long enough for in-flight requests to finish, short enough that we don't hang forever if a request is stuck.
> 4. Call `srv.Shutdown(ctx)`. It stops accepting new connections and waits for existing ones to drain.
> 5. Log + return. All deferred cleanups run on the way out (Redis close, DB pool close, signal stop).
>
> `errors.Is(err, http.ErrServerClosed)` filters the "normal shutdown" error. `ListenAndServe` returns this error when `Shutdown` is called; it's not a real failure.

### What this teaches

- DI is **explicit, in main, top-to-bottom**. No magic.
- Every resource is paired with a `defer` cleanup the same line it's acquired.
- The root context owns the binary's lifetime. Everything derives from it.
- Graceful shutdown is a four-line pattern: signal → context → server.Shutdown → return.

Copy this main.go shape into any Go binary you write.

---

## Tour 2 — Cache interface + singleflight

File: `apps/api/internal/cache/redis.go`. This is a textbook example of "narrow interface + concrete implementation + dedupe pattern."

```go
type Cache interface {
    GetJSON(ctx context.Context, key string, dst any) error
    SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
    Delete(ctx context.Context, keys ...string) error
    Ping(ctx context.Context) error
}

var ErrMiss = errors.New("cache: miss")
```

> The **Cache interface** is 4 methods. That's it. Bigger Redis features (pipelines, scripts, pubsub) stay internal — service code never touches them. Why narrow? Because every method in the interface is a method services must mock in tests. Bigger = more mock setup.
>
> `ErrMiss` is a **sentinel error**. Callers do `errors.Is(err, cache.ErrMiss)` to distinguish "key wasn't there" from "Redis is broken." Two failure modes, two different responses (miss → run the loader; broken → log + bypass).

```go
type Redis struct {
    client *redis.Client
    sf     singleflight.Group
}
```

> **The struct.** Two fields: the underlying go-redis client + a singleflight group. We'll see what singleflight does in a moment.

```go
func (r *Redis) GetJSON(ctx context.Context, key string, dst any) error {
    b, err := r.client.Get(ctx, key).Bytes()
    if err != nil {
        if errors.Is(err, redis.Nil) {
            return ErrMiss
        }
        return fmt.Errorf("redis get %q: %w", key, err)
    }
    if err := json.Unmarshal(b, dst); err != nil {
        return fmt.Errorf("decode cache %q: %w", key, err)
    }
    return nil
}
```

> **GetJSON breakdown:**
>
> - `r.client.Get(ctx, key).Bytes()` — go-redis returns a chain. The terminal `.Bytes()` returns `[]byte, error`.
> - `errors.Is(err, redis.Nil)` — go-redis returns the sentinel `redis.Nil` for cache miss. Translate to *our* sentinel `ErrMiss` so service code doesn't need to import go-redis.
> - Real errors get wrapped with the key (for log/debugging context) and `%w` (so `errors.Is/As` continues to work upstream).
> - On success, unmarshal into the caller-provided `dst`. The `any` (interface{}) parameter lets one function serve every value shape.

Now the singleflight magic. Look further in the file (or grep `GetOrLoad`):

```go
func GetOrLoad[T any](
    c Cache, ctx context.Context, key string, ttl time.Duration,
    loader func() (T, error),
) (T, error) {
    var zero T
    var got T
    err := c.GetJSON(ctx, key, &got)
    if err == nil {
        return got, nil
    }
    if !errors.Is(err, ErrMiss) {
        // Real error - log + fall through to loader (don't fail the request).
        slog.Default().Warn("cache_get_failed", "key", key, "err", err)
    }
    // Cache miss or error — load.
    value, err := loader()
    if err != nil {
        return zero, err
    }
    if setErr := c.SetJSON(ctx, key, value, ttl); setErr != nil {
        slog.Default().Warn("cache_set_failed", "key", key, "err", setErr)
    }
    return value, nil
}
```

> **GetOrLoad** is a Go generic (note the `[T any]`). It's the read-through pattern:
>
> 1. Try cache. Hit → return.
> 2. Miss or error → call loader. Loader returns the real value (via SQL).
> 3. Write the result back to cache (best-effort; ignore set errors).
> 4. Return the value.
>
> **Key insight: cache failures don't fail the request.** A broken Redis logs a warning + falls through to PG. The user gets a slightly slower response, not an error.

Inside `Redis`, there's also `GetOrLoadSF` that uses singleflight:

```go
func (r *Redis) GetOrLoadSF(ctx context.Context, key string, ...) (...) {
    v, err, _ := r.sf.Do(key, func() (any, error) {
        // ... GetJSON-then-loader-then-SetJSON
    })
}
```

> `singleflight.Group.Do(key, fn)` says: "If another goroutine is currently calling `fn` for this `key`, wait for *that* result instead of calling `fn` again." This prevents **thundering herd** — 100 concurrent users hitting the rankings page on a cache miss don't fire 100 PG queries; they fire 1 and all share the answer.

### What this teaches

- Narrow interfaces. Two methods are better than ten if two is enough.
- Sentinel errors for known states (`ErrMiss`).
- Graceful degradation: cache errors → log + skip, never fail the request.
- Generics let you write read-through once and use it for any value type.
- Singleflight is the standard "dedupe concurrent calls" tool. Use it for any expensive idempotent operation that takes a stable key.

---

## Tour 3 — Temporal workflow + phase-aware retry policies

File: `apps/worker/internal/workflow/crawl/workflow.go`. Lines ~50–120.

```go
var (
    bookkeepingOpts = workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            InitialInterval:    time.Second,
            BackoffCoefficient: 2.0,
            MaximumInterval:    30 * time.Second,
            MaximumAttempts:    5,
        },
    }
```

> **bookkeepingOpts** — for CreateRun, PinRunVersion, CompleteRun, FailRun. These are short DB writes; we expect them to take <100ms. The retry policy is conservative: 5 attempts, exponential backoff starting at 1s. If the DB is so unhealthy that 5 retries fail, the workflow itself fails — no point pretending success.

```go
    phase1Opts = workflow.ActivityOptions{
        StartToCloseTimeout: time.Hour,
        HeartbeatTimeout:    2 * time.Minute,
        RetryPolicy: &temporal.RetryPolicy{
            InitialInterval:    5 * time.Second,
            BackoffCoefficient: 2.0,
            MaximumInterval:    2 * time.Minute,
            MaximumAttempts:    5,
        },
    }
```

> **phase1Opts** — for the Phase 1 rank snapshot activity. Very different:
>
> - `StartToCloseTimeout: 1 hour` — the activity can run up to an hour before Temporal forcibly fails it. A KR daily run walks 3 tiers × ~1000 players each + per-player lookups; an hour is generous.
> - `HeartbeatTimeout: 2 minutes` — if the activity doesn't heartbeat within 2 minutes, Temporal assumes the worker died and restarts it. The activity heartbeats every 25 players, so 2 minutes of silence = real problem.
> - `InitialInterval: 5s` — first retry waits 5 seconds. Then 10, 20, 40, 80 (capped at 120s). Five attempts total. This is what makes "Phase 1 fails with HTTP 401" play out over ~155 seconds before the workflow gives up.

Why the variation per phase? Because **failure modes differ per phase**. A DB write that times out at 30s is broken. A Riot API call that takes 30s is normal — it's paginating 200 players. One retry policy doesn't fit both.

Now the orchestration:

```go
func CrawlRegionWorkflow(ctx workflow.Context, in CrawlRegionInput) (CrawlRegionOutput, error) {
    log := workflow.GetLogger(ctx)
    profile := in.Profile
    if in.ProfileName != "" {
        var err error
        profile, err = resolveProfile(ctx, in.ProfileName)
        if err != nil { return CrawlRegionOutput{}, err }
    }
    log.Info("crawl_region_starting", "profile_name", in.ProfileName, "region", profile.Region)
```

> `workflow.GetLogger(ctx)` is the Temporal-aware logger. It writes log lines that **only appear during real execution**, not during replay. (Replay happens when Temporal recovers a workflow from history — you don't want every log to fire 1000 times during recovery.)

```go
    runID, err := createRun(ctx, profile)
    if err != nil {
        return CrawlRegionOutput{}, err
    }
    log.Info("crawl_region_run_created", "run_id", runID, "region", profile.Region)
```

> `createRun(ctx, profile)` is a helper that calls `workflow.ExecuteActivity(ctx, acts.CreateRun, ...)`. **Every Riot call, every DB write, every CDragon fetch — wrapped in an Activity.** The workflow itself does no I/O.

```go
    out := CrawlRegionOutput{ RunID: runID, StartedAt: workflow.Now(ctx) }

    if err := runPhases(ctx, runID, profile, &out); err != nil {
        // Workflow failed somewhere. Stamp the run row 'failed' via a
        // disconnected context so cancellation doesn't bleed in.
        disconnected, _ := workflow.NewDisconnectedContext(ctx)
        _ = workflow.ExecuteActivity(disconnected, acts.FailRun,
            crawlact.FailRunInput{RunID: runID, Reason: err.Error()}).
            Get(disconnected, nil)
        return out, err
    }
```

> Two important constructs:
>
> 1. **`workflow.Now(ctx)`** instead of `time.Now()`. Workflows must be deterministic; if a workflow replays, the original "now" must be recoverable from history. `workflow.Now` returns the deterministic timestamp.
> 2. **`workflow.NewDisconnectedContext(ctx)`** for FailRun. When the user cancels a workflow, the regular `ctx` is cancelled — which would prevent FailRun from running and the `runs` row from being stamped. The disconnected context survives cancellation, so the audit trail completes.

```go
    if err := completeRun(ctx, runID); err != nil {
        return out, err
    }
    log.Info("crawl_region_completed", "run_id", runID, "resolved_version", out.ResolvedVersion)
    return out, nil
}
```

> Happy path: stamp the row complete, log success, return the output. The output struct goes into Temporal's event history; subsequent Temporal SDK queries can read it.

Inside `runPhases`, look at the per-tier fan-out:

```go
if profile.Execution == "pipeline" {
    futures := make([]workflow.Future, 0, len(profile.TargetTiers))
    for _, tier := range profile.TargetTiers {
        in := crawlact.Phase2Input{
            RunID:   runID,
            Profile: profile,
            Tiers:   []string{tier},
        }
        f := workflow.ExecuteActivity(
            workflow.WithActivityOptions(ctx, phase2Opts),
            (*crawlact.Activities)(nil).Phase2MatchIDCollection,
            in,
        )
        futures = append(futures, f)
    }
    for _, f := range futures {
        var p2 crawlact.Phase2Output
        if err := f.Get(ctx, &p2); err != nil { return err }
        out.Phase2Outputs = append(out.Phase2Outputs, p2)
    }
}
```

> **Pipeline mode parallelism.** We ExecuteActivity once per tier (the schedule is non-blocking — it returns a `workflow.Future`). Then we collect them via `f.Get(ctx, &p2)` — that blocks until each tier's activity finishes. N tiers run in parallel; the workflow waits for the slowest.
>
> The legacy "PipelineStrategy" was per-tier *serial*. This Temporal version is the per-tier *parallel* win for free, just by using futures instead of synchronous calls.

### What this teaches

- Each phase declares its own retry policy + timeouts. One size doesn't fit all.
- Workflows are pure orchestration. Every I/O step is an Activity.
- Use `workflow.Now / workflow.NewDisconnectedContext / workflow.GetLogger` instead of stdlib equivalents — they preserve determinism + replay semantics.
- Parallel fan-out is `futures := []; for { futures = append(futures, ExecuteActivity(...)) }; for { f.Get() }`.

---

## Tour 4 — React-grade orchestration via useEffect

File: `apps/web/src/features/rankings/RankingsPage.tsx`.

```tsx
const fade = useFadeTransition();
const filters = useRankingsFilters({
    onBeforeCommit: () => {
        fade.beginExit();
        setLimit(PAGE_SIZE);
    },
});

const [limit, setLimit] = useState(PAGE_SIZE);
```

> Three pieces of state:
>
> - `fade` — owns the animation state machine.
> - `filters` — owns the selected vs committed filter values.
> - `limit` — owns the current pagination page size.
>
> The `onBeforeCommit` callback bridges them: **when a filter changes, also trigger the fade and reset the limit.** This is "lifting up" — the filters hook doesn't know about animation; the page wires the two together.

```tsx
useEffect(() => {
    if (fade.phase === "hidden") {
        filters.commit();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
}, [fade.phase]);
```

> **Effect 1: post-fade-out commit.**
>
> When `fade.phase` transitions to `"hidden"` (after the fade-out timer fires), we commit the new filter. That triggers `useRankingsQuery` to re-fire with the new variables.
>
> The eslint-disable is for `filters.commit` not being in deps. `filters.commit` is recreated every render (it's a closure over fresh state), so including it would re-fire the effect on every render. The pragmatic choice: trust the dependency, and document it. Chunk 5's reducer refactor will eliminate this.

```tsx
const versionsQuery = useVersionsQuery();
const regionsQuery = useRegionsQuery();

const rankings = useRankingsQuery({
    filters: filters.committed,
    limit,
    enabled: fade.phase === "shown" || fade.phase === "fading-in",
});
```

> **The data layer.** Three TanStack Query subscriptions. `useRankingsQuery` is the only one with conditional enabling — disabled during `"fading-out"` + `"hidden"` so we don't fire a request before the fade is in the right state.
>
> Actually, look at the `enabled` more carefully — it's `"shown" || "fading-in"`. We allow the query during fade-in so that if a user changes a filter twice quickly, the second change's `beginExit` triggers a fresh query and we don't lose the request.

```tsx
useEffect(() => {
    if (fade.phase !== "hidden") return;
    if (rankings.isLoading || rankings.isFetching) return;
    fade.beginEnter();
}, [fade, rankings.isLoading, rankings.isFetching]);
```

> **Effect 2: post-load fade-in.**
>
> When we're in `"hidden"` AND the query is no longer fetching, the new data has landed. Call `beginEnter` to start the fade-in.
>
> The triple guard (`phase !== "hidden"`, `isLoading`, `isFetching`) prevents firing in any other state. `isFetching` covers the case where TanStack Query has data but is refetching in the background.

```tsx
const onLoadMore = useCallback(() => {
    if (rankings.isFetching || rankings.isError) return;
    if (rankings.items.length < limit) return;
    setLimit((prev) => prev + PAGE_SIZE);
}, [rankings.isFetching, rankings.isError, rankings.items.length, limit]);

const sentinelRef = useInfiniteScroll<HTMLDivElement>({
    onLoadMore,
    enabled: fade.phase === "shown" && rankings.items.length >= limit,
});
```

> **Pagination.** `onLoadMore` is memoized so the IntersectionObserver effect inside `useInfiniteScroll` doesn't re-attach on every render.
>
> Notice the `setLimit((prev) => prev + PAGE_SIZE)` — the **functional form** of setState. Avoids the closure-over-stale-limit bug if the user scrolls fast.
>
> The observer is `enabled` only when the table is fully shown (no fade in progress) and there's likely more data (items.length >= limit).

```tsx
return (
    <section className="space-y-6">
        {/* header */}
        <RankingsFilters {...} />
        <div
            ref={fade.viewportRef}
            style={lockedStyle}
            className={cn(
                "transition-opacity duration-200 ease-out-soft",
                fade.phase === "fading-out" && "opacity-0",
                fade.phase === "hidden" && "opacity-0",
                fade.phase === "fading-in" && "opacity-100",
                fade.phase === "shown" && "opacity-100",
            )}
        >
            {/* table or skeleton */}
        </div>
    </section>
);
```

> The JSX is short. All the complexity is in the hooks + effects. **This is the pattern**: declarative JSX, imperative logic isolated to effects, reusable behavior in custom hooks.

### What this teaches

- Lift state up + bridge via callbacks. `onBeforeCommit` is the glue.
- One effect per "when X happens, do Y" relationship. Don't write giant effects with multiple if-branches.
- Use functional setState (`setLimit(prev => prev + N)`) when the new value depends on the old.
- Memoize callbacks (`useCallback`) that are passed to effect deps to prevent re-runs.

---

## Tour 5 — useFadeTransition: a state machine as a custom hook

Full file (already short): `apps/web/src/features/rankings/hooks/useFadeTransition.ts`. Re-read it now with the orchestration context from Tour 4 in mind.

```tsx
export type FadeTransitionPhase =
    | "shown"
    | "fading-out"
    | "hidden"
    | "fading-in";
```

> **Discriminated union as state.** Four named phases. The TS compiler knows there are exactly four; in JSX you can have `if (phase === "shown")` and TS narrows accordingly.

```tsx
const [phase, setPhase] = useState<FadeTransitionPhase>("shown");
const [lockedHeight, setLockedHeight] = useState<number | null>(null);

const viewportRef = useRef<HTMLDivElement>(null);
const outTimerRef = useRef<number | null>(null);
const inTimerRef = useRef<number | null>(null);
```

> Two `useState` + three `useRef`. Why mix?
>
> - `phase`, `lockedHeight` — when they change, the consumer re-renders. They're **state**.
> - `viewportRef` — points at a DOM node. Not a state value; setting `current` doesn't re-render. **DOM ref**.
> - Timer refs — these change between renders but the consumer never reads them (they're internal). **Mutable ref for non-rendered values**.

```tsx
const clearTimers = useCallback(() => {
    if (outTimerRef.current !== null) {
        window.clearTimeout(outTimerRef.current);
        outTimerRef.current = null;
    }
    if (inTimerRef.current !== null) {
        window.clearTimeout(inTimerRef.current);
        inTimerRef.current = null;
    }
}, []);

useEffect(() => clearTimers, [clearTimers]);
```

> **Cleanup pattern.** The effect's return value is the cleanup. Here we return `clearTimers` directly — equivalent to `useEffect(() => () => clearTimers(), [...])` but tighter. When the component unmounts, the cleanup fires; any pending timers are cancelled.
>
> Without this, a user navigating away during a fade would leave timers running. They'd fire 220ms later, call `setPhase` on an unmounted component, and React would warn.

```tsx
const beginExit = useCallback(() => {
    const h = viewportRef.current?.offsetHeight ?? 0;
    if (h > 0) setLockedHeight(h);

    clearTimers();
    setPhase("fading-out");
    outTimerRef.current = window.setTimeout(() => {
        setPhase("hidden");
        outTimerRef.current = null;
    }, fadeOutMs);
}, [clearTimers, fadeOutMs]);
```

> **beginExit:**
>
> 1. Capture the viewport's current height (lock it so the page doesn't reflow during the swap).
> 2. Clear any pending timers (back-to-back filter changes shouldn't pile up).
> 3. Move to `"fading-out"`. CSS handles the actual opacity animation.
> 4. Schedule a timer to transition to `"hidden"` once the fade is done. Save the timer handle to the ref for later cleanup.

```tsx
const beginEnter = useCallback(() => {
    setPhase((current) => {
        if (current !== "hidden") return current;

        if (inTimerRef.current !== null) window.clearTimeout(inTimerRef.current);
        inTimerRef.current = window.setTimeout(() => {
            setPhase("shown");
            setLockedHeight(null);
            inTimerRef.current = null;
        }, fadeInMs);

        return "fading-in";
    });
}, [fadeInMs]);
```

> **beginEnter** uses the **functional setState** form to read the current phase atomically. If we're not in `"hidden"`, this is a no-op (the consumer called us at the wrong time — guard against it).
>
> If we are in `"hidden"`, schedule the second timer, return `"fading-in"`. After the timer fires, we land back at `"shown"` and unlock the height.

```tsx
return {
    phase,
    lockedHeight,
    viewportRef,
    beginExit,
    beginEnter,
    isTransitioning: phase !== "shown",
};
```

> The hook's return shape. Notice `isTransitioning` is **derived from phase** — never store derived state separately. It's computed on every render; that's cheap and prevents drift.

### What this teaches

- Custom hooks make stateful logic reusable + testable without coupling to a specific UI.
- Mix `useState` (re-rendering values) + `useRef` (internal mutable storage) intentionally.
- Always pair `setTimeout` with a cleanup. Hold the handle in a ref.
- Use the functional setState form when the new value depends on the old.
- Return derived values (`isTransitioning`) instead of storing them.

---

## Tour 6 — Codegen + TanStack Query bridging

File: `apps/web/src/features/rankings/hooks/useRankingsQuery.ts`.

```tsx
import { useMemo } from "react";
import { keepPreviousData } from "@tanstack/react-query";

import {
    type ChampionRankingsFilter,
    type ChampionRankingsQuery,
    useChampionRankingsQuery,
} from "@shared/api";

import {
    type RankingsFiltersState,
    mapToTierGroup,
} from "./useRankingsFilters";

const FIXED_MIN_GAMES = 20;
```

> The imports tell the story:
>
> - `ChampionRankingsFilter` + `ChampionRankingsQuery` — types from **graphql-codegen**.
> - `useChampionRankingsQuery` — the **codegen'd React hook** that posts to `/graphql`.
> - `RankingsFiltersState` + `mapToTierGroup` — domain types from our own hook.
>
> The hook we're writing is **the bridge** between UI vocabulary and the GraphQL contract.

```tsx
export interface UseRankingsQueryOptions {
    filters: RankingsFiltersState;
    limit: number;
    enabled?: boolean;
}

export interface UseRankingsQueryResult {
    items: ChampionRankingsQuery["championRankings"]["items"];
    totalMatches: number;
    resolvedVersion: string | null;
    isLoading: boolean;
    isFetching: boolean;
    isError: boolean;
    error: Error | null;
}
```

> Two interfaces: the input options + the output shape. The output is **flatter than** the GraphQL response — we lift `items`, `totalMatches`, `resolvedVersion` to the top level so consumers don't navigate `query.data.championRankings.items.X` every read.
>
> Notice the type `ChampionRankingsQuery["championRankings"]["items"]`. That's TypeScript **lookup types** — "the type of `items` inside `championRankings` inside `ChampionRankingsQuery`." Saves us from re-declaring the shape; if the GraphQL schema changes, this auto-updates.

```tsx
export function useRankingsQuery({
    filters,
    limit,
    enabled = true,
}: UseRankingsQueryOptions): UseRankingsQueryResult {
    const filter = useMemo<ChampionRankingsFilter>(
        () => ({
            position: filters.position || "",
            tierGroup: mapToTierGroup(filters.tier),
            region: filters.region,
            version: filters.version,
            minGames: FIXED_MIN_GAMES,
            limit,
        }),
        [filters, limit],
    );
```

> **The filter projection.** We construct the GraphQL `filter` object inside `useMemo` so it doesn't change identity on every render (which would cause TanStack Query to think the variables changed and refetch).
>
> The `mapToTierGroup` call converts the UI tier vocabulary (`""`, `"challenger"`, `"grandmaster_plus"`, ...) into the GraphQL enum (`"ALL"`, `"CHALLENGER"`, `"GRANDMASTER_PLUS"`, ...). The two vocabularies meet here.
>
> `FIXED_MIN_GAMES = 20` is the constant floor — the rankings page never shows champions with fewer than 20 games. Hardcoded for legacy parity; could be a profile setting later.

```tsx
    const query = useChampionRankingsQuery(
        { filter },
        {
            enabled,
            placeholderData: keepPreviousData,
        },
    );
```

> **The codegen'd hook call.** First arg: variables `{filter}`. Second arg: TanStack Query options.
>
> `placeholderData: keepPreviousData` is the key UX option — while a new query fetches, the previous result stays visible. That's why infinite scroll doesn't blank the table during the next-page fetch.

```tsx
    return {
        items: query.data?.championRankings.items ?? [],
        totalMatches: query.data?.championRankings.totalMatches ?? 0,
        resolvedVersion: query.data?.championRankings.resolvedVersion ?? null,
        isLoading: query.isLoading,
        isFetching: query.isFetching,
        isError: query.isError,
        error: query.error instanceof Error ? query.error : null,
    };
}
```

> The output projection. Each field has a sensible default for "before data lands." `items` defaults to `[]` so consumers can do `items.map(...)` without null checks. `totalMatches` defaults to `0` so the stats bar can render "0 matches" (or it'll hide via its `if (totalMatches <= 0) return null` check).
>
> The `error instanceof Error ? query.error : null` is type-narrowing — TanStack Query's `error` is `unknown` (because the rejected promise can be anything in JS), but for our type contract we narrow to `Error | null`. Anything weirder than an `Error` becomes `null` and the consumer can rely on the `isError` flag.

### What this teaches

- Generated hooks are the boundary; your own hooks are the **adapter** between UI types and the generated contract.
- Use TypeScript lookup types (`Type["field"]["subfield"]`) to derive types instead of re-declaring.
- `useMemo` for objects passed to TanStack Query so they keep identity across renders.
- `keepPreviousData` for "don't blank the screen during refetch."
- Always return sensible defaults from data hooks; let the consumer skip null checks.

---

## What to do with these

When you're about to write code that resembles any of these six shapes, **come back here and re-read the relevant tour first.** Copying the shape gives you a structurally-sound starting point; the comments in the tour explain why each piece exists so you can adapt judiciously.

Common cases:

- Writing a new Go binary? Tour 1.
- Adding a cached service method? Tour 2.
- Writing a new Temporal workflow? Tour 3.
- A new page that orchestrates multiple data + animation hooks? Tour 4 + 5.
- Adding a new GraphQL operation? Tour 6.

You now have a worked example for each.

## Up next

You've completed the tutorial proper. Loop back to [Chapter 09 — Next steps](./09-next-steps.md) for the roadmap into Phase E + F, the ADR + plan rereading suggestions, and the contribution workflow.
