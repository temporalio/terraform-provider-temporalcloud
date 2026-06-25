#!/usr/bin/env bash
#
# test-timing.sh - profile why the acceptance tests are slow.
#
# Layer 1 (always on): prints a list of tests sorted slowest-first, parsed from
# `go test -json` output, so you can see which tests dominate the runtime.
#
# Layer 2/3 (opt-in): set TEMPORALCLOUD_TRACE=1 to additionally emit per-gRPC-call
# timings, per-operation await summaries, and an aggregated gRPC call summary at
# the end of the run (see internal/client/client.go and the namespace wait loop).
#
# Usage:
#   TF_ACC=1 ./scripts/test-timing.sh                       # all provider tests
#   TF_ACC=1 ./scripts/test-timing.sh -run TestAccNamespace # a subset
#   TF_ACC=1 TEMPORALCLOUD_TRACE=1 ./scripts/test-timing.sh -run TestAccNamespace_Basic
#
# Requires the TEMPORAL_CLOUD_API_KEY (and friends) env vars that the acceptance
# tests already need.

set -euo pipefail

cd "$(dirname "$0")/.."

PKG="${PKG:-./internal/provider/}"
RAW_LOG="$(mktemp -t tc-test-XXXXXX.jsonl)"
trap 'rm -f "$RAW_LOG"' EXIT

echo ">> running: go test -json -timeout 120m $PKG $* (TF_ACC=${TF_ACC:-unset}, TEMPORALCLOUD_TRACE=${TEMPORALCLOUD_TRACE:-unset})"
echo ">> raw json log: $RAW_LOG"

# Stream trace lines live to stderr while capturing the full json log for parsing.
set +e
go test -json -timeout 120m -v "$PKG" "$@" | tee "$RAW_LOG" \
  | grep --line-buffered -E '\[temporalcloud-trace\]|--- (PASS|FAIL)' >&2
STATUS=${PIPESTATUS[0]}
set -e

echo
echo "================ slowest tests ================"
# Each "pass"/"fail" action line in -json carries an Elapsed (seconds) field for
# its Test. Extract Test+Elapsed and sort descending.
grep -E '"Action":"(pass|fail)"' "$RAW_LOG" \
  | grep '"Test":' \
  | sed -E 's/.*"Test":"([^"]+)".*"Elapsed":([0-9.]+).*/\2\t\1/' \
  | sort -rn \
  | awk '{ printf "%8.2fs  %s\n", $1, $2 }' \
  | head -40

echo
echo ">> exit status: $STATUS"
exit "$STATUS"
