#!/usr/bin/env bash
set -euo pipefail

readonly task_name='GOGG Database VHDX Attach'
readonly schtasks=/mnt/c/Windows/System32/schtasks.exe
readonly mount_helper=/home/zrt/apps/gogg/scripts/gogg-db-wsl-mount.sh
readonly expected_uuid=3f631b73-56a3-4767-9728-92ecd0445366
readonly vhdx=/mnt/f/gogg-data/gogg-db.vhdx
readonly log_file=/run/gogg-db-wsl-startup.log

exec 9>/run/gogg-db-wsl-startup.lock
flock 9
exec >>"${log_file}" 2>&1

printf '%s starting GOGG database mount\n' "$(date --iso-8601=seconds)"

if bash "${mount_helper}" probe >/dev/null 2>&1; then
  echo 'GOGG database storage is already ready.'
  exit 0
fi

if [[ ! -f "${vhdx}" ]]; then
  echo "GOGG database VHDX is unavailable; skipping mount: ${vhdx}"
  exit 0
fi

device=$(blkid -U "${expected_uuid}" || true)
if [[ ! -b "${device}" ]]; then
  if [[ ! -x "${schtasks}" ]]; then
    echo "Windows Task Scheduler client is unavailable: ${schtasks}" >&2
    exit 1
  fi

  "${schtasks}" /Run /TN "${task_name}" | tr -d '\r'

  device=
  for _ in $(seq 1 120); do
    device=$(blkid -U "${expected_uuid}" || true)
    [[ -b "${device}" ]] && break
    sleep 0.5
  done
fi

if [[ ! -b "${device}" ]]; then
  echo "Timed out waiting for GOGG database filesystem ${expected_uuid}." >&2
  exit 1
fi

bash "${mount_helper}" mount
echo 'GOGG database storage mounted during Ubuntu startup.'
