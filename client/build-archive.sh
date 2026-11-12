#!/bin/bash
# Builds the HuffLabs RO client archive and publishes it to the portal's download folder.
# Usage: client/build-archive.sh <version, e.g. 2026.11.09> [--no-publish]
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION=${1:?version, e.g. 2026.11.09}
PUBLISH=1
[ "${2:-}" = --no-publish ] && PUBLISH=0
CLIENT=${CLIENT:-$HOME/Games/Ragnarok/Client}
. scripts/settings.sh
need HLRO_SITE_URL
SITE_URL=$HLRO_SITE_URL
DL=/srv/hufflabs-portal/download
STAGE=build/stage/HuffLabsRO
ZIP=dist/HuffLabsRO-client-$VERSION.zip

[ -f theme/out/HuffLabsRO.ico ] || { echo "run theme/build-theme.sh first" >&2; exit 1; }
mkdir -p dist build/stage

echo "== staging"
# Everything players need, minus client/excludes.txt (the WARP patcher, WARPGATE
# state, the unpatched exe, backups and personal settings; shared with
# hufflabs-ro-publish).
rsync -a --delete --exclude-from=client/excludes.txt \
	"$CLIENT/" "$STAGE/"
rsync -a theme/out/data/ "$STAGE/data/"
cp theme/out/HuffLabsRO.ico theme/out/HuffLabsRO.png "$STAGE/"
# Default settings the installers copy in (OptionInfo.lua, settings.reg).
mkdir -p "$STAGE/defaults" && cp client/defaults/OptionInfo.lua client/defaults/settings.reg "$STAGE/defaults/"
# The Linux installer is published next to the zip.

[ -f theme/out/HuffLabsRO.exe ] && cp theme/out/HuffLabsRO.exe "$STAGE/HuffLabsRO.exe"
printf '[Data]\r\n1=data.grf\r\n' > "$STAGE/DATA.INI"
printf '%s\n' "$VERSION" > "$STAGE/VERSION"

echo "== in-game Sign Up button -> $SITE_URL/"
# clientinfo files are EUC-KR; only ASCII bytes inside the tag change.
for f in clientinfo.xml sclientinfo.xml; do
	python3 -I - "$STAGE/data/$f" "$SITE_URL/" <<'PY'
import re, sys
p, url = sys.argv[1], sys.argv[2].encode()
b = open(p, 'rb').read()
b, n = re.subn(rb'<registrationweb>[^<]*</registrationweb>', b'<registrationweb>' + url + b'</registrationweb>', b)
assert n == 1, (p, n)
open(p, 'wb').write(b)
PY
done

echo "== zip"
rm -f "$ZIP"
(cd build/stage && zip -q -r -X "../../$ZIP" HuffLabsRO)
# Plain zip tops out at 4 GiB (the client is close to it); past that zip switches to
# Zip64 on its own. Either way, check that Linux unzip and Windows' tar (bsdtar)
# both read the archive before it goes out.
SIZE=$(stat -c %s "$ZIP")
if [ "$SIZE" -ge $((4 * 1024 * 1024 * 1024 - 64 * 1024 * 1024)) ]; then
	echo "note: $ZIP is $SIZE bytes, at or past the 4 GiB zip limit, so it uses Zip64"
fi
unzip -tq "$ZIP"
# Not a Windows check: Windows tar.exe fails on the Korean names (non-UTF-8 code page), so
# the Windows installer unpacks with .NET ZipFile instead. This only checks bsdtar can read it.
if command -v bsdtar >/dev/null; then bsdtar -tf "$ZIP" >/dev/null; fi
[ -f build/launcher/HuffLabsRO-Launcher.exe ] || { echo "run launcher/build.sh first" >&2; exit 1; }
cp build/launcher/HuffLabsRO-Launcher.exe dist/
(cd dist && sha256sum "$(basename "$ZIP")" HuffLabsRO-Launcher.exe > SHA256SUMS && printf '%s\n' "$VERSION" > VERSION)
ls -la "$ZIP"; cat dist/SHA256SUMS

[ "$PUBLISH" = 1 ] || exit 0
need HLRO_PROXMOX HLRO_CT
PROXMOX=$HLRO_PROXMOX CT=$HLRO_CT
echo "== publish to LXC $CT:$DL"
sed "s#__SITE_URL__#$SITE_URL#" installer/linux/hufflabs-ro-install.sh > dist/hufflabs-ro-install.sh
EXTRA=()
[ -f dist/HuffLabsRO-Setup.exe ] && EXTRA+=(dist/HuffLabsRO-Setup.exe)  # built with makensis
push() { # local-file target-path
	scp -q -o BatchMode=yes "$1" "$PROXMOX:/root/$(basename "$1")"
	ssh -o BatchMode=yes "$PROXMOX" "pct push $CT /root/$(basename "$1") $2 --perms 0644 && rm /root/$(basename "$1")"
}
for f in "$ZIP" dist/hufflabs-ro-install.sh dist/HuffLabsRO-Launcher.exe "${EXTRA[@]}"; do push "$f" "$DL/$(basename "$f")"; done
push dist/SHA256SUMS "$DL/SHA256SUMS.new"
push client/swap-release.sh /root/swap-release.sh
# Verify on the server, then switch VERSION over (see swap-release.sh) and drop old zips.
ssh -o BatchMode=yes "$PROXMOX" "pct exec $CT -- bash -c 'bash /root/swap-release.sh $DL $VERSION; rc=\$?; rm -f /root/swap-release.sh; chown -R root:portal $DL && ls -la $DL; exit \$rc'"
curl -sf -o /dev/null -w "site serves SHA256SUMS: %{http_code}\n" "$SITE_URL/download/SHA256SUMS"
