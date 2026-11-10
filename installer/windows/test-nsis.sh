#!/bin/bash
# Builds test-nsis.nsi and runs it under Wine (needs makensis and wine).
set -euo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
cd "$HERE"
W=$(mktemp -d)
trap 'rm -rf "$W"' EXIT
makensis -V2 "-DOUT=$W/test-nsis.exe" "$HERE/test-nsis.nsi" >/dev/null 2>"$W/build.err" || { cat "$W/build.err"; exit 1; }
WINEPREFIX=${WINEPREFIX:-$HOME/.cache/hufflabs-ro-test-wine} WINEDEBUG=-all wine "$W/test-nsis.exe" >/dev/null 2>&1 || true
[ -f "$W/results.txt" ] || { echo "no results (did the test exe run?)"; exit 1; }
tr -d '\r' < "$W/results.txt"
grep -q "^ALL PASS" "$W/results.txt"
