#!/bin/bash
# HuffLabs RO installer for Linux. Run: bash hufflabs-ro-install.sh [--dir <install folder>]
# Needs Tailscale connected (the server is only reachable over it) and umu-launcher,
# which runs the Windows game through Proton without Steam.
set -euo pipefail

# The release scripts fill in __SITE_URL__ when they publish this file.
SITE_URL=${HLRO_SITE_URL:-__SITE_URL__}
INSTALL_DIR="$HOME/Games/HuffLabsRO"
PREFIX=${HLRO_PREFIX:-$HOME/.local/share/hufflabs-ro/prefix}

while [ $# -gt 0 ]; do
	case $1 in
	--dir) INSTALL_DIR=${2:?--dir needs a folder}; shift 2 ;;
	-h | --help) echo "Usage: bash hufflabs-ro-install.sh [--dir <install folder>]"; exit 0 ;;
	*) echo "Unknown option: $1" >&2; exit 64 ;;
	esac
done
INSTALL_DIR=${INSTALL_DIR%/}
# Windows paths can't hold these, and Proton sees the folder as a Windows path.
case $INSTALL_DIR in
*[\"\\:*?\<\>\|]* | *$'\n'*)
	echo "The install folder can't contain any of these: \" \\ : * ? < > |" >&2
	exit 64 ;;
esac

case $SITE_URL in
https://* | http://127.0.0.1:*) ;; # plain http only for the local test server
*)
	echo "This copy of the installer wasn't built with a server address. Download it from your HuffLabs RO site." >&2
	exit 64 ;;
esac

say() { printf '\033[1;35m==>\033[0m %s\n' "$*"; }

