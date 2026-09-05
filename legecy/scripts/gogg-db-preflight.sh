#!/usr/bin/env bash
set -euo pipefail

readonly data_root=${GOGG_DATA_ROOT:-${GOGG_DB_ROOT:-/mnt/gogg-db}}
readonly raw_archive_root=${GOGG_RAW_ARCHIVE_ROOT:-${data_root}/riot-raw}
readonly assets_root=${GOGG_ASSETS_ROOT:-${data_root}/game-assets}
readonly tft_static_root=${GOGG_TFT_STATIC_ROOT:-${assets_root}}
readonly performance_root=${PERF_DIR:-${data_root}/performance}
readonly expected_uuid=3f631b73-56a3-4767-9728-92ecd0445366
readonly expected_marker=GOGG_DB_VOLUME_V1

fail() {
  echo "GOGG data storage check failed: $*" >&2
  echo "Mount F:\\gogg-data\\gogg-db.vhdx, then retry." >&2
  exit 1
}

mount_source=$(findmnt -rn -M "${data_root}" -o SOURCE) || \
  fail "${data_root} is not a mount point"
mount_uuid=$(findmnt -rn -M "${data_root}" -o UUID || true)
if [[ -z "${mount_uuid}" && -b "${mount_source}" ]]; then
  mount_uuid=$(blkid -s UUID -o value "${mount_source}" || true)
fi
[[ -n "${mount_uuid}" ]] || fail "cannot read the filesystem UUID"
mount_fstype=$(findmnt -rn -M "${data_root}" -o FSTYPE) || \
  fail "cannot read the filesystem type"
mount_options=$(findmnt -rn -M "${data_root}" -o OPTIONS) || \
  fail "cannot read the mount options"

[[ "${mount_uuid}" == "${expected_uuid}" ]] || \
  fail "unexpected UUID ${mount_uuid:-<empty>} on ${data_root}"
[[ "${mount_fstype}" == ext4 ]] || \
  fail "expected ext4, found ${mount_fstype:-<empty>}"
[[ ",${mount_options}," == *,rw,* ]] || \
  fail "${data_root} is not mounted read-write"
[[ -f "${data_root}/.gogg-db-volume" ]] || \
  fail "volume marker is missing"
grep -Fxq "${expected_marker}" "${data_root}/.gogg-db-volume" || \
  fail "volume marker is invalid"

data_root_real=$(realpath -e -- "${data_root}") || \
  fail "cannot resolve ${data_root}"

check_configured_root() {
  local name=$1 configured=$2 resolved source uuid fstype options
  [[ -d "${configured}" ]] || fail "${name} path ${configured} is missing"
  resolved=$(realpath -e -- "${configured}") || \
    fail "cannot resolve ${name} path ${configured}"
  case "${resolved}" in
    "${data_root_real}"|"${data_root_real}"/*) ;;
    *) fail "${name} path ${configured} is outside ${data_root}" ;;
  esac

  source=$(findmnt -rn -T "${resolved}" -o SOURCE) || \
    fail "cannot find the filesystem for ${name} path ${configured}"
  uuid=$(findmnt -rn -T "${resolved}" -o UUID || true)
  if [[ -z "${uuid}" && -b "${source}" ]]; then
    uuid=$(blkid -s UUID -o value "${source}" || true)
  fi
  fstype=$(findmnt -rn -T "${resolved}" -o FSTYPE) || \
    fail "cannot read the filesystem type for ${name}"
  options=$(findmnt -rn -T "${resolved}" -o OPTIONS) || \
    fail "cannot read the mount options for ${name}"
  [[ "${uuid}" == "${expected_uuid}" ]] || \
    fail "${name} path ${configured} is not on the expected F-backed filesystem"
  [[ "${fstype}" == ext4 ]] || fail "${name} path ${configured} is not ext4"
  [[ ",${options}," == *,rw,* ]] || fail "${name} path ${configured} is not read-write"
  [[ -w "${resolved}" ]] || fail "${name} path ${configured} is not writable by the current user"
}

check_directory() {
  local directory=$1 expected=$2 target
  target=${data_root}/${directory}
  [[ -d "${target}" ]] || fail "${target} is missing"
  [[ "$(stat -c '%u:%g:%a' "${target}")" == "${expected}" ]] || \
    fail "${target} has unexpected ownership or permissions (want ${expected})"
}

check_directory app-postgres 70:70:700
check_directory temporal-postgres 70:70:700
check_directory redis 999:1000:750
check_directory riot-raw 1000:1000:2775
check_directory game-assets 1000:1000:2775
check_directory game-assets/profile-icons 1000:1000:2775
check_directory prometheus 65534:65534:750
check_directory grafana 472:0:750
check_directory performance 1000:1000:750
check_directory experiments 1000:1000:755

check_configured_root GOGG_RAW_ARCHIVE_ROOT "${raw_archive_root}"
check_configured_root GOGG_ASSETS_ROOT "${assets_root}"
check_configured_root GOGG_TFT_STATIC_ROOT "${tft_static_root}"
check_configured_root PERF_DIR "${performance_root}"

echo "GOGG data storage ready: ${mount_source} (${mount_fstype}, UUID ${mount_uuid}, rw)"
