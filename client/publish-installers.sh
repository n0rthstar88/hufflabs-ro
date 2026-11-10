#!/bin/bash
# Uploads the installers and the launcher to the download page without rebuilding
# the 4 GB zip. The launcher's checksum line is merged into SHA256SUMS through
# swap-release.sh, so there's never a moment without a checksum.
# Usage: client/publish-installers.sh   (after launcher/build.sh and makensis)
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/settings.sh
need HLRO_SITE_URL HLRO_PROXMOX HLRO_CT
PROXMOX=$HLRO_PROXMOX CT=$HLRO_CT
DL=/srv/hufflabs-portal/download
cp build/launcher/HuffLabsRO-Launcher.exe dist/
sed "s#__SITE_URL__#$HLRO_SITE_URL#" installer/linux/hufflabs-ro-install.sh > dist/hufflabs-ro-install.sh
FILES=(dist/HuffLabsRO-Launcher.exe dist/hufflabs-ro-install.sh)
[ -f dist/HuffLabsRO-Setup.exe ] && FILES+=(dist/HuffLabsRO-Setup.exe)
push() { # local-file target-path
	scp -q -o BatchMode=yes "$1" "$PROXMOX:/root/$(basename "$1")"
	ssh -o BatchMode=yes "$PROXMOX" "pct push $CT /root/$(basename "$1") $2 --perms 0644 && rm /root/$(basename "$1")"
}
for f in "${FILES[@]}"; do push "$f" "$DL/$(basename "$f")"; done
LSUM=$(sha256sum dist/HuffLabsRO-Launcher.exe | cut -d' ' -f1)
push client/swap-release.sh /root/swap-release.sh
# SHA256SUMS.new = the live zip's line + the new launcher line; same VERSION
ssh -o BatchMode=yes "$PROXMOX" "pct exec $CT -- bash -c 'cd $DL && V=\$(cat VERSION) && grep \" HuffLabsRO-client-\$V.zip\$\" SHA256SUMS > SHA256SUMS.new && echo \"$LSUM  HuffLabsRO-Launcher.exe\" >> SHA256SUMS.new && bash /root/swap-release.sh $DL \$V; rc=\$?; rm -f /root/swap-release.sh; chown -R root:portal $DL; ls -la $DL; exit \$rc'"
