#!/usr/bin/env bash
# Turns on the HuffLabs RO server options on the game server container.
#   ./deploy.sh            install everything (no restart)
#   ./deploy.sh --restart  ...and restart the map server (refuses while anyone is online)
# Safe to run again: every step checks first, and every overwritten file gets a .bak-<time> copy.
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
addline() { # addline <file> <line>: append once, backing the file up first
  run "cd $RA; grep -qxF '$2' $1 || { cp -a $1 $1.bak-$stamp; echo '$2' >> $1; echo '$1: added $2'; }"
}

# 1. battle options: monsters drop zeny, day/night cycle (2 h day, 30 min night)
put /tmp/hlro-opts.txt server/battle_conf.txt
run "set -e; cd $RA; f=conf/import/battle_conf.txt; cp -a \$f \$f.bak-$stamp
python3 - <<'PY'
p='conf/import/battle_conf.txt'; s=open(p).read()
a='// >>> HuffLabs server options'; b='// <<< HuffLabs server options'
if a in s: s=s[:s.index(a)].rstrip('\n')+'\n'+s[s.index(b)+len(b):].lstrip('\n')
s=s.rstrip('\n')+'\n\n'+a+'\n'+open('/tmp/hlro-opts.txt').read().rstrip('\n')+'\n'+b+'\n'
open(p,'w').write(s)
PY
rm -f /tmp/hlro-opts.txt; chown rathena:rathena \$f; echo 'battle_conf: options block replaced'"

# 2. players may use @autoloot and @alootid (group 0)
run "set -e; cd $RA; f=conf/import/groups.yml; cp -a \$f \$f.bak-$stamp
python3 - <<'PY'
p='conf/import/groups.yml'; s=open(p).read()
s=s.replace('      alootid: true\n','')  # an alias, not a command name: groups need autolootitem
for c in ('autoloot','autolootitem'):
    if '      %s: true\n' % c not in s:
        s=s.replace('      go: true\n','      go: true\n      %s: true\n' % c,1)
open(p,'w').write(s)
PY
grep -q '      autolootitem: true' \$f && echo 'groups.yml: @autoloot/@alootid for players'"

# 3. daily attendance rewards (20-day cycle)
run "cd $RA; cp -a db/import/attendance.yml db/import/attendance.yml.bak-$stamp"
put "$RA/db/import/attendance.yml" server/attendance.yml
run "chown rathena:rathena $RA/db/import/attendance.yml; echo 'attendance.yml: installed'"

# 4. roulette: rAthena's official prize table, loaded once
run "cd $RA; n=\$(mysql -N ragnarok -e 'select count(*) from db_roulette'); if [ \"\$n\" = 0 ]; then mysql ragnarok < sql-files/roulette_default_data.sql && echo 'db_roulette: official prizes loaded'; else echo \"db_roulette: already has \$n prizes\"; fi"

