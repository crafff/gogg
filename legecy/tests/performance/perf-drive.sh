#!/usr/bin/env bash
set -euo pipefail

readonly drive_root=${GOGG_PERF_DRIVE_ROOT:-/mnt/gogg-perf}
readonly expected_uuid=${GOGG_PERF_DRIVE_UUID:-cca4d6cf-e43e-4a46-be1e-38c73d4dc341}
readonly dataset_name=${GOGG_PERF_DATASET:-rankings-kr-16.13-current-v1}
readonly dataset_dir="$drive_root/snapshots/$dataset_name"
readonly compose_file=deploy/compose/docker-compose.perf-drive.yml
readonly dev_compose_file=deploy/compose/docker-compose.dev.yml

export GOGG_PERF_DRIVE_ROOT="$drive_root"

die() {
  printf 'perf-drive: %s\n' "$*" >&2
  exit 1
}

preflight() {
  local actual_uuid options
  mountpoint -q "$drive_root" || die "$drive_root is not a mount point"
  actual_uuid=$(findmnt -no UUID --target "$drive_root")
  test "$actual_uuid" = "$expected_uuid" ||
    die "wrong drive UUID: got $actual_uuid, expected $expected_uuid"
  options=$(findmnt -no OPTIONS --target "$drive_root")
  case ",$options," in
    *,rw,*) ;;
    *) die "$drive_root is not mounted read-write" ;;
  esac
  test -w "$drive_root" || die "$drive_root is not writable by $(id -un)"
}

snapshot() {
  local running
  preflight
  running=$(docker compose -f "$dev_compose_file" exec -T postgres \
    psql -U gogg -d gogg -X -Atc \
    "SELECT count(*) FROM runs
     WHERE status = 'running'
       AND updated_at >= now() - interval '10 minutes'")
  test "$running" = 0 || die "$running crawler run(s) were updated in the last 10 minutes"

  mkdir -p "$dataset_dir"
  test ! -e "$dataset_dir/database.dump" ||
    die "$dataset_dir/database.dump already exists; refusing to overwrite it"

  docker compose -f "$dev_compose_file" exec -T postgres \
    pg_dump -U gogg -d gogg --format=custom --compress=6 --no-owner --no-acl \
    >"$dataset_dir/database.dump"

  docker compose -f "$dev_compose_file" exec -T postgres \
    psql -U gogg -d gogg -X -A -F '|' -P pager=off >"$dataset_dir/manifest.txt" <<'SQL'
SELECT 'postgres_version', current_setting('server_version');
SELECT 'schema_migration', version, dirty FROM schema_migrations;
SELECT 'game_version', version, patch_start_at, is_latest
FROM game_versions WHERE version = '16.13';
SELECT 'matches', region, fetch_status, timeline_status, count(*)
FROM matches WHERE version = '16.13'
GROUP BY region, fetch_status, timeline_status
ORDER BY region, fetch_status, timeline_status;
SELECT 'kr_time_range', min(game_start_ts), max(game_start_ts), max(created_at)
FROM matches WHERE region = 'KR' AND version = '16.13';
SELECT 'kr_tier', coalesce(avg_tier, 'NULL'), count(*)
FROM matches WHERE region = 'KR' AND version = '16.13'
GROUP BY avg_tier ORDER BY avg_tier;
SELECT 'kr_position', coalesce(mp.team_position, 'NULL'), count(*)
FROM match_participants mp JOIN matches m USING (match_id)
WHERE m.region = 'KR' AND m.version = '16.13'
GROUP BY mp.team_position ORDER BY mp.team_position;
SELECT 'kr_matches', count(*) FROM matches
WHERE region = 'KR' AND version = '16.13';
SELECT 'kr_participants', count(*)
FROM match_participants mp JOIN matches m USING (match_id)
WHERE m.region = 'KR' AND m.version = '16.13';
SQL

  (cd "$dataset_dir" && sha256sum database.dump manifest.txt >SHA256SUMS)
  printf 'Snapshot created: %s\n' "$dataset_dir"
}

up() {
  preflight
  mkdir -p "$drive_root/postgres-16"
  docker compose -f "$compose_file" up -d --wait postgres
  printf 'Performance PostgreSQL: postgres://gogg:goggpass@localhost:55434/gogg?sslmode=disable\n'
}

restore() {
  preflight
  test -s "$dataset_dir/database.dump" || die "missing snapshot: $dataset_dir/database.dump"
  (cd "$dataset_dir" && sha256sum --check SHA256SUMS)
  docker compose -f "$compose_file" up -d --wait postgres
  docker compose -f "$compose_file" exec -T postgres \
    dropdb -U gogg --if-exists gogg
  docker compose -f "$compose_file" exec -T postgres \
    createdb -U gogg gogg
  docker compose -f "$compose_file" exec -T postgres \
    pg_restore -U gogg -d gogg --no-owner --no-acl <"$dataset_dir/database.dump"
  docker compose -f "$compose_file" exec -T postgres \
    psql -U gogg -d gogg -v ON_ERROR_STOP=1 -c 'ANALYZE;'
  printf 'Dataset restored to postgres://gogg:goggpass@localhost:55434/gogg?sslmode=disable\n'
}

down() {
  preflight
  docker compose -f "$compose_file" down
}

status() {
  preflight
  df -h "$drive_root"
  docker compose -f "$compose_file" ps
}

case ${1:-} in
  snapshot) snapshot ;;
  up) up ;;
  restore) restore ;;
  down) down ;;
  status) status ;;
  *) die "usage: $0 snapshot|up|restore|down|status" ;;
esac
