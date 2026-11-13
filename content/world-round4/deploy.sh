#!/usr/bin/env bash
# Installs world round 4 (extra dyes + costume headgears) on the game server.
#   ./deploy.sh            copy files and set the dye limits (no restart)
#   ./deploy.sh --restart  ...and restart the map server (refuses while anyone is online)
# Needs content/coin-shop deployed (its item_db.yml block imports item_db_headgears.yml),
# content/world-round1 (Stylist) and content/leveling (@menu) re-deployed with this round's changes.
set -euo pipefail
cd "$(dirname "$0")"
source ../../scripts/settings.sh
need HLRO_PROXMOX HLRO_CT
PVE="$HLRO_PROXMOX"; CT="$HLRO_CT"; RA=/home/rathena/rathena
RESTART=0
for a in "$@"; do case $a in --restart) RESTART=1 ;; *) echo "unknown option $a" >&2; exit 2 ;; esac; done
run() { ssh -o BatchMode=yes "$PVE" "pct exec $CT -- bash -c $(printf '%q' "$1")"; }
put() { ssh -o BatchMode=yes "$PVE" "pct exec $CT -- tee $1 >/dev/null" < "$2"; }
stamp=$(date +%Y%m%d-%H%M%S)

python3 -I build-server.py server >/dev/null
put "$RA/db/import/item_db_headgears.yml" server/db/import/item_db_headgears.yml
run "chown rathena:rathena $RA/db/import/item_db_headgears.yml"
for f in server/npc/*.txt ../world-round1/server/npc/adv_stylist.txt ../leveling/server/npc/menu.txt; do
  b=$(basename "$f")
  put "$RA/npc/custom/hufflabs/$b" "$f"; run "chown rathena:rathena $RA/npc/custom/hufflabs/$b"
  run "cd $RA; grep -qxF 'npc: npc/custom/hufflabs/$b' conf/import/map_conf.txt || { cp -a conf/import/map_conf.txt conf/import/map_conf.txt.bak-$stamp; echo 'npc: npc/custom/hufflabs/$b' >> conf/import/map_conf.txt; echo 'map_conf: added $b'; }"
done
# dye limits (the Stylist narrows them per character, see server/npc/dyes.txt)
run "set -e; cd $RA; f=conf/import/battle_conf.txt; cp -a \$f \$f.bak-$stamp
python3 - <<'PY'
p='conf/import/battle_conf.txt'; s=open(p).read()
a='// >>> HuffLabs dyes'; b='// <<< HuffLabs dyes'
if a in s: s=s[:s.index(a)].rstrip('\n')+'\n'+s[s.index(b)+len(b):].lstrip('\n')
s=s.rstrip('\n')+'\n\n'+a+'\nmax_hair_color: 251\nmax_cloth_color: 553\n'+b+'\n'
open(p,'w').write(s)
PY
chown rathena:rathena \$f; echo 'battle_conf: dye limits set'"
echo "round 4 files installed"

if [ $RESTART = 1 ]; then
  online=$(run "mariadb -N ragnarok -e 'SELECT COUNT(*) FROM \`char\` WHERE online=1'")
  if [ "$online" != 0 ]; then echo "$online player(s) online; not restarting" >&2; exit 1; fi
  run "systemctl restart rathena-map; sleep 30; echo rathena-map: \$(systemctl is-active rathena-map)
pid=\$(systemctl show -p MainPID --value rathena-map); journalctl _PID=\$pid --no-pager | grep -iE 'error|warning' | grep -viE 'roulette|npc_parse_shop|Connection to' | tail -40"
fi
