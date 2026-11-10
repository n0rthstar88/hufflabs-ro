#!/bin/bash
# Runs on the server after a release upload: switches the download folder to
# <version> without a moment where VERSION names a zip that has no checksum.
# Usage: swap-release.sh <download dir> <version>
# Expects HuffLabsRO-client-<version>.zip and its SHA256SUMS.new already uploaded.
set -euo pipefail
cd "${1:?download dir}"
VERSION=${2:?version}
NAME="HuffLabsRO-client-$VERSION.zip"
hook() { [ -z "${SWAP_HOOK:-}" ] || bash -c "$SWAP_HOOK" _ "$1"; }   # tests only

hook start
sha256sum -c --quiet SHA256SUMS.new
grep -q " $NAME\$" SHA256SUMS.new
# 1. checksums for old and new, so either VERSION is covered
[ -f SHA256SUMS ] || : > SHA256SUMS
awk '!seen[$2]++' SHA256SUMS.new SHA256SUMS 2>/dev/null > SHA256SUMS.tmp
mv SHA256SUMS.tmp SHA256SUMS
hook both
# 2. point VERSION at the new zip
printf '%s\n' "$VERSION" > VERSION.tmp
mv VERSION.tmp VERSION
hook version
# 3. drop the old checksum and zips
mv SHA256SUMS.new SHA256SUMS
hook sums
find . -maxdepth 1 -name 'HuffLabsRO-client-*.zip' ! -name "$NAME" -delete
hook done
