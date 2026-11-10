#!/bin/bash
# Build hufflabs-portal and install it in the portal container. Starts it only once the Tailscale
# OAuth client is filled in portal.env.
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/settings.sh
need HLRO_PROXMOX HLRO_CT
PROXMOX=$HLRO_PROXMOX CT=$HLRO_CT

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o build/hufflabs-portal ./cmd/hufflabs-portal
scp -q -o BatchMode=yes build/hufflabs-portal deploy/hufflabs-portal.service "$PROXMOX:/root/"
ssh -o BatchMode=yes "$PROXMOX" bash -s -- "$CT" <<'HOST'
set -euo pipefail
CT=$1
pct push "$CT" /root/hufflabs-portal /usr/local/bin/hufflabs-portal.new --perms 0755
pct push "$CT" /root/hufflabs-portal.service /etc/systemd/system/hufflabs-portal.service --perms 0644
rm /root/hufflabs-portal /root/hufflabs-portal.service
pct exec "$CT" -- bash -s <<'CTSCRIPT'
set -euo pipefail
mv /usr/local/bin/hufflabs-portal.new /usr/local/bin/hufflabs-portal
systemctl daemon-reload
if grep -q '^TS_CLIENT_SECRET=.\+' /etc/hufflabs-portal/portal.env; then
	systemctl stop tstest.service 2>/dev/null || true
	systemctl enable hufflabs-portal >/dev/null 2>&1
	systemctl restart hufflabs-portal
	sleep 2
	systemctl --no-pager --lines=5 status hufflabs-portal | sed -n '1,3p;/listening/p;/err/p'
else
	echo "Installed, not started: TS_CLIENT_ID / TS_CLIENT_SECRET are empty in /etc/hufflabs-portal/portal.env"
fi
CTSCRIPT
HOST
