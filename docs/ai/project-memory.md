# GOGG durable project memory

Last evidence review: 2026-08-28.

This file records stable project facts and proven lessons for future human and
agent sessions. It is not a task tracker. Validate any fact that may have gone
stale against code and tests before relying on it.

## Product and architecture

- GOGG is a production-oriented modular monolith for League of Legends
  champion statistics and summoner match history.
- KR and NA1 are the initial regions. The UI supports zh-CN and en-US.
- The runtime is split into a React/Vite frontend, a Go API, and Go Temporal
  workers, backed by PostgreSQL and Redis.
- The API owns synchronous product contracts. Temporal owns durable crawl and
  enrichment work. PostgreSQL is the durable source of truth; Redis is a cache
  and coordination layer, not the sole record of user-visible job state.
- Summoner match history reuses the collected match dataset and adds
  query-oriented tables/indexes and job metadata instead of maintaining a
  second duplicate match database.
- The rankings version/region catalog exposes only slices with non-empty
  published rollups. Raw-only versions remain valid in summoner history but do
  not appear as empty choices in statistics filters; `latest` follows the
  newest published rollup rather than the newest raw match.
- Champion-detail rune signatures preserve two style IDs, all six selected
  perk IDs, and three stat-shard IDs. Do not omit `perk5` when rebuilding the
  rollup or shift the API slice boundaries back to the old ten-field shape.

## Current summoner-history contract

- The initial queue filters are Draft Pick (`400`), Ranked Solo/Duo (`420`),
  Ranked Flex (`440`), and Swiftplay (`480`). `ALL` is the union of those four.
- Each returned history match includes its ten participants so the match card
  can expand into blue-team and red-team player details without per-row API
  requests. Older collected matches may not have Riot ID labels; the combat
  facts remain available and the UI falls back to participant numbers.
- Match history exposes participant rank snapshots plus the match average tier
  and coverage when enrichment data exists. These are nearest available
  snapshots rather than guaranteed game-start ranks, so the UI marks them with
  an approximation indicator and keeps missing values explicit as pending.
- On-demand summoner refreshes enrich distinct participants only within the
  lookup job, using ranked-solo entries, then recompute those matches' average
  tiers before finalizing the job. Participant-scoped terminal rank failures
  leave only those rows pending and make the completed job partial.
- The browser keeps the five most recent summoner searches in local storage,
  deduplicated by region and Riot ID, and exposes an explicit clear action.
- A lookup is a long-running server-side operation. The UI must be able to
  reconnect to an existing job after reload, navigation, or a repeated search.
- Duplicate requests for the same Riot ID must return the active or recent job
  rather than start duplicate Temporal workflows.
- Temporal workflow IDs are stable per lookup job. Activities own Riot and
  database side effects and must be retry-safe.
- Riot ID not found in a selected region is a terminal domain result. It must
  not become a retry storm or a misleading refresh-rate-limit response.
- A rate limit may delay new work, but it must not hide an already running or
  terminal job that the client should reconnect to.

## Current browser-auth contract

- Browser login initially enables Google only. Discord remains dormant, and
  Riot RSO remains a separate approval-gated integration.
- Google authorization uses one-shot database-backed state, S256 PKCE, and a
  short-lived HttpOnly browser-binding cookie. Provider exchange and user-info
  calls happen outside database transactions.
- The React application never receives or persists Google tokens or GOGG JWTs.
  It receives only an opaque HttpOnly application-session cookie; PostgreSQL
  stores the SHA-256 digest and owns expiry and revocation.
- The nullable GraphQL `Me` query is the frontend session source of truth.
  TanStack Query restores it on reload; protected routes distinguish anonymous
  results from session-service failures.
- Cookie-authenticated unsafe requests require `X-GOGG-CSRF: 1`. Logout revokes
  the server session before clearing the cookie. Production callbacks and
  cookies require HTTPS; local Vite development proxies `/oauth` and `/auth` to
  the API on port 8080.
- OAuth starts use the shared Redis fixed-window limiter when Redis is
  configured. Login start also removes expired OAuth attempts and browser
  sessions, so the public entrypoint cannot grow those tables without bounds.

## Asset contract

- CommunityDragon game assets are synchronized into the configured shared
  game-assets root and served locally through `/game-assets`. On the primary
  workstation that root is `/mnt/gogg-db/game-assets` in the F-backed ext4
  VHDX.
- Item synchronization includes valid icon-bearing entries even when
  CommunityDragon marks them `inStore=false`; transformed items, quest rewards,
  and consumables can still appear in a participant's final inventory.
- The frontend should not depend on CommunityDragon availability during normal
  page rendering.
- Missing versioned assets may be fetched and cached by the local API/worker
  path, with tests covering routing and fallback behavior.

## Current TFT product contract

- TFT collection runs in its own Temporal worker and task queues across all 15
  supported platforms, while Redis coordinates the Riot quota shared with LoL.
  Scheduled crawl and static-data schedules are deployed paused and controlled
  independently from LoL work.
- Standard ranked analysis uses immutable published lineup datasets. The React
  `/tft` page reads only catalog-backed filter combinations through
  `tftAnalysisCatalog` and `tftLineups`; URLs preserve the selected platform,
  patch, set, cohort, window, and sample threshold.
- TFT player history has a separate 15-platform identity and lookup-job model.
  `/tft/player/:platform/:gameName/:tagLine` reconnects to stable Temporal jobs,
  polls their persisted status, and reads canonical match facts with cursor
  pagination through `tftMatchHistory`.
