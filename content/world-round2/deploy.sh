#!/usr/bin/env bash
# Installs world round 2 (vending currency + random option drops) on the game server.
#   ./deploy.sh            copy files, apply the source patch, add the DB column (no build, no restart)
#   ./deploy.sh --build    ...and rebuild rAthena
#   ./deploy.sh --restart  ...and restart login, char and map (refuses while anyone is online)
# Needs content/coin-shop deployed too (items 2600200 "Zeny" and the coin) and content/leveling's
# db/import/mob_db.yml, whose Footer imports mob_db_randopt.yml.
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

# 1. source patch: vending currency
put /tmp/hlro-world2.patch rathena.patch
run "set -e; cd $RA; G='git -c safe.directory=*'
if \$G apply -R --check /tmp/hlro-world2.patch 2>/dev/null; then echo 'patch: already applied'
else \$G apply --check /tmp/hlro-world2.patch; \$G apply /tmp/hlro-world2.patch; \$G apply --numstat /tmp/hlro-world2.patch | cut -f3 | xargs chown rathena:rathena; echo 'patch: applied'; fi
rm -f /tmp/hlro-world2.patch"

# 2. shops that stay open offline remember their currency
run "mariadb ragnarok -e 'ALTER TABLE \`vendings\` ADD COLUMN IF NOT EXISTS \`hlro_currency\` INT UNSIGNED NOT NULL DEFAULT 0' && echo 'vendings.hlro_currency: ready'"

# 3. random option drops (the leveling mob import pulls mob_db_randopt.yml in through its Footer)
for f in server/db/import/item_randomopt_group.yml server/db/import/mob_db_randopt.yml ../leveling/server/db/import/mob_db.yml; do
  b=$(basename "$f")
  run "cd $RA; [ -f db/import/$b ] && cp -a db/import/$b db/import/$b.bak-$stamp || true"
  put "$RA/db/import/$b" "$f"
  run "chown rathena:rathena $RA/db/import/$b"
  echo "db/import/$b: installed"
done

# 4. build
if [ $BUILD = 1 ]; then
  echo "building rAthena (takes a while)..."
  if ! run "cd $RA && su rathena -s /bin/bash -c 'make server -j2 > /tmp/hlro-build.log 2>&1'"; then
    run "tail -40 /tmp/hlro-build.log"; echo "BUILD FAILED; servers not restarted" >&2; exit 1
  fi
  run "tail -3 /tmp/hlro-build.log; cd $RA; echo map-server vending_currency_item: \$(strings map-server | grep -c vending_currency_item)"
fi

# 5. restart
if [ $RESTART = 1 ]; then
  online=$(run "mariadb -N ragnarok -e 'SELECT COUNT(*) FROM \`char\` WHERE online=1'")
  if [ "$online" != 0 ]; then echo "$online player(s) online; not restarting" >&2; exit 1; fi
  run "systemctl restart rathena-login; sleep 3; systemctl restart rathena-char; sleep 5; systemctl restart rathena-map; sleep 30
for u in rathena-login rathena-char rathena-map; do echo \$u: \$(systemctl is-active \$u); done
journalctl -u rathena-map -u rathena-char --since '-50s' --no-pager | grep -iE 'error|warning' | grep -viE 'roulette|npc_parse_shop' | tail -40"
fi
