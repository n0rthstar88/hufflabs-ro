#!/usr/bin/env bash
# Switches Prontera to another season everywhere: client files (the owner's client folder), the
# server's map cache (map server restart), then publishes the client update to friends.
#   ./season.sh <spring|summer|fall|winter>
#   ./season.sh auto      the season for today's date (spring Mar 20, summer Jun 21, fall Sep 22,
#                         winter Dec 21); does nothing if the server already has it
# Exits 75 (try again later) without changing anything while the game runs on this PC or
# anyone is online.
set -euo pipefail
cd "$(dirname "$0")"
source ../../scripts/settings.sh
need HLRO_PROXMOX HLRO_CT
SEASON=${1:?season or auto}
if [ "$SEASON" = auto ]; then
  md=$(date +%m%d)
  if   [ "$md" -ge 1221 ] || [ "$md" -lt 0320 ]; then SEASON=winter
  elif [ "$md" -lt 0621 ]; then SEASON=spring
  elif [ "$md" -lt 0922 ]; then SEASON=summer
  else SEASON=fall; fi
fi
case $SEASON in spring|summer|fall|winter) ;; *) echo "unknown season $SEASON" >&2; exit 2 ;; esac
run() { ssh -o BatchMode=yes "$HLRO_PROXMOX" "pct exec $HLRO_CT -- bash -c $(printf '%q' "$1")"; }

current=$(run "cat /home/rathena/rathena/db/import/hlro_season.txt 2>/dev/null || true")
if [ "$current" = "$SEASON" ] && [ "${1}" = auto ]; then echo "already $SEASON"; exit 0; fi
if pgrep -f 'HuffLabsRO\.exe' >/dev/null; then echo "the game is running here; try later" >&2; exit 75; fi
online=$(run "mariadb -N ragnarok -e 'SELECT COUNT(*) FROM \`char\` WHERE online=1'")
if [ "$online" != 0 ]; then echo "$online player(s) online; try later" >&2; exit 75; fi

echo "switching Prontera: ${current:-?} -> $SEASON"
CLIENT=${HLRO_CLIENT:-$HOME/Games/Ragnarok/Client}
PICKS=${HLRO_PICKS:-$HOME/Games/Ragnarok/custom-downloads/picks}
work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
go build -C ../.. -o "$work/grftool" ./cmd/grftool
"$work/grftool" list "$CLIENT/data.grf" > "$work/grf.txt"
python3 -I build-client.py "$PICKS" "$CLIENT" "$work/grf.txt" "$SEASON"
./deploy.sh "$SEASON" --restart
(cd "$CLIENT" && hufflabs-ro-publish | grep -v hashed)
echo "Prontera is now $SEASON"
