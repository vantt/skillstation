#!/bin/sh
# Fails when a function that no binary reaches is not in scripts/deadcode-baseline.txt.
# The baseline lists existing debt as "package function" lines (no line numbers) and
# should only shrink. Refresh it with: scripts/deadcode-check.sh --update
set -eu
cd "$(dirname "$0")/.."
baseline=scripts/deadcode-baseline.txt
current="$(mktemp)"
trap 'rm -f "$current"' EXIT
go run golang.org/x/tools/cmd/deadcode@v0.38.0 ./cmd/... \
  | sed -E 's/^([^:]+):[0-9]+:[0-9]+: unreachable func: /\1 /' | LC_ALL=C sort -u > "$current"
if [ "${1:-}" = "--update" ]; then
  cp "$current" "$baseline"
  echo "baseline updated: $(wc -l < "$baseline") functions"
  exit 0
fi
added="$(LC_ALL=C comm -13 "$baseline" "$current")"
if [ -n "$added" ]; then
  echo "unreachable functions not in $baseline (remove them, or use them from a command):"
  echo "$added"
  exit 1
fi
echo "deadcode: no new unreachable functions ($(wc -l < "$current") in the baseline debt)"
