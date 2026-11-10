#!/bin/bash
# One-time portal setup inside the portal container. Idempotent; pushed and run by server-setup.sh.
# Usage: ct-setup.sh <SITE_URL>
set -euo pipefail
SITE_URL=${1:?SITE_URL}
R=/home/rathena/rathena
LOGIN_CONF=$R/conf/import/login_conf.txt
ENV=/etc/hufflabs-portal/portal.env

echo "== portal user and folders"
id portal >/dev/null 2>&1 || useradd --system --home-dir /srv/hufflabs-portal --shell /usr/sbin/nologin portal
install -d -o root -g portal -m 750 /etc/hufflabs-portal
install -d -o root -g portal -m 755 /srv/hufflabs-portal /srv/hufflabs-portal/download \
	/srv/hufflabs-portal/update /srv/hufflabs-portal/update/files

echo "== rAthena MD5 passwords"
if grep -q '^use_MD5_passwords: yes' "$LOGIN_CONF"; then
	echo "already enabled"
else
	BK=/root/login-before-md5-$(date +%F).sql
	# rathena.target does not stop login/char, so stop each server explicitly.
	systemctl stop rathena-map rathena-char rathena-login
	(umask 077; mariadb-dump ragnarok login > "$BK")
	echo "backup: $BK ($(wc -l < "$BK") lines)"
	# One-time conversion of every row (guarded by the config line above).
	mariadb ragnarok -e "UPDATE login SET user_pass = MD5(user_pass)"
	printf '\n// HuffLabs portal (2026-11-09): passwords stored as MD5\nuse_MD5_passwords: yes\n' >> "$LOGIN_CONF"
	systemctl start rathena-login; sleep 2; systemctl start rathena-char; sleep 2; systemctl start rathena-map
	echo "converted $(mariadb -N ragnarok -e 'SELECT COUNT(*) FROM login') accounts"
fi

echo "== portal database and settings"
if [ -f "$ENV" ]; then
	echo "$ENV exists, leaving it"
else
	PW=$(openssl rand -hex 16)
	mariadb <<SQL
CREATE DATABASE IF NOT EXISTS hufflabs_portal CHARACTER SET utf8mb4;
CREATE USER IF NOT EXISTS 'portal'@'localhost' IDENTIFIED BY '$PW';
ALTER USER 'portal'@'localhost' IDENTIFIED BY '$PW';
GRANT ALL ON hufflabs_portal.* TO 'portal'@'localhost';
GRANT SELECT, INSERT ON ragnarok.login TO 'portal'@'localhost';
SQL
	DEV=$(tailscale status --json | python3 -c 'import json,sys; print(json.load(sys.stdin)["Self"]["ID"])')
	SOCK='unix(/run/mysqld/mysqld.sock)'
	(umask 027; cat > "$ENV" <<EOF
PORTAL_DSN=portal:$PW@$SOCK/hufflabs_portal?parseTime=true
GAME_DSN=portal:$PW@$SOCK/ragnarok?parseTime=true
# Tailscale OAuth client with only the device_invites scope (paste them in).
TS_CLIENT_ID=
TS_CLIENT_SECRET=
TS_DEVICE_ID=$DEV
SITE_URL=$SITE_URL
DOWNLOAD_DIR=/srv/hufflabs-portal/download
UPDATE_DIR=/srv/hufflabs-portal/update
SESSION_KEY=$(openssl rand -hex 32)
EOF
	)
	chown root:portal "$ENV"
	chmod 640 "$ENV"
	echo "wrote $ENV (device $DEV); TS_CLIENT_ID/TS_CLIENT_SECRET still empty"
fi
# launcher updates (added 2026-11-10; existing portal.env files get the line here)
grep -q '^UPDATE_DIR=' "$ENV" || { echo 'UPDATE_DIR=/srv/hufflabs-portal/update' >> "$ENV"; echo "added UPDATE_DIR to $ENV"; }
