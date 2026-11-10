#!/bin/bash
# Runs internal/store integration tests inside the portal container against throwaway
# ptest_portal / ptest_game databases (DSNs in /root/ptest.env). They were dropped
# after the 2026-11-09 review; to test again, recreate them in the container as root:
#   mariadb -e "CREATE DATABASE ptest_portal; CREATE DATABASE ptest_game;
#     CREATE TABLE ptest_game.login LIKE ragnarok.login;
#     CREATE USER ptest@localhost IDENTIFIED BY '<new password>';
#     GRANT ALL ON ptest_portal.* TO ptest@localhost; GRANT ALL ON ptest_game.* TO ptest@localhost;"
#   /root/ptest.env:
#     export PORTAL_TEST_DSN='ptest:<pw>@unix(/run/mysqld/mysqld.sock)/ptest_portal?parseTime=true'
#     export GAME_TEST_DSN='ptest:<pw>@unix(/run/mysqld/mysqld.sock)/ptest_game?parseTime=true'
# and drop them again afterwards. The tests refuse databases not named ptest_*.
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/settings.sh
need HLRO_PROXMOX HLRO_CT
PROXMOX=$HLRO_PROXMOX CT=$HLRO_CT
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c -o build/store.test ./internal/store
scp -q -o BatchMode=yes build/store.test "$PROXMOX:/root/store.test"
ssh -o BatchMode=yes "$PROXMOX" "pct push $CT /root/store.test /root/store.test --perms 0700 && rm /root/store.test && pct exec $CT -- bash -c '. /root/ptest.env && /root/store.test -test.count=1 $*; rc=\$?; rm -f /root/store.test; exit \$rc'"
