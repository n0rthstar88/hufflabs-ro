#!/usr/bin/env bash
# Installs the leveling project on the game server container (HLRO_CT).
#   ./deploy.sh            copy data/config/scripts and apply the source patch (no build, no restart)
#   ./deploy.sh --build    ...and rebuild rAthena (as user rathena, make server -j2)
#   ./deploy.sh --restart  ...and restart login, char and map (refuses while anyone is online)
# Safe to run again: the patch is applied once, config blocks are replaced, npc lines added once.
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

# 1. source patch (applied once; already-applied is detected by a reverse check)
put /tmp/hlro-leveling.patch rathena.patch
run "set -e; cd $RA; G='git -c safe.directory=*'
if \$G apply -R --check /tmp/hlro-leveling.patch 2>/dev/null; then echo 'patch: already applied'
else \$G apply --check /tmp/hlro-leveling.patch; \$G apply /tmp/hlro-leveling.patch; \$G apply --numstat /tmp/hlro-leveling.patch | cut -f3 | xargs chown rathena:rathena; echo 'patch: applied'; fi
rm -f /tmp/hlro-leveling.patch"

# 2. db/import files
if compgen -G 'server/db/import/*.yml' >/dev/null; then
  for f in server/db/import/*.yml; do
    b=$(basename "$f")
    run "cd $RA; [ -f db/import/$b ] && cp -a db/import/$b db/import/$b.bak-$stamp || true"
    put "$RA/db/import/$b" "$f"
    run "chown rathena:rathena $RA/db/import/$b"
    echo "db/import/$b: installed"
  done
fi

# 3. npc scripts
if compgen -G 'server/npc/*.txt' >/dev/null; then
  run "mkdir -p $RA/npc/custom/hufflabs"
  for f in server/npc/*.txt; do
    b=$(basename "$f")
    put "$RA/npc/custom/hufflabs/$b" "$f"
    run "cd $RA; chown rathena:rathena npc/custom/hufflabs/$b; grep -qx 'npc: npc/custom/hufflabs/$b' conf/import/map_conf.txt || { cp -a conf/import/map_conf.txt conf/import/map_conf.txt.bak-$stamp; echo 'npc: npc/custom/hufflabs/$b' >> conf/import/map_conf.txt; }"
    echo "npc/custom/hufflabs/$b: installed"
  done
fi
# stock rAthena healer in every town (healer_extra.txt adds Lasagna and needs it loaded first)
run "cd $RA; grep -qx 'npc: npc/custom/healer.txt' conf/import/map_conf.txt || sed -i '\\|^npc: npc/custom/hufflabs/healer_extra.txt|i npc: npc/custom/healer.txt' conf/import/map_conf.txt; grep -qx 'npc: npc/custom/healer.txt' conf/import/map_conf.txt || echo 'npc: npc/custom/healer.txt' >> conf/import/map_conf.txt"

# 4. battle config block (between the HuffLabs leveling markers, replaced each time)
put /tmp/hlro-battle.txt server/battle_conf.txt
run "set -e; cd $RA; f=conf/import/battle_conf.txt; cp -a \$f \$f.bak-$stamp
python3 - <<'PY'
p='conf/import/battle_conf.txt'; s=open(p).read()
a='// >>> HuffLabs leveling'; b='// <<< HuffLabs leveling'
if a in s: s=s[:s.index(a)].rstrip('\n')+'\n'+s[s.index(b)+len(b):].lstrip('\n')
s=s.rstrip('\n')+'\n\n'+a+'\n'+open('/tmp/hlro-battle.txt').read().rstrip('\n')+'\n'+b+'\n'
open(p,'w').write(s)
PY
rm -f /tmp/hlro-battle.txt; chown rathena:rathena \$f; echo 'battle_conf: block replaced'"

# 5. build
if [ $BUILD = 1 ]; then
  echo "building rAthena (takes a while)..."
  if ! run "cd $RA && su rathena -s /bin/bash -c 'make server -j2 > /tmp/hlro-build.log 2>&1'"; then
    run "tail -40 /tmp/hlro-build.log"; echo "BUILD FAILED; servers not restarted" >&2; exit 1
  fi
  run "tail -3 /tmp/hlro-build.log; cd $RA; echo map-server HLRO_PACE strings: \$(strings map-server | grep -c HLRO_PACE)"
fi

# 6. restart
if [ $RESTART = 1 ]; then
  online=$(run "mariadb -N ragnarok -e 'SELECT COUNT(*) FROM \`char\` WHERE online=1'")
  if [ "$online" != 0 ]; then echo "$online player(s) online; not restarting" >&2; exit 1; fi
  run "systemctl restart rathena-login; sleep 3; systemctl restart rathena-char; sleep 5; systemctl restart rathena-map; sleep 25
for u in rathena-login rathena-char rathena-map; do echo \$u: \$(systemctl is-active \$u); done
journalctl -u rathena-map -u rathena-char --since '-45s' --no-pager | grep -iE 'error|warning' | grep -v roulette | tail -30"
fi
