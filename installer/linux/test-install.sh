#!/bin/bash
# Tests hufflabs-ro-install.sh against a local fake site, with a stub umu-run and a
# throwaway HOME. Run: bash installer/linux/test-install.sh
set -uo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
INSTALLER="$HERE/hufflabs-ro-install.sh"
W=$(mktemp -d)
trap 'kill $SRV 2>/dev/null; rm -rf "$W"' EXIT
fail=0
check() { if eval "$2"; then echo "ok   $1"; else echo "FAIL $1"; fail=1; fi; }

# --- fake release ---
mkdir -p "$W/rel/HuffLabsRO/data" "$W/rel/HuffLabsRO/defaults" "$W/site/download"
echo exe > "$W/rel/HuffLabsRO/HuffLabsRO.exe"
echo '<clientinfo/>' > "$W/rel/HuffLabsRO/data/clientinfo.xml"
echo 9.9.9 > "$W/rel/HuffLabsRO/VERSION"
head -c 200000 /dev/urandom > "$W/rel/HuffLabsRO/data/big.bin"   # big enough to cut mid-download
printf 'OptionInfoList["WIDTH"] = 1280\n' > "$W/rel/HuffLabsRO/defaults/OptionInfo.lua"
printf 'Windows Registry Editor Version 5.00\r\n\r\n[HKEY_CURRENT_USER\\Software\\Gravity\\RagnarokOnline]\r\n"RagPath"="__INSTALLDIR__\\\\"\r\n' > "$W/rel/HuffLabsRO/defaults/settings.reg"
magick -size 16x16 xc:purple "$W/rel/HuffLabsRO/HuffLabsRO.png" 2>/dev/null || echo png > "$W/rel/HuffLabsRO/HuffLabsRO.png"
(cd "$W/rel" && zip -q -r "$W/site/download/HuffLabsRO-client-9.9.9.zip" HuffLabsRO)
echo 9.9.9 > "$W/site/download/VERSION"
echo launcher-exe > "$W/site/download/HuffLabsRO-Launcher.exe"
(cd "$W/site/download" && sha256sum HuffLabsRO-client-9.9.9.zip HuffLabsRO-Launcher.exe > SHA256SUMS)
PORT=$((20000 + RANDOM % 20000))
python3 -I "$HERE/test-server.py" "$W/site" "$PORT" >/dev/null 2>&1 &
SRV=$!
sleep 1

# --- stub umu-run records what it was asked to do ---
mkdir -p "$W/bin"
cat > "$W/bin/umu-run" <<'EOF'
#!/bin/bash
echo "WINEPREFIX=$WINEPREFIX GAMEID=$GAMEID $*" >> "$UMU_LOG"
[ "$1" = regedit ] && { cp "$3" "$UMU_LOG.reg.raw"; iconv -f UTF-16 -t UTF-8 "$3" > "$UMU_LOG.reg" 2>/dev/null || cp "$3" "$UMU_LOG.reg"; }
[ "$1" = ./HuffLabsRO-Launcher.exe ] && pwd > "$UMU_LOG.pwd"
[ -z "${UMU_FAIL:-}" ]
EOF
chmod +x "$W/bin/umu-run"

run() { # home args...
	local h=$1; shift
	HOME="$h" PATH="$W/bin:$PATH" UMU_LOG="$h/umu.log" HLRO_SITE_URL="http://127.0.0.1:$PORT" bash "$INSTALLER" "$@" > "$h/out.txt" 2>&1
}

