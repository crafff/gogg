# ADR-0004: Independent TFT data pipeline with shared Riot quota authority

Status: Accepted

## Context

GOGG needs Teamfight Tactics match collection for composition analysis and
later player match history. Riot application limits apply to one credential
and route, so LoL and TFT collectors cannot safely coordinate only inside one
process. TFT also has different ladder, match, static-data, eligibility, and
analysis semantics from League of Legends.

## Decision

- Run TFT in its own `tft-worker` process and Temporal task queues. Its crawl
  and static schedules are created paused and require an explicit operator
  action to start. Persist both Temporal's Workflow ID and unique execution
  Run ID; the latter is the idempotency key for one crawl, while the stable
  schedule/profile scope identifies stale runs across scheduled executions.
  Do not impose a short root execution timeout because an operator pause is a
  durable state. State transitions retry until persisted, and a newer
  execution reconciles an active row left by a closed execution of the same
  logical workflow.
- Fan one crawl workflow out over all supported platform routes and four match
  routing regions. Keep platform and routing-region identity on every record.
- Treat platform match-list responses as run-scoped candidates. After every
  platform has completed discovery, deterministically admit at most the frozen
  per-routing-region target by a versioned stable hash, then project only the
  admitted candidates into discovery provenance and the global detail-job
  queue. Candidate arrival order, activity retries, and cached global jobs must
  not change the admitted sample; a route with insufficient candidates reports
  a shortfall instead of silently shrinking healthy routes. Keep player match
  watermarks run-local while candidates are open; the finalizer atomically
  seals admission, enqueues detail jobs, and commits those watermarks.
- Make a dedicated no-eviction Redis the fail-closed request authority shared
  by LoL and TFT workers.
  Application and conservative routing-family budgets omit the product name;
  method budgets remain endpoint-specific. Operator weights split available
  application capacity while both products are active.
- Archive every successful dynamic Riot response before JSON decoding in a
  permanent content-addressed gzip store, then ingest replayable normalized
  TFT facts into independent tables.
- Analyze standard ranked TFT (`queue_id = 1100`) in separate `MASTER_PLUS`
  and `DIAMOND` cohorts. Exact lineup identity is the sorted set of mapped,
  purchasable, positive-cost unit IDs; equipment and tiers are attributes, not
  identity.
- Synchronize Data Dragon and CommunityDragon snapshots independently from
  player crawling. Publish immutable static revisions only after their local
  assets are complete; the API never points clients at third-party asset hosts.
- Freeze each crawl to the newest requested published CommunityDragon patch.
  Matches from other patches remain in the raw archive and normalized history
  facts, but are explicitly excluded from that run's composition analysis so a
  fresh deployment cannot deadlock waiting for historical static catalogs.
  When one of those patches later becomes the target, its archived jobs are
  requeued and enriched against the newly published catalog.
- Expose published analysis through GraphQL. Player-history queries reuse the
  same match facts when added later and do not create a second match store.

## Consequences

TFT can be paused without interrupting LoL work, while both products still
obey one cross-process rate authority. The extra raw and publication layers
increase storage use, but preserve upstream evidence, replayability, and stable
read behavior during rebuilds. Redis is required by both production workers;
if it is unavailable, Riot requests stop instead of running uncoordinated.
The candidate/admission split adds temporary run-scoped rows, but prevents
large platform ladders from dominating a routing-region comparison and keeps
unselected cached matches out of analysis publications.
