#!/usr/bin/env bash
# Puts the coin shop on the game server container (HLRO_CT) and restarts the map server.
# Safe to run again: it replaces the HuffLabs block instead of adding a second one.
set -euo pipefail
cd "$(dirname "$0")"
source ../../scripts/settings.sh
need HLRO_PROXMOX HLRO_CT; PVE="$HLRO_PROXMOX"; CT="$HLRO_CT"
RA=/home/rathena/rathena
run() { ssh -o BatchMode=yes "$PVE" "pct exec $CT -- bash -c $(printf '%q' "$1")"; }
put() { ssh -o BatchMode=yes "$PVE" "pct exec $CT -- tee $1 >/dev/null" < "$2"; }

./build-shop.sh
stamp=$(date +%Y%m%d-%H%M%S)
run "set -e; cd $RA; cp -a db/import/item_db.yml db/import/item_db.yml.bak-$stamp; cp -a conf/import/map_conf.txt conf/import/map_conf.txt.bak-$stamp; mkdir -p npc/custom/hufflabs"
put /tmp/hlro-items.yml server/item_db.yml
put $RA/npc/custom/hufflabs/coin_drops.txt server/coin_drops.txt
put $RA/npc/custom/hufflabs/coin_shop.txt server/coin_shop.txt
run "set -e; cd $RA
# item_db: drop any earlier HuffLabs block, then append the new one
python3 - <<'PY'
p='db/import/item_db.yml'; s=open(p).read()
m='###########################################################################\n# HuffLabs RO: coin shop items'
if m in s: s=s[:s.index(m)].rstrip('\n')+'\n'
open(p,'w').write(s+'\n'+open('/tmp/hlro-items.yml').read())
PY
rm -f /tmp/hlro-items.yml
for f in coin_drops coin_shop; do grep -qx \"npc: npc/custom/hufflabs/\$f.txt\" conf/import/map_conf.txt || echo \"npc: npc/custom/hufflabs/\$f.txt\" >> conf/import/map_conf.txt; done
chown -R rathena:rathena npc/custom/hufflabs db/import/item_db.yml conf/import/map_conf.txt
echo online: \$(mariadb -N ragnarok -e 'SELECT COUNT(*) FROM \`char\` WHERE online=1')"
if [[ "${1:-}" == --restart ]]; then
  run "systemctl restart rathena-map; sleep 20; journalctl -u rathena-map --since '-40s' --no-pager | grep -iE 'error|warning|hufflabs|2600000|coin_' | grep -v -i 'Done reading' | tail -40; systemctl is-active rathena-map"
else
  echo "Files are in place. Run again with --restart to restart the map server (kicks anyone online)."
fi