# 1. good install into a path with a space
H1="$W/h1"; mkdir -p "$H1"
run "$H1" --dir "$H1/My Games/HLRO"; rc=$?
D="$H1/My Games/HLRO"
check "good install exits 0" '[ $rc = 0 ] || { cat "$H1/out.txt"; false; }'
check "game files unpacked" '[ -f "$D/HuffLabsRO.exe" ] && [ -f "$D/data/clientinfo.xml" ]'
check "default settings copied" 'grep -q 1280 "$D/savedata/OptionInfo.lua"'
check "registry imported with Wine path" 'grep -q "Z:\\\\\\\\" "$H1/umu.log.reg" && ! grep -q __INSTALLDIR__ "$H1/umu.log.reg"'
check "registry path escaped for .reg" 'grep -qF "\"RagPath\"=\"Z:\\\\" "$H1/umu.log.reg" && grep -qF "\\\\My Games\\\\HLRO\\\\\"" "$H1/umu.log.reg"'
check "download folder removed after install" '[ ! -d "$D/.download" ]'
check "launcher written" '[ -x "$H1/.local/bin/hufflabs-ro" ] && grep -q "umu-run ./HuffLabsRO-Launcher.exe" "$H1/.local/bin/hufflabs-ro"'
check "launcher exe installed" '[ "$(cat "$D/HuffLabsRO-Launcher.exe" 2>/dev/null)" = launcher-exe ]'
HOME="$H1" PATH="$W/bin:$PATH" UMU_LOG="$H1/umu.log" bash "$H1/.local/bin/hufflabs-ro" >/dev/null 2>&1
check "launcher script starts the launcher in the install folder" 'grep -q "HuffLabsRO-Launcher.exe" "$H1/umu.log" && [ "$(cat "$H1/umu.log.pwd")" = "$D" ]'
check "launcher cds into a path with a space" 'grep -qxF "cd $(printf %q "$D") || exit 1" "$H1/.local/bin/hufflabs-ro"'
check "desktop entry written" 'grep -q "^Exec=.*hufflabs-ro" "$H1/.local/share/applications/hufflabs-ro.desktop" && grep -q "^Icon=hufflabs-ro" "$H1/.local/share/applications/hufflabs-ro.desktop"'
check "icon installed" '[ -f "$H1/.local/share/icons/hicolor/256x256/apps/hufflabs-ro.png" ]'

# 2. rerun keeps the player's settings, and doesn't download the same version again
echo 'OptionInfoList["WIDTH"] = 2560' > "$D/savedata/OptionInfo.lua"
: > "$W/requests.log"
rm -f "$H1/.local/bin/hufflabs-ro"
run "$H1" --dir "$D"; rc=$?
check "rerun exits 0" '[ $rc = 0 ] || { cat "$H1/out.txt"; false; }'
check "rerun keeps savedata" 'grep -q 2560 "$D/savedata/OptionInfo.lua"'
check "rerun skips download of installed version" '! grep -q "\.zip" "$W/requests.log" && grep -q "already installed" "$H1/out.txt"'
check "rerun still repairs the launcher" '[ -x "$H1/.local/bin/hufflabs-ro" ]'

# 2b. a folder the launcher manages isn't re-downloaded, even when the zip is newer
H12="$W/h12"; D12="$H12/Games/HuffLabsRO"; mkdir -p "$D12/.hufflabs-launcher"
echo 9.9.8 > "$D12/VERSION"; echo '{}' > "$D12/.hufflabs-launcher/state.json"
cp -r "$W/rel/HuffLabsRO/defaults" "$W/rel/HuffLabsRO/HuffLabsRO.png" "$D12/"
echo "my newer file" > "$D12/HuffLabsRO.exe"
: > "$W/requests.log"
run "$H12"; rc=$?
check "launcher-managed install exits 0" '[ $rc = 0 ] || { cat "$H12/out.txt"; false; }'
check "launcher-managed install skips the zip" '! grep -q "\.zip" "$W/requests.log" && [ "$(cat "$D12/HuffLabsRO.exe")" = "my newer file" ]'
check "launcher-managed install refreshes shortcuts" '[ -x "$H12/.local/bin/hufflabs-ro" ] && [ -f "$D12/HuffLabsRO-Launcher.exe" ]'

