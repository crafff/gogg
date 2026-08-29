#!/usr/bin/env bash
set -euo pipefail

readonly db_root=${GOGG_DB_ROOT:-/mnt/gogg-db}
readonly expected_uuid=3f631b73-56a3-4767-9728-92ecd0445366
readonly expected_marker=GOGG_DB_VOLUME_V1

fail() {
  echo "GOGG database storage check failed: $*" >&2
  echo "Mount F:\\gogg-data\\gogg-db.vhdx, then retry." >&2
  exit 1
}

mount_source=$(findmnt -rn -M "${db_root}" -o SOURCE) || \
  fail "${db_root} is not a mount point"
mount_uuid=$(findmnt -rn -M "${db_root}" -o UUID || true)
if [[ -z "${mount_uuid}" && -b "${mount_source}" ]]; then
  mount_uuid=$(blkid -s UUID -o value "${mount_source}" || true)
fi
[[ -n "${mount_uuid}" ]] || fail "cannot read the filesystem UUID"
mount_fstype=$(findmnt -rn -M "${db_root}" -o FSTYPE) || \
  fail "cannot read the filesystem type"
mount_options=$(findmnt -rn -M "${db_root}" -o OPTIONS) || \
  fail "cannot read the mount options"

[[ "${mount_uuid}" == "${expected_uuid}" ]] || \
  fail "unexpected UUID ${mount_uuid:-<empty>} on ${db_root}"
[[ "${mount_fstype}" == ext4 ]] || \
  fail "expected ext4, found ${mount_fstype:-<empty>}"
[[ ",${mount_options}," == *,rw,* ]] || \
  fail "${db_root} is not mounted read-write"
[[ -f "${db_root}/.gogg-db-volume" ]] || \
  fail "volume marker is missing"
grep -Fxq "${expected_marker}" "${db_root}/.gogg-db-volume" || \
  fail "volume marker is invalid"

for data_directory in app-postgres temporal-postgres; do
  target=${db_root}/${data_directory}
  [[ -d "${target}" ]] || fail "${target} is missing"
  [[ "$(stat -c '%u:%g:%a' "${target}")" == 70:70:700 ]] || \
    fail "${target} has unexpected ownership or permissions"
done

echo "GOGG database storage ready: ${mount_source} (${mount_fstype}, UUID ${mount_uuid}, rw)"
