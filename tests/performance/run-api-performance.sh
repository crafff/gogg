#!/usr/bin/env bash
set -euo pipefail

mode=${1:?usage: run-api-performance.sh warm|cold}
case "$mode" in
  warm|cold) ;;
  *) echo "unknown cache mode: $mode" >&2; exit 2 ;;
esac

scenario=${PERF_SCENARIO:-rankings}
vus=${PERF_VUS:-20}
duration=${PERF_DURATION:-1m}
root=${PERF_DIR:-tmp/performance}
experiment=${PERF_EXPERIMENT:-EXP-$(date +%Y%m%d-%H%M%S)}
variant=${PERF_VARIANT:-baseline}
compose_file=${COMPOSE_FILE:-deploy/compose/docker-compose.dev.yml}

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
EOF

finish() {
  status=$?
  {
    printf 'finished_at=%s\n' "$(date --iso-8601=seconds)"
    printf 'exit_code=%s\n' "$status"
  } >>"$output/metadata.env"
}
trap finish EXIT

docker compose -f "$compose_file" exec -T postgres psql -U gogg -d gogg \
  -Atc "SELECT format('%s=%s', relname, n_live_tup) FROM pg_stat_user_tables ORDER BY relname" \
  >"$output/dataset.env"
dataset_id=$(sha256sum "$output/dataset.env" | cut -c1-12)
api_image=$(docker inspect --format '{{.Image}}' gogg-dev-api-observed 2>/dev/null || printf unknown)
{
  printf 'dataset_id=%s\n' "$dataset_id"
  printf 'api_image=%s\n' "$api_image"
} >>"$output/metadata.env"

if test "$mode" = cold; then
  docker compose -f "$compose_file" exec -T redis redis-cli FLUSHDB >/dev/null
fi

set +e
docker compose -f "$compose_file" --profile observability --profile performance run --rm \
  -e SCENARIO="$scenario" -e VUS="$vus" -e DURATION="$duration" \
  -e CACHE_MODE="$mode" -e TEST_MODE="$mode" \
  k6 run --summary-export="/results/$experiment/$variant/$scenario-$mode/$run_id/k6-summary.json" \
  /scripts/api-baseline.js
status=$?
set -e

printf 'Performance result: %s\n' "$output"
exit "$status"
