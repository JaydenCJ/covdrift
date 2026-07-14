#!/usr/bin/env bash
# Minimal PR gate: diff the baseline coverage (from your main branch's
# last run) against the current run and fail on per-file regressions.
# Works the same in any CI system — covdrift is just an exit code.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASELINE="${1:-$HERE/baseline.info}"   # e.g. downloaded from your main-branch artifact
CURRENT="${2:-$HERE/current.xml}"      # e.g. produced by this PR's test run

# Prefer a locally built binary; fall back to PATH.
if [ -x "$HERE/../covdrift" ]; then
  covdrift() { "$HERE/../covdrift" "$@"; }
elif ! command -v covdrift >/dev/null; then
  echo "covdrift not found: run 'go build -o covdrift ./cmd/covdrift' first" >&2
  exit 3
fi

# Human-readable report to the CI log; exit 1 blocks the merge.
if covdrift diff \
    --tolerance 0.5 \
    --min-lines 5 \
    --exclude 'vendor/**' \
    "$BASELINE" "$CURRENT"; then
  echo "coverage gate passed"
else
  status=$?
  # Optionally drop a Markdown summary where your CI surfaces artifacts.
  covdrift diff --no-gate --format markdown "$BASELINE" "$CURRENT" > coverage-diff.md
  echo "coverage gate failed; see coverage-diff.md"
  exit "$status"
fi
