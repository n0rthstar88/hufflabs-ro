#!/bin/bash
# One-time container changes for the portal, run from your PC. Idempotent.
set -euo pipefail
cd "$(dirname "$0")"
. ../scripts/settings.sh
need HLRO_PROXMOX HLRO_CT HLRO_SITE_URL
PROXMOX=$HLRO_PROXMOX CT=$HLRO_CT SITE_URL=$HLRO_SITE_URL
JOB=${HLRO_BACKUP_JOB:-}   # Proxmox backup job to add the container to; empty skips it

scp -q -o BatchMode=yes ct-setup.sh "$PROXMOX:/root/ct-setup.sh"
ssh -o BatchMode=yes "$PROXMOX" bash -s -- "$CT" "$SITE_URL" "$JOB" <<'HOST'
set -euo pipefail
CT=$1 SITE_URL=$2 JOB=$3
echo "== disk"
SIZE=$(pct config "$CT" | sed -n 's/^rootfs:.*size=\([0-9]*\)G.*/\1/p')
if [ "${SIZE:-0}" -lt 40 ]; then pct resize "$CT" rootfs 40G; echo "resized ${SIZE}G -> 40G"; else echo "already ${SIZE}G"; fi
pct push "$CT" /root/ct-setup.sh /root/ct-setup.sh --perms 0700
rm /root/ct-setup.sh
pct exec "$CT" -- /root/ct-setup.sh "$SITE_URL"
pct exec "$CT" -- rm /root/ct-setup.sh
echo "== nightly backup job"
if [ -z "$JOB" ]; then echo "HLRO_BACKUP_JOB not set; skipped"; exit 0; fi
VMIDS=$(pvesh get /cluster/backup/"$JOB" --output-format json | python3 -c 'import json,sys; print(json.load(sys.stdin)["vmid"])')
case ",$VMIDS," in
	*",$CT,"*) echo "already includes $CT: $VMIDS" ;;
	*) pvesh set /cluster/backup/"$JOB" --vmid "$VMIDS,$CT"; echo "vmid now $VMIDS,$CT" ;;
esac
HOST
