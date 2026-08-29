# TFT collector runbook

The TFT pipeline is deliberately inert after deployment: both Temporal
schedules are created in the paused state. Run migrations and start the worker
before enabling either schedule.

## Build and deploy

```bash
docker build -f deploy/docker/worker.Dockerfile \
  --build-arg CMD_PATH=./apps/worker/cmd/tft-worker \
  -t gogg-tft-worker:prod .

docker build -f deploy/docker/worker.Dockerfile \
  --build-arg CMD_PATH=./apps/worker/cmd/crawlctl \
  -t gogg-crawlctl:prod .

docker compose \
  -f deploy/compose/docker-compose.prod.yml \
  -f deploy/compose/docker-compose.tft-worker.yml \
  --profile tft-worker up -d worker worker-tft
```

Run operator commands inside the Compose network so they reach the same
Temporal service and dedicated quota Redis as the workers:

```bash
tftctl() {
  docker compose \
    -f deploy/compose/docker-compose.prod.yml \
    -f deploy/compose/docker-compose.tft-worker.yml \
    --profile tft-ops run --rm crawlctl "$@"
}
```

The worker needs PostgreSQL, Temporal, the overlay's dedicated no-eviction
quota Redis, `riot.api_key`, a durable
`raw_archive.root`, and a writable `tft.static_root`. Configure all 15 platform
routes unless a deliberately smaller deployment is required.

## Safe startup

Start static synchronization first so match ingestion can map current units:

```bash
tftctl status --schedule gogg-tft-static
tftctl enable --schedule gogg-tft-static
tftctl trigger --schedule gogg-tft-static
```

`status` reports `syncing`, `downloading`, and `publishing`, the current source,
and exact snapshot-scoped asset counts. New runs synchronize, download, and
publish CommunityDragon before fetching Data Dragon catalogs or assets, so a
failure in those later stages does not withhold the catalog required by match
ingestion. Resolving the target patch still requires Data Dragon's
`versions.json`. Historical runs that predate scoped accounting print
`mode=legacy-global` with global processed and remaining counts instead of a
misleading percentage.

Static assets are claimed by unique URL across both locales and downloaded with
bounded concurrency. This avoids downloading the much larger full Data Dragon
archive while still removing duplicate locale requests. The defaults are 128
unique URLs per activity and 16 concurrent requests; the batch may not exceed
eight times the worker count. Transient failures use persisted exponential
backoff. Known missing Data Dragon queue-mode icons are recorded as `skipped`
and receive a valid local transparent PNG placeholder; other 403/404 responses
remain failures.

Each crawl freezes its target to an already published English CommunityDragon
snapshot. It fails fast with `MISSING_STATIC` if none exists. A seven-day match
window can cross a patch boundary; those non-target matches are still archived
and normalized for match-history use, but are excluded from lineup analysis
instead of waiting forever for an older catalog. When that patch becomes the
published target, the next crawl automatically requeues those facts for static
mapping and analysis.

After a static publication exists, inspect and enable crawling:

```bash
tftctl status --schedule gogg-tft-crawl-global
tftctl enable --schedule gogg-tft-crawl-global
tftctl trigger --schedule gogg-tft-crawl-global
```

`enable` and `trigger` refuse to proceed unless Temporal reports the required
workflow poller and all four regional activity pollers. This prevents a run
from being scheduled into an unserved queue.

Temporal uses overlap policy `SKIP`, a one-hour catch-up window, and
pause-on-failure. Restarting the worker updates schedule definitions without
changing their current paused state. Active crawl executions have no short
execution timeout, so an operator pause can last indefinitely; pause, resume,
and terminal database state changes retry durably until PostgreSQL recovers.

## Pause, resume, cancel, and weights

```bash
# Stop new runs and allow the current run to drain.
tftctl disable --schedule gogg-tft-crawl-global --drain

# Stop new runs and wait until the active run acknowledges its pause signal.
tftctl disable --schedule gogg-tft-crawl-global --pause-active

# Copy the real workflow ID printed by `tftctl status`.
tftctl resume-run --workflow-id <active-workflow-id>
tftctl cancel-run --workflow-id <active-workflow-id>

# Weighted sharing applies only while both products are actively requesting.
tftctl set-weight --product lol --weight 2
tftctl set-weight --product tft --weight 1
```

An HTTP 401/403 transitions the workflow to `paused_needs_auth`; replace the
credential, restart the worker, then use `resume-run`. A Riot 404 is terminal
for that match. Rate limits and transient failures retain retryable jobs with
their next eligible time.

## Low-rate routing-limit observation

The probe makes exactly one LoL and one TFT match-list request on the same
regional hostname. It prints only status and rate headers, never the key or
response body, and does not try to trigger a 429:

```bash
RIOT_API_KEY=... gogg-tft-quota-probe --platform KR --puuid <puuid>
```

Compare the two `app_count` values to gather evidence about cross-product
application accounting on that route. Repeat only after the reported window
has reset. This is observational evidence, not permission to route-hop around
a limit.

## Default scope and outputs

Each run collects Challenger and Grandmaster in full, deterministically samples
up to 500 Master players by LP decile, and samples 50 players from each Diamond
division. It requests 20 recent match IDs per selected seed in a seven-day
window ending 30 minutes behind real time. Later runs begin from the prior
player watermark minus a six-hour overlap. If a patch is older than 48 hours
and has fewer than 10,000 eligible participant observations, the sample caps
increase once to 1,000 Master and 100 per Diamond division; lower tiers remain
out of scope.

Raw captures, discovery edges, jobs, normalized matches and participants,
static snapshots, exact rollups, and immutable lineup publications are all
stored separately. GraphQL readers see published rows only through
`tftAnalysisCatalog` and `tftLineups`.

## Player-history lookups

The web routes `/tft/player` and
`/tft/player/:platform/:gameName/:tagLine` use the TFT-specific GraphQL
operations `refreshTFTPlayer`, `tftLookupJob`, and `tftMatchHistory`. A refresh
starts `TFTPlayerLookupWorkflow` on `tft-seed`; match-detail activities execute
on the selected routing-region match queue and therefore share the same Redis
quota authority and adaptive gate as scheduled collection.

Player lookups support all configured TFT platforms. They resolve Account-V1,
verify the platform with TFT Summoner-V1, archive Riot responses before parsing,
and ingest up to 20 recent matches idempotently. These facts are queryable as
history but are not admitted to high-tier analysis unless a scheduled discovery
edge independently includes the match.

Lookup jobs are reconnectable for seven days. Duplicate or reloaded requests
reuse an active/recent job before public refresh limits are evaluated. Use the
GraphQL job status rather than Temporal UI as the browser-facing source of
truth.
