#!/usr/bin/env bash
# Installs the server-side season switch (server/season/) on the game server container and
# records the season that is live now. After this the publisher's PC is no longer needed for
# season changes; it only has to have published once with --seasons (all four signed).
#   ./install-season-switch.sh
set -euo pipefail
cd "$(dirname "$0")"
source ../../scripts/settings.sh
need HLRO_PROXMOX HLRO_CT
run() { ssh -o BatchMode=yes "$HLRO_PROXMOX" "pct exec $HLRO_CT -- bash -c $(printf '%q' "$1")"; }
put() { ssh -o BatchMode=yes "$HLRO_PROXMOX" "pct exec $HLRO_CT -- tee $1 >/dev/null" < "$2"; }
run "mkdir -p /usr/local/lib/hufflabs /srv/hufflabs-portal/update/seasons"
put /usr/local/lib/hufflabs/mapcache-gat.py server/mapcache-gat.py
put /usr/local/sbin/hlro-season-switch.sh server/season/hlro-season-switch.sh
put /etc/systemd/system/hlro-season.service server/season/hlro-season.service
put /etc/systemd/system/hlro-season.timer server/season/hlro-season.timer
run "set -e; chmod 755 /usr/local/sbin/hlro-season-switch.sh; D=/srv/hufflabs-portal/update
[ -s \$D/seasons/current ] || { cat /home/rathena/rathena/db/import/hlro_season.txt > \$D/seasons/current; echo \"seasons/current: \$(cat \$D/seasons/current)\"; }
chgrp -R portal \$D/seasons 2>/dev/null || true
systemctl daemon-reload; systemctl enable --now hlro-season.timer >/dev/null; /usr/local/sbin/hlro-season-switch.sh --status"
