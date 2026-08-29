#!/usr/bin/env bash
set -euo pipefail

readonly action=${1:-}
readonly db_root=/mnt/gogg-db
readonly expected_uuid=3f631b73-56a3-4767-9728-92ecd0445366
readonly preflight=/home/zrt/apps/gogg/scripts/gogg-db-preflight.sh

case "${action}" in
  probe)
    bash "${preflight}" >/dev/null 2>&1
    ;;
  device)
    blkid -U "${expected_uuid}"
    ;;
  mount)
    if findmnt -rn -M "${db_root}" >/dev/null 2>&1; then
      bash "${preflight}"
      exit 0
    fi

    device=
    for _ in $(seq 1 60); do
      device=$(blkid -U "${expected_uuid}" || true)
      [[ -b "${device}" ]] && break
      sleep 0.5
    done
    [[ -b "${device}" ]]
    mkdir -p "${db_root}"
    mount -U "${expected_uuid}" "${db_root}"
    bash "${preflight}"
    ;;
  *)
    echo "usage: $0 {probe|device|mount}" >&2
    exit 2
    ;;
esac