# 2c. a launcher with the wrong checksum is refused
H13="$W/h13"; mkdir -p "$H13"
cp "$W/site/download/HuffLabsRO-Launcher.exe" "$W/launcher.good"
echo tampered > "$W/site/download/HuffLabsRO-Launcher.exe"
run "$H13"; rc=$?
check "bad launcher exits 3" '[ $rc = 3 ] && ! [ -e "$H13/.local/bin/hufflabs-ro" ] && ! [ -e "$H13/Games/HuffLabsRO/HuffLabsRO-Launcher.exe" ]'
cp "$W/launcher.good" "$W/site/download/HuffLabsRO-Launcher.exe"

# 3. wrong checksum
H3="$W/h3"; mkdir -p "$H3"
cp "$W/site/download/SHA256SUMS" "$W/sums.good"
sed -i '/client-9.9.9.zip/s/^[0-9a-f]*/'"$(printf '0%.0s' {1..64})"'/' "$W/site/download/SHA256SUMS"
run "$H3"; rc=$?
check "bad checksum exits 3" '[ $rc = 3 ]'
check "bad download removed" '! ls "$H3/Games/HuffLabsRO/.download/"*.zip >/dev/null 2>&1'
check "bad checksum message" 'grep -q "The download was damaged. Run this again to retry." "$H3/out.txt"'
cp "$W/sums.good" "$W/site/download/SHA256SUMS"

# 3b. interrupted download keeps the partial file and says to run again
H6="$W/h6"; mkdir -p "$H6"
echo 50000 > "$W/cut"
: > "$W/requests.log"
run "$H6"; rc=$?
PART="$H6/Games/HuffLabsRO/.download/HuffLabsRO-client-9.9.9.zip"
check "interrupted exits 5" '[ $rc = 5 ] || { cat "$H6/out.txt"; false; }'
check "interrupted message" 'grep -q "The download was interrupted. Run this again to continue." "$H6/out.txt"'
check "partial download kept" '[ -s "$PART" ]'
partsize=$(stat -c %s "$PART" 2>/dev/null || echo 0)
rm -f "$W/cut"
: > "$W/requests.log"
run "$H6"; rc=$?
check "resumed install exits 0" '[ $rc = 0 ] || { cat "$H6/out.txt"; false; }'
check "resume asks for the rest only" 'grep -q "\.zip bytes=$partsize-" "$W/requests.log"'
check "resumed install unpacked" '[ -f "$H6/Games/HuffLabsRO/data/big.bin" ]'

# 3c. a complete but corrupt leftover download is thrown away
H7="$W/h7"; mkdir -p "$H7/Games/HuffLabsRO/.download"
head -c "$(stat -c %s "$W/site/download/HuffLabsRO-client-9.9.9.zip")" /dev/zero > "$H7/Games/HuffLabsRO/.download/HuffLabsRO-client-9.9.9.zip"
run "$H7"; rc=$?
check "corrupt full download exits 3" '[ $rc = 3 ] || { cat "$H7/out.txt"; false; }'
check "corrupt full download removed" '! ls "$H7/Games/HuffLabsRO/.download/"*.zip >/dev/null 2>&1'
run "$H7"; rc=$?
check "then a rerun installs" '[ $rc = 0 ]'

# 3d. no checksum file on the server
H8="$W/h8"; mkdir -p "$H8"
mv "$W/site/download/SHA256SUMS" "$W/sums.away"
run "$H8"; rc=$?
check "missing SHA256SUMS exits 2 with a message" '[ $rc = 2 ] && grep -q "The server has no checksum" "$H8/out.txt"'
mv "$W/sums.away" "$W/site/download/SHA256SUMS"

# 3e. Proton setup fails
H9="$W/h9"; mkdir -p "$H9"
UMU_FAIL=1 run "$H9"; rc=$?
check "Proton failure exits 4 with a message" '[ $rc = 4 ] && grep -q "Setting up Proton failed" "$H9/out.txt"'