# 4b. Card Seller reads cards from SQL tables (item_db_re, mob_db_re); this server uses the YAML
#     databases, so build rAthena's yaml2sql once and load the official tables (stock sql-files restored)
run "set -e; cd $RA; n=\$(mysql -N ragnarok -e \"select count(*) from information_schema.tables where table_schema='ragnarok' and table_name in ('item_db_re','mob_db_re')\")
if [ \"\$n\" = 2 ]; then echo 'item_db_re/mob_db_re: already loaded'; else
  [ -x yaml2sql ] || su rathena -s /bin/bash -c 'make tools -j2 >/dev/null'
  su rathena -s /bin/bash -c 'printf yyyyyyyyyyyyyyyyyyyy | ./yaml2sql >/dev/null 2>&1'
  mysql ragnarok < sql-files/item_db_re.sql
  for f in item_db_re_usable item_db_re_equip item_db_re_etc mob_db_re; do mysql ragnarok < sql-files/\$f.sql; done
  git -c safe.directory='*' checkout -- sql-files/
  echo \"item_db_re/mob_db_re: loaded (\$(mysql -N ragnarok -e 'select count(*) from item_db_re') items)\"
fi"
# drop the first version of the daily login script (flat 3 Bronze a day), replaced by daily_streak.txt
run "cd $RA; grep -qx 'npc: npc/custom/hufflabs/daily_roulette.txt' conf/import/map_conf.txt && { cp -a conf/import/map_conf.txt conf/import/map_conf.txt.bak-$stamp; sed -i '\\|^npc: npc/custom/hufflabs/daily_roulette.txt\$|d' conf/import/map_conf.txt; rm -f npc/custom/hufflabs/daily_roulette.txt; echo 'daily_roulette.txt: removed'; } || true"

# 5. NPCs: daily login streak, stock extras, Battlegrounds-style PvP room
run "mkdir -p $RA/npc/custom/hufflabs"
put "$RA/npc/custom/hufflabs/daily_streak.txt" server/npc/daily_streak.txt
run "chown rathena:rathena $RA/npc/custom/hufflabs/daily_streak.txt"
# Monster Arena's stock spot (158,193) sits between the Job Master and the Warper at the fountain
run "cd $RA; grep -qP '^prontera,158,193,6\t' npc/custom/etc/monster_arena.txt && { cp -a npc/custom/etc/monster_arena.txt npc/custom/etc/monster_arena.txt.bak-$stamp; sed -i 's/^prontera,158,193,6\t/prontera,166,192,4\t/' npc/custom/etc/monster_arena.txt; echo 'monster_arena.txt: moved to 166,192'; } || true"
for f in hufflabs/daily_streak.txt etc/mvp_room.txt etc/mvp_arena.txt etc/monster_arena.txt etc/morroc_raceway.txt \
         resetnpc.txt etc/bank_kafra.txt etc/lottery.txt etc/blackjack.txt etc/rpsroulette.txt etc/marriage.txt \
         card_seller.txt; do
  addline conf/import/map_conf.txt "npc: npc/custom/$f"
done

# Battlegrounds-style PvP room: must load after the stock map flags (gvg.txt marks guild_vs3 as GvG,
# which would strip the room's battleground flag), so it goes in npc/scripts_custom.conf, read last
run "cd $RA; grep -qx 'npc: npc/custom/battleground/bg_pvp.txt' conf/import/map_conf.txt && { cp -a conf/import/map_conf.txt conf/import/map_conf.txt.bak-$stamp; sed -i '\\|^npc: npc/custom/battleground/bg_pvp.txt\$|d' conf/import/map_conf.txt; } || true"
addline npc/scripts_custom.conf "npc: npc/custom/battleground/bg_pvp.txt"

# 5b. seasonal town weather: the refreshweather script command (source patch, applied once; the map
#     server is rebuilt when the patch goes in) and the weather script
put /tmp/hlro-weather.patch rathena-weather.patch
run "set -e; cd $RA; G='git -c safe.directory=*'
if \$G apply --reverse --check /tmp/hlro-weather.patch 2>/dev/null; then echo 'weather patch: already applied'
else \$G apply /tmp/hlro-weather.patch; echo 'weather patch: applied, rebuilding the map server'
  su rathena -s /bin/bash -c 'make map -j2 >/tmp/hlro-weather-build.log 2>&1' || { tail -20 /tmp/hlro-weather-build.log; exit 1; }
fi; rm -f /tmp/hlro-weather.patch"
put "$RA/npc/custom/hufflabs/town_weather.txt" server/npc/town_weather.txt
run "chown rathena:rathena $RA/npc/custom/hufflabs/town_weather.txt"
addline conf/import/map_conf.txt "npc: npc/custom/hufflabs/town_weather.txt"

# 5c. GM panel (@gm, and "GM panel" in @menu for GMs; the @menu script lives in content/leveling)
put "$RA/npc/custom/hufflabs/gm_panel.txt" server/npc/gm_panel.txt
put "$RA/npc/custom/hufflabs/menu.txt" ../leveling/server/npc/menu.txt
run "chown rathena:rathena $RA/npc/custom/hufflabs/gm_panel.txt $RA/npc/custom/hufflabs/menu.txt"
addline conf/import/map_conf.txt "npc: npc/custom/hufflabs/gm_panel.txt"

# 6. seasonal events by date: calendar + hourly timer; the list is imported by npc/scripts_custom.conf
put /usr/local/sbin/hlro-events.sh server/events/hlro-events.sh
put /etc/hufflabs-events.tsv server/events/events.tsv
put /etc/systemd/system/hlro-events.service server/events/hlro-events.service
put /etc/systemd/system/hlro-events.timer server/events/hlro-events.timer
run "set -e; chmod 755 /usr/local/sbin/hlro-events.sh; cd $RA
[ -f npc/custom/hufflabs/events.conf ] || { echo '// generated by hlro-events.sh' > npc/custom/hufflabs/events.conf; chown rathena:rathena npc/custom/hufflabs/events.conf; }
grep -qx 'import: npc/custom/hufflabs/events.conf' npc/scripts_custom.conf || { cp -a npc/scripts_custom.conf npc/scripts_custom.conf.bak-$stamp; printf '\n// HuffLabs RO: seasonal events for today (written by /usr/local/sbin/hlro-events.sh)\nimport: npc/custom/hufflabs/events.conf\n' >> npc/scripts_custom.conf; echo 'scripts_custom.conf: events import added'; }
systemctl daemon-reload; systemctl enable --now hlro-events.timer >/dev/null; echo \"events today: \$(/usr/local/sbin/hlro-events.sh --list | tr '\n' ' ')\""

if [ $RESTART = 1 ]; then
  run "n=\$(mysql -N ragnarok -e 'select count(*) from \`char\` where online=1'); [ \"\$n\" = 0 ] || { echo \"\$n online, not restarting\"; exit 1; }
/usr/local/sbin/hlro-events.sh || true
systemctl restart rathena-map; sleep 15; systemctl is-active rathena-map"
fi
