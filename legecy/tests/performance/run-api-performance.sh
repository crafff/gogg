#!/usr/bin/env bash
set -euo pipefail

mode=${1:?usage: run-api-performance.sh warm|cold}
case "$mode" in
  warm|cold) ;;
  *) echo "unknown cache mode: $mode" >&2; exit 2 ;;
esac

scenario=${PERF_SCENARIO:-rankings_graphql}
vus=${PERF_VUS:-20}
duration=${PERF_DURATION:-1m}
root=${PERF_DIR:-tmp/performance}
experiment=${PERF_EXPERIMENT:-EXP-$(date +%Y%m%d-%H%M%S)}
variant=${PERF_VARIANT:-baseline}
compose_file=${COMPOSE_FILE:-deploy/compose/docker-compose.dev.yml}
perf_db_container=${PERF_DB_CONTAINER:-}
stable_dataset=rankings-kr-16.13-current-v1
snapshot_sha=a9402acd48c1a5229581dffb41fc72dffea54bc355bf58638741925dd54e9a43
prometheus_url=${PERF_PROMETHEUS_URL:-http://localhost:9090}
api_container=${PERF_API_CONTAINER:-gogg-dev-api-observed}

case "$experiment/$variant/$scenario" in
  *[!A-Za-z0-9._/-]*)
    echo "experiment, variant, and scenario may only contain letters, numbers, ., _, and -" >&2
    exit 2
    ;;
esac

base="$root/$experiment/$variant/$scenario-$mode"
mkdir -p "$base"
run=1
while ! mkdir "$base/run-$(printf '%02d' "$run")" 2>/dev/null; do
  run=$((run + 1))
done
run_id=$(printf 'run-%02d' "$run")
output="$base/$run_id"

commit=$(git rev-parse --verify HEAD 2>/dev/null || printf unknown)
git status --short --branch >"$output/git-status.txt" 2>/dev/null || true
git diff --binary >"$output/git-diff.patch" 2>/dev/null || true
git diff --cached --binary >"$output/git-diff-cached.patch" 2>/dev/null || true
if test -n "$(git status --porcelain 2>/dev/null)"; then dirty=true; else dirty=false; fi
started_at=$(date --iso-8601=seconds)
started_epoch=$(date +%s)

cat >"$output/metadata.env" <<EOF
experiment=$experiment
variant=$variant
run=$run_id
started_at=$started_at
git_commit=$commit
git_dirty=$dirty
scenario=$scenario
cache_mode=$mode
vus=$vus
duration=$duration
hostname=$(hostname)
os=$(uname -srmo)
docker=$(docker version --format '{{.Client.Version}}' 2>/dev/null || printf unknown)
prometheus_url=$prometheus_url
EOF

finish() {
  status=$?
  {
    printf 'finished_at=%s\n' "$(date --iso-8601=seconds)"
    printf 'exit_code=%s\n' "$status"
  } >>"$output/metadata.env"
}
trap finish EXIT

if test -n "$perf_db_container"; then
  test "$(docker inspect -f '{{.State.Health.Status}}' "$perf_db_container" 2>/dev/null)" = healthy || {
    echo "performance database $perf_db_container is not healthy; run make perf-env-up" >&2
    exit 2
  }
  api_env=$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' gogg-dev-api-observed 2>/dev/null || true)
  case "$api_env" in
    *GOGG_DATABASE_DSN=*host.docker.internal:55434*) ;;
    *) echo "observed API is not connected to the mobile-drive database; run make perf-env-up" >&2; exit 2 ;;
  esac
  kr_matches=$(docker exec -i "$perf_db_container" psql -U gogg -d gogg -X -Atc \
    "SELECT count(*) FROM matches WHERE region='KR' AND version='16.13'")
  test "$kr_matches" = 159644 || {
    echo "unexpected KR 16.13 match count: $kr_matches (want 159644)" >&2
    exit 2
  }
  docker exec -i "$perf_db_container" psql -U gogg -d gogg -X \
    -Atc "SELECT format('%s=%s', relname, n_live_tup) FROM pg_stat_user_tables ORDER BY relname" \
    >"$output/dataset.env"
else
  docker compose -f "$compose_file" exec -T postgres psql -U gogg -d gogg \
    -Atc "SELECT format('%s=%s', relname, n_live_tup) FROM pg_stat_user_tables ORDER BY relname" \
    >"$output/dataset.env"