- On-demand player matches remain outside analysis cohorts. Account, platform
  profile, match-list, and match-detail responses use the same raw-first archive
  as scheduled collection; activities are retry-safe and terminal Riot 404
  outcomes are persisted without retry storms.
- TFT entity images are served only from published local `/game-assets` static
  snapshots. Entity names and image paths are resolved per match patch so a
  cross-patch history page cannot mix versions.
- CommunityDragon TFT client catalogs do not share one object schema:
  `tftsets` identifies sets with `SetName`, champions nest
  `character_record.character_id`, traits use `trait_id`, and items and portals
  commonly use `nameId`. The parser version participates in the immutable
  snapshot revision so parser upgrades build a new snapshot; retries of that
  same revision merge objects idempotently and resume its static-asset jobs.
- Static sync publishes CommunityDragon before attempting Data Dragon. Asset
  jobs are scoped to the snapshots created by that source sync, duplicate URLs
  across locales share one network fetch, and publication accepts only local
  completed assets or explicitly skipped optional placeholders.
- TFT crawl progress is run-scoped by unique `(routing_region, match_id)` and
  must remain distinct from the shared global routing queues. Operator status
  reads a consistent PostgreSQL snapshot, exposes not-yet-enqueued discoveries,
  and falls back to the latest persisted run after the Temporal execution ends.

## Local development facts

- `make dev` starts PostgreSQL, Redis, Temporal, Temporal UI, and supporting
  local services.
- On the primary Windows workstation, application and Temporal PostgreSQL data
  live in the ext4 filesystem inside `F:\gogg-data\gogg-db.vhdx`. Ubuntu
  mounts filesystem UUID `3f631b73-56a3-4767-9728-92ecd0445366` at
  `/mnt/gogg-db`. Application and Temporal PostgreSQL, Redis, Riot raw
  archives, game assets, Prometheus, Grafana, and performance results use
  bind-mounted or direct subdirectories beneath that root. Ignored experiment
  datasets and artifacts are linked to the same storage root.
- `make dev` fails closed unless the external database filesystem has the
  expected UUID, ext4 type, read-write state, marker, data directories,
  ownership, and modes. Local API and worker launch targets apply the same
  preflight so a missing F mount cannot silently create split data elsewhere.
  Ubuntu startup is configured to request an on-demand elevated Windows task
  to attach the VHDX, then mount it inside the normal Ubuntu session. Windows
  logon does not start WSL for this storage path. Keep attachment and mounting
  separate; do not combine them in one elevated WSL mount namespace.
- Verified pre-migration logical backups exist on separate physical disks at
  `F:\gogg-backups\pre-migration-20260828T162134Z` and
  `D:\gogg-backups\pre-migration-20260828T162134Z`. The obsolete Docker named
  volumes `gogg-dev_pg_data` and `gogg-dev_temporal_pg_data` were removed only
  after offline-copy, SQL, Temporal, and cold-mount verification passed.
- API: `http://localhost:8080`; web: `http://localhost:5173`; Temporal UI:
  `http://localhost:8233`; local PostgreSQL is exposed on port `55433`.
- API and worker must point at the same database during summoner-flow testing.
- A PostgreSQL DSN is one uninterrupted URL. A newline between `?` and
  `sslmode=disable` creates an invalid control character and prevents startup.
- After API or worker code changes, restart the corresponding long-running
  process; Vite handles normal frontend hot reload.

## Proven failure modes and guardrails

- Browser state alone is insufficient for a long lookup. Persist the job
  identity server-side and make status queryable by the stable player identity.
- Apply active-job lookup before refresh throttling. Otherwise reloads and
  duplicate searches surface “too many requests” instead of progress.
- Mark terminal Riot outcomes non-retryable at the worker boundary. Generic
  retryable 404 handling caused long stalls in real testing.
- Keep workflow progress monotonic and useful even when individual match
  details are missing or skipped.
- Releasing an unfinished static-asset lease must also undo the claim attempt.
  Otherwise cancellation can create `pending` rows that have exhausted their
  attempt count, remain visible as work, and hot-loop without being claimable.
- Persist static-asset retry eligibility in PostgreSQL and make both claims and
  progress timers honor it. Do not consume all attempts in a tight workflow
  loop after a transient resource failure.
- Temporal schedules persist workflow type strings. Register legacy Go function
  names for replay compatibility, but write stable contract aliases into all
  new schedule actions.
- Inside `workflow.Go`, derive child options and execute every blocking call
  from the callback context. Reusing a parent coroutine's context in
  `Future.Get` panics live Workflow Tasks with an already-blocked-coroutine
  error; keep shared cancellation on the context passed into `workflow.Go`.
- Do not replace a stale TFT Riot-ID-to-PUUID mapping with a DELETE CTE followed
  by INSERT. PostgreSQL's same-statement snapshot can still trip the expression
  unique index; perform the delete and PUUID upsert as two statements in one
  transaction.
- Never invoke the region-wide phase 3.5 backlog scan from a single summoner
  lookup; rank enrichment must remain scoped by the lookup job's match set.
- Never use `make dev-reset` as an automatic recovery step; it deletes local
  volumes.
- Do not move live PostgreSQL data directories onto exFAT. If storage migration
  is revisited, use a database-supported filesystem or logical backup/restore.

## How to update this memory

Add an entry only when at least one of these is true:

- code and tests establish a stable product contract;
- the user accepts a durable architectural decision;
- a real failure reveals a reusable guardrail;
- environment behavior is repeatedly needed across sessions.

Remove or revise stale entries in the same change that invalidates them. Never
store secrets, access tokens, private user data, raw chat transcripts, temporary
plans, or unverified guesses.
