#!/usr/bin/env bash
# Installs world round 3 (Prontera Seasons, Poring Family, NPC looks/townsfolk) on the game server.
#   ./deploy.sh <season>            copy files and set Prontera's map cache for <season> (no restart)
#   ./deploy.sh <season> --restart  ...and restart the map server (refuses while anyone is online)
# <season>: spring | summer | fall | winter. The client side is build-client.py (same season!).
# Needs content/leveling's db/import/mob_db.yml (its Footer imports mob_db_porings.yml) and
# content/world-round1 (the NPC faces live in its scripts).
set -euo pipefail
cd "$(dirname "$0")"
source ../../scripts/settings.sh
need HLRO_PROXMOX HLRO_CT
PVE="$HLRO_PROXMOX"; CT="$HLRO_CT"; RA=/home/rathena/rathena
SEASON=${1:?season: spring, summer, fall or winter}; shift
case $SEASON in spring|summer|fall|winter) ;; *) echo "unknown season $SEASON" >&2; exit 2 ;; esac
RESTART=0
for a in "$@"; do case $a in --restart) RESTART=1 ;; *) echo "unknown option $a" >&2; exit 2 ;; esac; done
run() { ssh -o BatchMode=yes "$PVE" "pct exec $CT -- bash -c $(printf '%q' "$1")"; }
put() { ssh -o BatchMode=yes "$PVE" "pct exec $CT -- tee $1 >/dev/null" < "$2"; }
stamp=$(date +%Y%m%d-%H%M%S)
PICKS=${HLRO_PICKS:-$HOME/Games/Ragnarok/custom-downloads/picks}

python3 -I build-server.py server >/dev/null

# 1. Prontera's walkable cells for this season (the season's prontera.gat into the import map cache)
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
python3 -I - "$PICKS/prontera-seasons/prontera_seasons_v3-3.zip" "$SEASON" "$tmp/prontera.gat" <<'PY'
import sys, zipfile
z = zipfile.ZipFile(sys.argv[1])
want = f"_{sys.argv[2]}/data/prontera.gat"
hits = [i for i in z.infolist() if i.filename.endswith(want)]
assert len(hits) == 1, hits
open(sys.argv[3], "wb").write(z.read(hits[0]))
PY
put /tmp/hlro-prontera.gat "$tmp/prontera.gat"
put /tmp/hlro-mapcache-gat.py server/mapcache-gat.py
run "set -e; cd $RA; cp -a db/import/map_cache.dat db/import/map_cache.dat.bak-$stamp; python3 -I /tmp/hlro-mapcache-gat.py $RA prontera /tmp/hlro-prontera.gat; chown rathena:rathena db/import/map_cache.dat; rm -f /tmp/hlro-prontera.gat /tmp/hlro-mapcache-gat.py"
run "cd $RA; echo $SEASON > db/import/hlro_season.txt"

# 2. monsters + scripts
for f in server/db/import/mob_db_porings.yml ../leveling/server/db/import/mob_db.yml; do
  b=$(basename "$f")
  run "cd $RA; [ -f db/import/$b ] && cp -a db/import/$b db/import/$b.bak-$stamp || true"
  put "$RA/db/import/$b" "$f"; run "chown rathena:rathena $RA/db/import/$b"; echo "db/import/$b: installed"
done
for f in server/npc/*.txt ../world-round1/server/npc/*.txt; do
  b=$(basename "$f")
  put "$RA/npc/custom/hufflabs/$b" "$f"; run "chown rathena:rathena $RA/npc/custom/hufflabs/$b"
  run "cd $RA; grep -qxF 'npc: npc/custom/hufflabs/$b' conf/import/map_conf.txt || { cp -a conf/import/map_conf.txt conf/import/map_conf.txt.bak-$stamp; echo 'npc: npc/custom/hufflabs/$b' >> conf/import/map_conf.txt; echo 'map_conf: added $b'; }"
done
echo "scripts installed"

# 3. restart (the map cache is read at start)
if [ $RESTART = 1 ]; then
  online=$(run "mariadb -N ragnarok -e 'SELECT COUNT(*) FROM \`char\` WHERE online=1'")
  if [ "$online" != 0 ]; then echo "$online player(s) online; not restarting" >&2; exit 1; fi
  run "systemctl restart rathena-map; sleep 30; echo rathena-map: \$(systemctl is-active rathena-map)
journalctl -u rathena-map --since '-40s' --no-pager | grep -iE 'error|warning' | grep -viE 'roulette|npc_parse_shop|Connection to' | tail -40"
fi
