#!/usr/bin/env bash
# Installs world round 1 on the game server (container $HLRO_CT).
#   ./deploy.sh            copy scripts/db/config and apply the source patch (no build, no restart)
#   ./deploy.sh --build    ...and rebuild rAthena (as user rathena, make server -j2)
#   ./deploy.sh --restart  ...and restart login, char and map (refuses while anyone is online)
# Safe to run again: the patch is applied once, files are replaced, config lines added once.
set -euo pipefail
cd "$(dirname "$0")"
source ../../scripts/settings.sh
need HLRO_PROXMOX HLRO_CT
PVE="$HLRO_PROXMOX"; CT="$HLRO_CT"; RA=/home/rathena/rathena
BUILD=0; RESTART=0
for a in "$@"; do case $a in --build) BUILD=1 ;; --restart) RESTART=1 ;; *) echo "unknown option $a" >&2; exit 2 ;; esac; done
run() { ssh -o BatchMode=yes "$PVE" "pct exec $CT -- bash -c $(printf '%q' "$1")"; }
put() { ssh -o BatchMode=yes "$PVE" "pct exec $CT -- tee $1 >/dev/null" < "$2"; }
stamp=$(date +%Y%m%d-%H%M%S)
addline() { # addline <file> <line>: append once, backing the file up first
  run "cd $RA; grep -qxF '$2' $1 || { cp -a $1 $1.bak-$stamp; echo '$2' >> $1; echo '$1: added $2'; }"
}

# 1. source patch: charms, element view, MVP tombs, card announce
put /tmp/hlro-world1.patch rathena.patch
run "set -e; cd $RA; G='git -c safe.directory=*'
if \$G apply -R --check /tmp/hlro-world1.patch 2>/dev/null; then echo 'patch: already applied'
else \$G apply --check /tmp/hlro-world1.patch; \$G apply /tmp/hlro-world1.patch; \$G apply --numstat /tmp/hlro-world1.patch | cut -f3 | xargs chown rathena:rathena; echo 'patch: applied'; fi
rm -f /tmp/hlro-world1.patch"

# 2. db imports (instance, quest, refine broadcast)
for f in server/db/import/*.yml; do
  b=$(basename "$f")
  run "cd $RA; [ -f db/import/$b ] && cp -a db/import/$b db/import/$b.bak-$stamp || true"
  put "$RA/db/import/$b" "$f"
  run "chown rathena:rathena $RA/db/import/$b"
  echo "db/import/$b: installed"
done

# 3. Mini Endless Tower map: 1@mtower is a copy of 1@tower (server cache + index + map list)
put /tmp/hlro-mapcache-alias.py server/mapcache-alias.py
run "set -e; cd $RA; cp -a db/import/map_cache.dat db/import/map_cache.dat.bak-$stamp; python3 -I /tmp/hlro-mapcache-alias.py $RA 1@mtower=1@tower; chown rathena:rathena db/import/map_cache.dat; rm -f /tmp/hlro-mapcache-alias.py"
addline db/import/map_index.txt "1@mtower"
addline conf/import/map_conf.txt "map: 1@mtower"

# 4. stock WoE schedulers off; the WoE controller script runs WoE instead
for f in npc/guild/agit_controller.txt npc/guild2/agit_start_se.txt npc/re/guild3/agit_start_te.txt; do
  addline conf/import/map_conf.txt "delnpc: $f"
done

# 5. Build Manager table
put /tmp/hlro-bm.sql server/build_manager.sql
run "mariadb ragnarok < /tmp/hlro-bm.sql && rm -f /tmp/hlro-bm.sql && echo 'ero_buildmanager: ready'"

# 6. npc scripts (and the leveling @menu, which lists the new commands and NPCs)
run "mkdir -p $RA/npc/custom/hufflabs"
for f in server/npc/*.txt ../leveling/server/npc/menu.txt; do
  b=$(basename "$f")
  put "$RA/npc/custom/hufflabs/$b" "$f"
  run "chown rathena:rathena $RA/npc/custom/hufflabs/$b"
  addline conf/import/map_conf.txt "npc: npc/custom/hufflabs/$b"
  echo "npc/custom/hufflabs/$b: installed"
done

# 7. build
if [ $BUILD = 1 ]; then
  echo "building rAthena (takes a while)..."
  if ! run "cd $RA && su rathena -s /bin/bash -c 'make server -j2 > /tmp/hlro-build.log 2>&1'"; then
    run "tail -40 /tmp/hlro-build.log"; echo "BUILD FAILED; servers not restarted" >&2; exit 1
  fi
  run "tail -3 /tmp/hlro-build.log; cd $RA; echo map-server mob_ele_view: \$(strings map-server | grep -c mob_ele_view)"
fi

# 8. restart
if [ $RESTART = 1 ]; then
  online=$(run "mariadb -N ragnarok -e 'SELECT COUNT(*) FROM \`char\` WHERE online=1'")
  if [ "$online" != 0 ]; then echo "$online player(s) online; not restarting" >&2; exit 1; fi
  run "systemctl restart rathena-login; sleep 3; systemctl restart rathena-char; sleep 5; systemctl restart rathena-map; sleep 30
for u in rathena-login rathena-char rathena-map; do echo \$u: \$(systemctl is-active \$u); done
journalctl -u rathena-map -u rathena-char --since '-50s' --no-pager | grep -iE 'error|warning' | grep -v roulette | tail -40"
fi
