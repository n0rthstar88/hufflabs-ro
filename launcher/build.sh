#!/bin/bash
# Builds the Windows launcher (Linux runs the same exe through Proton).
# Needs HLRO_SITE_URL and HLRO_TRUSTED_KEYS (comma-separated base64 ed25519 public
# keys from `hufflabs-ro-publish keygen`), from the environment or the settings
# file (scripts/settings.sh). Output: build/launcher/HuffLabsRO-Launcher.exe
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/settings.sh
need HLRO_SITE_URL HLRO_TRUSTED_KEYS
case $HLRO_SITE_URL in https://*) ;; *) echo "HLRO_SITE_URL must start with https://" >&2; exit 1 ;; esac
PKG=cmd/hufflabs-ro-launcher
trap 'rm -f "$PKG"/rsrc_windows_*.syso' EXIT
(cd "$PKG" && go run github.com/tc-hib/go-winres@v0.3.3 make --in winres/winres.json --arch amd64)
mkdir -p build/launcher
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
	-ldflags "-H=windowsgui -s -w -X main.siteURL=$HLRO_SITE_URL -X main.trustedKeys=$HLRO_TRUSTED_KEYS" \
	-o build/launcher/HuffLabsRO-Launcher.exe "./$PKG"
ls -la build/launcher/HuffLabsRO-Launcher.exe