fi
dataset_id=$(sha256sum "$output/dataset.env" | cut -c1-12)
api_image=$(docker inspect --format '{{.Image}}' "$api_container" 2>/dev/null || printf unknown)
{
  printf 'dataset_id=%s\n' "$dataset_id"
  printf 'stable_dataset=%s\n' "$stable_dataset"
  printf 'snapshot_sha256=%s\n' "$snapshot_sha"
  printf 'api_image=%s\n' "$api_image"
} >>"$output/metadata.env"

if test "$mode" = cold; then
  docker compose -f "$compose_file" exec -T redis redis-cli FLUSHDB >/dev/null
fi

# Reset only the performance database's statement counters. This makes the
# archived pg_stat_statements snapshot attributable to this run instead of to
# earlier experiments.
if test -n "$perf_db_container"; then
  docker exec -i "$perf_db_container" psql -U gogg -d gogg -X -v ON_ERROR_STOP=1 \
    -Atc 'SELECT pg_stat_statements_reset();' >/dev/null
else
  docker compose -f "$compose_file" exec -T postgres psql -U gogg -d gogg \
    -X -v ON_ERROR_STOP=1 -Atc 'SELECT pg_stat_statements_reset();' >/dev/null
fi

docker stats --no-stream --format \
  '{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDs}}' \
  "$api_container" >"$output/container-stats-before.tsv"

set +e
docker compose -f "$compose_file" --profile observability --profile performance run --rm --no-deps \
  -e SCENARIO="$scenario" -e VUS="$vus" -e DURATION="$duration" \
  -e CACHE_MODE="$mode" -e TEST_MODE="$mode" \
  k6 run --summary-export="/results/$experiment/$variant/$scenario-$mode/$run_id/k6-summary.json" \
  /scripts/api-baseline.js
status=$?
set -e

finished_at=$(date --iso-8601=seconds)
finished_epoch=$(date +%s)

mkdir -p "$output/prometheus"
cat >"$output/prometheus/queries.tsv" <<'EOF'
api_cpu	rate(process_cpu_seconds_total{job="gogg-api"}[1m])
api_resident_memory	process_resident_memory_bytes{job="gogg-api"}
api_heap	go_memstats_heap_alloc_bytes{job="gogg-api"}
api_goroutines	go_goroutines{job="gogg-api"}
api_in_flight	gogg_api_http_requests_in_flight
api_request_rate	sum(rate(gogg_api_http_requests_total[1m]))
api_latency_rate	sum(rate(gogg_api_http_request_duration_seconds_sum[1m])) / clamp_min(sum(rate(gogg_api_http_request_duration_seconds_count[1m])), 0.001)
redis_hits	rate(redis_keyspace_hits_total[1m])
redis_misses	rate(redis_keyspace_misses_total[1m])
redis_memory	redis_memory_used_bytes
postgres_connections	pg_stat_database_numbackends{datname="gogg"}
postgres_blocks_read	rate(pg_stat_database_blks_read{datname="gogg"}[1m])
postgres_blocks_hit	rate(pg_stat_database_blks_hit{datname="gogg"}[1m])
postgres_transactions	rate(pg_stat_database_xact_commit{datname="gogg"}[1m])
EOF

observation_status=0
while IFS=$'\t' read -r name query; do
  curl -fsS -G "$prometheus_url/api/v1/query_range" \
    --data-urlencode "query=$query" \
    --data-urlencode "start=$started_epoch" \
    --data-urlencode "end=$finished_epoch" \
    --data-urlencode 'step=5s' \
    >"$output/prometheus/$name.json" || observation_status=1
done <"$output/prometheus/queries.tsv"

docker logs --since "$started_at" --until "$finished_at" "$api_container" \
  >"$output/api.log" 2>&1 || observation_status=1
docker stats --no-stream --format \
  '{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}\t{{.PIDs}}' \
  "$api_container" >"$output/container-stats-after.tsv" || observation_status=1

if test -n "$perf_db_container"; then
  docker exec -i "$perf_db_container" psql -U gogg -d gogg -X -P pager=off \
    -f /dev/stdin <deploy/observability/postgres/slow-queries.sql \
    >"$output/postgres-statements.txt" || observation_status=1
else
  docker compose -f "$compose_file" exec -T postgres psql -U gogg -d gogg -X -P pager=off \
    -f /dev/stdin <deploy/observability/postgres/slow-queries.sql \
    >"$output/postgres-statements.txt" || observation_status=1
fi

printf 'observation_exit_code=%s\n' "$observation_status" >>"$output/metadata.env"
if test "$observation_status" -ne 0; then
  echo "one or more observation artifacts could not be archived" >&2
  test "$status" -ne 0 || status=3
fi

printf 'Performance result: %s\n' "$output"
exit "$status"
