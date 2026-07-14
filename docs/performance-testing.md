# Local API performance testing

This setup keeps development dependencies and observability in one Compose
project while running the Go API on the host for fast iteration.

## Start

```bash
make dev
make observability
```

Open:

- Grafana: <http://localhost:3001> (`admin` / `admin` for local admin access)
- Prometheus: <http://localhost:9090>
- Observed API: <http://localhost:18080>
- API metrics: <http://localhost:18080/metrics>

Grafana provisions the **Gogg API Overview** dashboard automatically. Prometheus
retains local metrics for 15 days. The observed API runs in Compose so its
database, cache, load-generator, and scrape network are reproducible across
Linux and Docker Desktop. `make run-api` remains available on port 8080 for
fast host-side development, but it is not the target of this dashboard.
The observed API uses a 60-second write timeout so an uncached aggregate can
complete and be measured; regular application defaults are not changed.

## Run a baseline

The default test runs 20 virtual users for one minute against rankings with a
warm Redis cache:

```bash
make perf-warm
```

Override the scenario and load:

```bash
make perf-warm PERF_SCENARIO=graphql PERF_VUS=50 PERF_DURATION=2m
```

Supported scenarios are `rankings`, `versions`, `regions`, and `graphql`.
Every invocation creates a new run directory instead of overwriting an earlier
result. Group related before/after runs under one experiment ID:

```bash
export PERF_EXPERIMENT=EXP-20260714-01

# One uncounted environment warm-up, followed by three measured baseline runs.
make perf-warm PERF_VARIANT=warmup
make perf-warm PERF_VARIANT=baseline
make perf-warm PERF_VARIANT=baseline
make perf-warm PERF_VARIANT=baseline

# After making one isolated change, run the same load three times.
make perf-warm PERF_VARIANT=candidate
make perf-warm PERF_VARIANT=candidate
make perf-warm PERF_VARIANT=candidate
```

If `PERF_EXPERIMENT` is omitted, a timestamp-based ID is generated. Results use
the following layout:

```text
tmp/performance/<experiment>/<variant>/<scenario>-<mode>/run-NN/
  metadata.env       # timestamps, commit, image, dataset ID, load, and host
  dataset.env        # sorted PostgreSQL table row estimates behind dataset ID
  k6-summary.json    # raw k6 result
```

`tmp/performance/` is intentionally not committed. Commit only the experiment
conclusion to `docs/api-optimization-log.md`, including the experiment ID, the
three run IDs, and their median. Do not compare results with different dataset,
scenario, cache mode, VUs, duration, or machine configuration.

The cold-cache target deletes local Redis data and measures exactly one request.
This avoids mixing the initial database load with later cache hits. It is
deliberately wired to the Compose Redis container and must not be copied into a
shared or production environment:

```bash
make perf-cold
```

Compare the k6 p50/p95/p99 and response bytes with the Grafana API latency,
Redis hit-rate, and PostgreSQL connection panels. First perform one uncounted
environment warm-up, then run three or more measured iterations and use their
median when comparing code changes. For the single-request cold test, compare
request duration rather than percentiles within one run; use the median across
three or more cold runs.

## Inspect PostgreSQL

`pg_stat_statements` is enabled by the local PostgreSQL container. View the top
queries by cumulative execution time:

```bash
make db-slow-queries
```

For one ranking query, substitute representative parameters and run
`EXPLAIN (ANALYZE, BUFFERS, WAL, SETTINGS)` directly in `psql`. Do not use
`EXPLAIN ANALYZE` on mutating statements unless they are wrapped in a rolled-back
transaction.

## Interpretation

- k6 reports client-observed latency and transfer size.
- API Prometheus metrics report server processing latency and response size.
- Redis exporter reports keyspace hit/miss behavior for the local Redis instance.
- PostgreSQL exporter reports database health and activity.
- `pg_stat_statements` attributes database execution time to normalized SQL.

Local results find regressions and validate improvements. Capacity decisions and
release gates require a dedicated test environment with production-like data and
an external load generator.
