#!/bin/bash
# Tests swap-release.sh in a temp folder. Run: bash client/test-swap-release.sh
set -uo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
D=$(mktemp -d)
trap 'rm -rf "$D"' EXIT
fail=0
check() { if eval "$2"; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi; }
cd "$D"
echo old > HuffLabsRO-client-1.zip
sha256sum HuffLabsRO-client-1.zip > SHA256SUMS
echo 1 > VERSION
# upload of version 2 has landed: the zip and its checksum file, VERSION still 1
echo new > HuffLabsRO-client-2.zip
sha256sum HuffLabsRO-client-2.zip > SHA256SUMS.new

# a hook that runs between steps and records what an installer would see
export SWAP_HOOK='echo "$1" >> hook.ran; v=$(cat VERSION); grep -q "HuffLabsRO-client-$v.zip" SHA256SUMS && [ -f "HuffLabsRO-client-$v.zip" ] || echo "BROKEN at $1 (VERSION=$v)" >> hook.log'
bash "$HERE/swap-release.sh" "$D" 2; rc=$?
check "swap exits 0" '[ $rc = 0 ]'
check "VERSION is 2" '[ "$(cat VERSION)" = 2 ]'
check "only the new checksum is left" '[ "$(wc -l < SHA256SUMS)" = 1 ] && grep -q client-2.zip SHA256SUMS'
check "old zip removed" '[ ! -e HuffLabsRO-client-1.zip ] && [ -e HuffLabsRO-client-2.zip ]'
check "no SHA256SUMS.new left" '[ ! -e SHA256SUMS.new ]'
check "VERSION always had a checksum and zip" '[ ! -s hook.log ] || { cat hook.log; false; }'
check "hook actually ran" 'grep -q . hook.ran'

# a damaged upload changes nothing
echo bad > HuffLabsRO-client-3.zip
echo "0000000000000000000000000000000000000000000000000000000000000000  HuffLabsRO-client-3.zip" > SHA256SUMS.new
bash "$HERE/swap-release.sh" "$D" 3 >/dev/null 2>&1; rc=$?
check "bad upload fails" '[ $rc != 0 ]'
check "bad upload leaves VERSION and sums" '[ "$(cat VERSION)" = 2 ] && grep -q client-2.zip SHA256SUMS && ! grep -q client-3 SHA256SUMS'

[ $fail = 0 ] && echo "ALL PASS" || { echo "SOME FAILED"; exit 1; }
