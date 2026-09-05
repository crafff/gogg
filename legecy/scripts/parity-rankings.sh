#!/usr/bin/env bash
# Diff /api/rankings/champions (legacy) vs /api/v1/rankings/champions (new)
# across a curated set of filter combos. Run after starting both binaries
# side-by-side (see docs/runbooks/parity-test.md).
#
# Usage: scripts/parity-rankings.sh [legacy_base] [new_base]
#   defaults: http://localhost:8080  http://localhost:8081
#
# Exit code: 0 if all combos parity OK (or only the documented
# err.Error()-vs-sanitized divergence on error paths), 1 otherwise.

set -euo pipefail

LEGACY="${1:-http://localhost:8080}"
NEW="${2:-http://localhost:8081}"

CASES=(
  "?limit=5"
  "?limit=5&minGames=20"
  "?limit=5&queueId=420"
  "?limit=5&region=KR"
  "?limit=5&tier=master_plus"
  "?limit=5&position=MIDDLE"
  "?limit=5&position=MIDDLE&tier=challenger"
  "?limit=5&positionThreshold=10"
  "?limit=5&version=15.1.1"
  "?limit=5&version=latest"
  "?limit=200&minGames=1&position=TOP&tier=master&region=KR"
)

ok=0
fail=0
allowed_divergence=0

for q in "${CASES[@]}"; do
  legacy=$(curl -s "${LEGACY}/api/rankings/champions${q}")
  new=$(curl -s "${NEW}/api/v1/rankings/champions${q}")

  if [ "$legacy" = "$new" ]; then
    echo "OK    ${q}"
    ok=$((ok+1))
    continue
  fi

  # ADR-0003-allowed divergence: legacy leaks err.Error() in the
  # body for 5xx responses; new stack sanitizes. Detect by both
  # bodies starting with "failed to" or {"error":...} respectively.
  if [[ "$legacy" == failed* ]] && [[ "$new" == *'"error":'* ]]; then
    echo "DIVERGE (ADR-0003 allowed): ${q}"
    allowed_divergence=$((allowed_divergence+1))
    continue
  fi

  echo "FAIL  ${q}"
  echo "  legacy: ${legacy:0:200}"
  echo "  new   : ${new:0:200}"
  fail=$((fail+1))
done

echo ""
echo "=== Summary: ${ok} ok, ${allowed_divergence} divergence (ADR-0003), ${fail} fail ==="

if [ "$fail" -gt 0 ]; then
  exit 1
fi
