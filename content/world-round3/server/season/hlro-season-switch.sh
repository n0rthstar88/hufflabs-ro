#!/bin/bash
# HuffLabs RO: switches Prontera to the real season on the server, no PC needed.
# The publisher signs one client manifest per season (update/seasons/<season>.json, see
# cmd/hufflabs-ro-publish/seasons.go). On the season's date this script, under the same lock
# the publisher uses: points update/manifest.json at that season, puts the season's
# prontera.gat (taken from the update store) into the server's map cache, and restarts the map
# server. While anyone is online it waits (exit 75); the hourly timer tries again.
#   hlro-season-switch.sh            switch if today's season differs from the live one
#   hlro-season-switch.sh <season>   switch to <season> now (still waits for an empty server)
#   hlro-season-switch.sh --status   print the live season and today's season
set -euo pipefail
RA=/home/rathena/rathena
D=/srv/hufflabs-portal/update
GAT=/usr/local/lib/hufflabs/mapcache-gat.py
md=$(date +%m%d)
if   [ "$md" -ge 1221 ] || [ "$md" -lt 0320 ]; then today=winter
elif [ "$md" -lt 0621 ]; then today=spring
elif [ "$md" -lt 0922 ]; then today=summer
else today=fall; fi
cur=$(cat "$D/seasons/current" 2>/dev/null || echo none)
if [ "${1:-}" = --status ]; then echo "live: $cur, today: $today"; exit 0; fi
want=${1:-$today}
case $want in spring|summer|fall|winter) ;; *) echo "unknown season $want" >&2; exit 2 ;; esac
if [ "$cur" = "$want" ]; then echo "already $want"; exit 0; fi
[ -f "$D/seasons/$want.json" ] || { echo "no signed manifest for $want yet (publish from the PC once)" >&2; exit 1; }
online=$(mysql -N ragnarok -e 'select count(*) from `char` where online=1')
if [ "$online" != 0 ]; then echo "$online online; waiting"; exit 75; fi

exec 9>"$D/seasons/.lock"; flock -w 120 9
sha=$(python3 -I - "$D/seasons/$want.json" <<'PY'
import base64, json, sys
env = json.load(open(sys.argv[1]))
m = json.loads(base64.b64decode(env["payload"]))
hits = [f["SHA256"] for f in m["Files"] if f["Path"].lower() == "data/prontera.gat"]
assert len(hits) == 1, hits
print(hits[0])
PY
)
[ -f "$D/files/$sha" ] || { echo "prontera.gat for $want ($sha) is not in the update store" >&2; exit 1; }
stamp=$(date +%Y%m%d-%H%M%S)
cp -a "$RA/db/import/map_cache.dat" "/root/backups/map_cache.dat.bak-$stamp"
python3 -I "$GAT" "$RA" prontera "$D/files/$sha"
chown rathena:rathena "$RA/db/import/map_cache.dat"
echo "$want" > "$RA/db/import/hlro_season.txt"
cp "$D/seasons/$want.json" "$D/manifest.json.new"
chmod 644 "$D/manifest.json.new"; chgrp portal "$D/manifest.json.new" 2>/dev/null || true
mv -f "$D/manifest.json.new" "$D/manifest.json"
echo "$want" > "$D/seasons/current"
systemctl restart rathena-map
ls -1t /root/backups/map_cache.dat.bak-* 2>/dev/null | tail -n +5 | xargs -r rm -f
echo "Prontera: $cur -> $want (client manifest switched, map server restarted)"
