#!/usr/bin/env bash
# Prontera's season is switched by the server itself (server/season/hlro-season-switch.sh on an
# hourly timer, from the four season manifests every publish signs), so friends get it even
# when this PC is off. This script only keeps the publisher's own client folder in step.
#   ./season.sh auto        make this PC's client match the server's live season
#   ./season.sh <season>    switch the server now (waits for an empty server), then this PC
#   ./season.sh status      show the server's live season and today's season
# <season>: spring | summer | fall | winter. Exits 75 (try again later) while the game runs here.
set -euo pipefail
cd "$(dirname "$0")"
source ../../scripts/settings.sh
need HLRO_PROXMOX HLRO_CT
ARG=${1:?auto, status or a season}
run() { ssh -o BatchMode=yes "$HLRO_PROXMOX" "pct exec $HLRO_CT -- bash -c $(printf '%q' "$1")"; }
CLIENT=${HLRO_CLIENT:-$HOME/Games/Ragnarok/Client}
SETS=${HLRO_SEASONS:-$HOME/Games/Ragnarok/season-sets}

case $ARG in
  status) run "/usr/local/sbin/hlro-season-switch.sh --status"; exit 0 ;;
  spring|summer|fall|winter) run "/usr/local/sbin/hlro-season-switch.sh $ARG" ;;
  auto) ;;
  *) echo "unknown argument $ARG" >&2; exit 2 ;;
esac
SEASON=$(run "cat /srv/hufflabs-portal/update/seasons/current")

# which season does this PC's client hold? (the set whose files all match)
have=$(python3 -I - "$CLIENT" "$SETS" <<'PY'
import filecmp, pathlib, sys
client, sets = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2])
for s in ("spring", "summer", "fall", "winter"):
    files = [f for f in (sets / s).rglob("*") if f.is_file()]
    if files and all((client / f.relative_to(sets / s)).is_file() and
                     filecmp.cmp(f, client / f.relative_to(sets / s), shallow=False) for f in files):
        print(s); break
else:
    print("unknown")
PY
)
if [ "$have" = "$SEASON" ]; then echo "this PC already has $SEASON"; exit 0; fi
if pgrep -f 'HuffLabsRO\.exe' >/dev/null; then echo "the game is running here; try later" >&2; exit 75; fi

echo "this PC's client: $have -> $SEASON"
# remove the other seasons' files, then copy this season's (same names the publisher uses)
python3 -I - "$CLIENT" "$SETS" "$SEASON" <<'PY'
import pathlib, shutil, sys
client, sets, want = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2]), sys.argv[3]
mine = {f.relative_to(sets / want).as_posix().lower() for f in (sets / want).rglob("*") if f.is_file()}
for s in ("spring", "summer", "fall", "winter"):
    for f in (sets / s).rglob("*"):
        rel = f.relative_to(sets / s)
        if f.is_file() and rel.as_posix().lower() not in mine and (client / rel).is_file():
            (client / rel).unlink()
for f in (sets / want).rglob("*"):
    if f.is_file():
        dst = client / f.relative_to(sets / want)
        dst.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(f, dst)
PY
echo "this PC's client is now $SEASON"