# 3f. non-ASCII install path
H10="$W/h10"; mkdir -p "$H10"
run "$H10" --dir "$H10/Jeux à moi/RO"; rc=$?
check "non-ASCII path installs" '[ $rc = 0 ] && [ -f "$H10/Jeux à moi/RO/HuffLabsRO.exe" ] && grep -qF "Jeux à moi\\\\RO" "$H10/umu.log.reg"'

# 3g. awkward characters in the install folder and a space in HOME
H11="$W/home dir"; mkdir -p "$H11"
D11="$H11/R&D #1 it's \$HOME"
run "$H11" --dir "$D11"; rc=$?
check "odd-character path installs" '[ $rc = 0 ] && [ -f "$D11/HuffLabsRO.exe" ] || { cat "$H11/out.txt"; false; }'
check "odd-character path in .reg" 'grep -qF "R&D #1 it'"'"'s \$HOME\\\\\"" "$H11/umu.log.reg"'
check ".reg is UTF-16LE with BOM" '[ "$(head -c2 "$H11/umu.log.reg.raw" | od -An -tx1 | tr -d " ")" = fffe ]'
HOME="$H11" PATH="$W/bin:$PATH" UMU_LOG="$H11/umu.log" bash "$H11/.local/bin/hufflabs-ro" >/dev/null 2>&1
check "launcher runs from the odd-character folder" '[ "$(cat "$H11/umu.log.pwd" 2>/dev/null)" = "$D11" ]'
check "desktop Exec quoted for a space in HOME" 'grep -qxF "Exec=\"$H11/.local/bin/hufflabs-ro\"" "$H11/.local/share/applications/hufflabs-ro.desktop"'
run "$H11" --dir "$H11/bad\"quote"; rc=$?
check "double quote in path refused" '[ $rc = 64 ] && grep -q "can.t contain" "$H11/out.txt"'

# 3h. an installer copy without a server address (not published by the release
#     scripts, which fill it in) says so instead of guessing
H14="$W/h14"; mkdir -p "$H14"
HOME="$H14" PATH="$W/bin:$PATH" bash "$INSTALLER" > "$H14/out.txt" 2>&1; rc=$?
check "unconfigured installer exits 64 with a message" '[ $rc = 64 ] && grep -q "wasn.t built with a server address" "$H14/out.txt"'
sed "s#__SITE_URL__#http://127.0.0.1:$PORT#" "$INSTALLER" > "$W/filled.sh"
H15="$W/h15"; mkdir -p "$H15"
HOME="$H15" PATH="$W/bin:$PATH" UMU_LOG="$H15/umu.log" bash "$W/filled.sh" > "$H15/out.txt" 2>&1; rc=$?
check "filled-in installer works without HLRO_SITE_URL" '[ $rc = 0 ] || { cat "$H15/out.txt"; false; }'

# 4. server down
H4="$W/h4"; mkdir -p "$H4"
HOME="$H4" PATH="$W/bin:$PATH" UMU_LOG="$H4/umu.log" HLRO_SITE_URL="http://127.0.0.1:1" bash "$INSTALLER" > "$H4/out.txt" 2>&1; rc=$?
check "unreachable exits 2" '[ $rc = 2 ]'
check "unreachable message" 'grep -q "Install and connect Tailscale" "$H4/out.txt"'

# 5. umu-run missing
H5="$W/h5"; mkdir -p "$H5"
mkdir -p "$W/minbin"
for t in curl unzip sha256sum iconv cat; do ln -sf "$(command -v $t)" "$W/minbin/$t"; done
HOME="$H5" PATH="$W/minbin" HLRO_SITE_URL="http://127.0.0.1:$PORT" "$BASH" "$INSTALLER" > "$H5/out.txt" 2>&1; rc=$?
check "missing umu-run exits 1" '[ $rc = 1 ] && grep -q "umu-launcher" "$H5/out.txt"'

[ $fail = 0 ] && echo "ALL PASS" || { echo "SOME FAILED"; exit 1; }