# 1. tools
missing=()
for t in curl unzip sha256sum iconv; do command -v "$t" >/dev/null || missing+=("$t"); done
if [ ${#missing[@]} -gt 0 ]; then
	echo "Please install: ${missing[*]}" >&2
	exit 1
fi
if ! command -v umu-run >/dev/null; then
	cat >&2 <<'EOF'
HuffLabs RO needs umu-launcher to run the game. Install it, then run this again:
  Arch / CachyOS / Omarchy:  sudo pacman -S umu-launcher
  Fedora:                    sudo dnf install umu-launcher
  Debian / Ubuntu:           download the .deb from
                             https://github.com/Open-Wine-Components/umu-launcher/releases
EOF
	exit 1
fi

# 2. can we reach the server?
if ! VERSION=$(curl -sf --max-time 15 "$SITE_URL/download/VERSION"); then
	echo "Can't reach the HuffLabs RO server. Install and connect Tailscale: https://tailscale.com/download" >&2
	exit 2
fi
VERSION=$(printf '%s' "$VERSION" | tr -d '[:space:]')
NAME="HuffLabsRO-client-$VERSION.zip"
SUMS=$(curl -sf --max-time 15 "$SITE_URL/download/SHA256SUMS" || true)

# 3a. the launcher (checks for updates, then starts the game); fetched first so a
#     bad download stops before anything is installed
LWANT=$(printf '%s\n' "$SUMS" | awk '$2 == "HuffLabsRO-Launcher.exe" {print $1}')
[ -n "$LWANT" ] || { echo "The server has no checksum for the launcher. Try again later." >&2; exit 2; }
mkdir -p "$INSTALL_DIR"
LTMP=$(mktemp "$INSTALL_DIR/.launcher.XXXXXX")
trap 'rm -f "$LTMP"' EXIT
if ! curl -fsL --retry 5 --retry-delay 3 --max-time 600 -o "$LTMP" "$SITE_URL/download/HuffLabsRO-Launcher.exe"; then
	echo "Couldn't download the launcher. Run this again." >&2
	exit 5
fi
if [ "$(sha256sum "$LTMP" | cut -d' ' -f1)" != "$LWANT" ]; then
	echo "The launcher download was damaged. Run this again to retry." >&2
	exit 3
fi

if [ -f "$INSTALL_DIR/.hufflabs-launcher/state.json" ]; then
	say "HuffLabs RO is already installed here; the launcher keeps it up to date"
elif [ "$(tr -d '[:space:]' 2>/dev/null < "$INSTALL_DIR/VERSION")" = "$VERSION" ]; then
	say "HuffLabs RO $VERSION is already installed; checking settings and shortcuts"
else
	WANT=$(printf '%s\n' "$SUMS" | awk -v n="$NAME" '$2 == n {print $1}')
	[ -n "$WANT" ] || { echo "The server has no checksum for $NAME. Try again later." >&2; exit 2; }

	# 3. download (resumes if interrupted) and verify
	DL="$INSTALL_DIR/.download"
	mkdir -p "$DL"
	ZIP="$DL/$NAME"
	have() { [ -f "$ZIP" ] && [ "$(sha256sum "$ZIP" | cut -d' ' -f1)" = "$WANT" ]; }
	if have; then
		say "Already downloaded HuffLabs RO $VERSION"
	else
		say "Downloading HuffLabs RO $VERSION (about 4 GB)"
		SIZE=$(curl -sfI --max-time 15 "$SITE_URL/download/$NAME" | tr -d '\r' | awk 'tolower($1) == "content-length:" {print $2}' || true)
		curl -fL -C - --retry 5 --retry-delay 3 --speed-limit 1024 --speed-time 60 -o "$ZIP" "$SITE_URL/download/$NAME" && ok=1 || ok=0
		got=$(stat -c %s "$ZIP" 2>/dev/null || echo 0)
		if ! have; then
			# Keep a partial file so the next run picks up where this one stopped;
			# throw it away only when it is complete (or too big) and still wrong.
			if [ $ok = 0 ] && [ -n "$SIZE" ] && [ "$got" -lt "$SIZE" ]; then
				echo "The download was interrupted. Run this again to continue." >&2
				exit 5
			fi
			rm -f "$ZIP"
			echo "The download was damaged. Run this again to retry." >&2
			exit 3
		fi
	fi

	# 4. unpack (the archive holds one HuffLabsRO/ folder; savedata/ isn't in it, so a
	#    player's settings survive an update)
	say "Installing to $INSTALL_DIR"
	TMP=$(mktemp -d "$INSTALL_DIR/.unpack.XXXXXX")
	trap 'rm -rf "$TMP" "$LTMP"' EXIT
	unzip -q -o "$ZIP" -d "$TMP"
	cp -a "$TMP/HuffLabsRO/." "$INSTALL_DIR/"
	rm -rf "$TMP" "$DL"   # the 4 GB zip isn't needed once unpacked
fi

chmod 644 "$LTMP"
mv -f "$LTMP" "$INSTALL_DIR/HuffLabsRO-Launcher.exe"

# 5. default settings, so the game never waits for OpenSetup on first start
mkdir -p "$INSTALL_DIR/savedata"
[ -f "$INSTALL_DIR/savedata/OptionInfo.lua" ] || cp "$INSTALL_DIR/defaults/OptionInfo.lua" "$INSTALL_DIR/savedata/OptionInfo.lua"
mkdir -p "$PREFIX"
WINPATH="Z:${INSTALL_DIR//\//\\}"
REG="$PREFIX/hufflabs-settings.reg"
# .reg strings need doubled backslashes. Plain bash replacement (quoted, so & and #
# in the path stay literal), then UTF-16LE with a BOM, which is what regedit
# expects for paths that aren't plain ASCII.
REGTEXT=$(cat "$INSTALL_DIR/defaults/settings.reg")
REGTEXT=${REGTEXT//__INSTALLDIR__/"${WINPATH//\\/\\\\}"}
{ printf '\xff\xfe'; printf '%s\r\n' "${REGTEXT%$'\r'}" | iconv -f UTF-8 -t UTF-16LE; } > "$REG"
say "Setting up Proton (the first run downloads it, which takes a few minutes)"
if ! WINEPREFIX="$PREFIX" GAMEID=0 umu-run regedit /S "$REG" > "$PREFIX/setup.log" 2>&1; then
	echo "Setting up Proton failed. Details are in $PREFIX/setup.log. Check your internet connection and run this again." >&2
	exit 4
fi

# 6. launcher, desktop entry and icon
mkdir -p "$HOME/.local/bin" "$HOME/.local/share/applications" "$HOME/.local/share/icons/hicolor/256x256/apps"
cat > "$HOME/.local/bin/hufflabs-ro" <<EOF
#!/bin/bash
# HuffLabs RO (installed by hufflabs-ro-install.sh)
cd $(printf '%q' "$INSTALL_DIR") || exit 1
export WINEPREFIX=$(printf '%q' "$PREFIX") GAMEID=0
exec umu-run ./HuffLabsRO-Launcher.exe "\$@"
EOF
chmod +x "$HOME/.local/bin/hufflabs-ro"
cp "$INSTALL_DIR/HuffLabsRO.png" "$HOME/.local/share/icons/hicolor/256x256/apps/hufflabs-ro.png"
# Exec= quoting from the desktop entry spec: escape " ` $ \ inside the quotes, then
# double every backslash for the key's own string escaping.
EXEC="$HOME/.local/bin/hufflabs-ro"
EXEC=${EXEC//\\/\\\\}; EXEC=${EXEC//\"/\\\"}; EXEC=${EXEC//\`/\\\`}; EXEC=${EXEC//\$/\\\$}
EXEC=${EXEC//\\/\\\\}
cat > "$HOME/.local/share/applications/hufflabs-ro.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=HuffLabs RO
Comment=Ragnarok Online on the HuffLabs server
Exec="$EXEC"
Icon=hufflabs-ro
Categories=Game;
EOF
command -v update-desktop-database >/dev/null && update-desktop-database "$HOME/.local/share/applications" 2>/dev/null || true

say "Done. Open HuffLabs RO from your app launcher, or run: hufflabs-ro"
