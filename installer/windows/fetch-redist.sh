#!/bin/bash
# Downloads Microsoft's runtime installers bundled into HuffLabsRO-Setup.exe, from
# Microsoft's own URLs. The first run records their SHA-256 in redist.sha256
# (committed); later runs refuse files that don't match.
set -euo pipefail
cd "$(dirname "$0")"
mkdir -p redist
fetch() { # name url
	[ -f "redist/$1" ] || curl -fL --retry 3 -o "redist/$1" "$2"
}
fetch vc_redist.x86.exe https://aka.ms/vs/17/release/vc_redist.x86.exe
fetch vc_redist.x64.exe https://aka.ms/vs/17/release/vc_redist.x64.exe
fetch directx_Jun2010_redist.exe https://download.microsoft.com/download/8/4/A/84A35BF1-DAFE-4AE8-82AF-AD2AE20B6B14/directx_Jun2010_redist.exe
if [ -f redist.sha256 ]; then
	(cd redist && sha256sum -c ../redist.sha256)
else
	(cd redist && sha256sum vc_redist.x86.exe vc_redist.x64.exe directx_Jun2010_redist.exe) > redist.sha256
	echo "recorded redist.sha256 (commit it)"
fi
