#!/bin/bash
# Builds the HuffLabs RO client theme into theme/out/ (mirrors the client folder),
# plus the website images. Inputs: the three art PNGs in ~/Pictures and the
# English login bitmaps already in the client's data folder.
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/settings.sh
ADMIN_NAME=${HLRO_ADMIN_NAME:-the admin}   # named on the in-game rules page
ART=${ART:-$HOME/Pictures}
CLIENT=${CLIENT:-$HOME/Games/Ragnarok/Client}
BG="$ART/Ravens Over the Amethyst Cathedral.png"
CREST="$ART/Hufflabs RO Gothic Raven Crest.png"
EMBLEM="$ART/Gothic Silver H Shield Emblem.png"
# The English patch stores 유저인터페이스 as its CP949 bytes read as Windows-1252,
# which is the name the client asks for on an English-locale system.
UI='À¯ÀúÀÎÅÍÆäÀÌ½º'
OUT=theme/out
UIOUT="$OUT/data/texture/$UI"
LOGIN_SRC="$CLIENT/data/texture/$UI/login_interface"

for f in "$BG" "$CREST" "$EMBLEM" "$LOGIN_SRC/win_login.bmp"; do
	[ -f "$f" ] || { echo "missing: $f" >&2; exit 1; }
done
rm -rf "$OUT"
mkdir -p "$UIOUT/login_interface" "$OUT/site"
go build -o build/themetool ./cmd/themetool
go build -o build/grftool ./cmd/grftool

echo "== login windows and buttons (dark amethyst)"
for w in win_login win_service; do
	build/themetool window "$LOGIN_SRC/$w.bmp" "$UIOUT/login_interface/$w.bmp" 17
done
for b in btn_connect btn_request btn_intro btn_ok btn_cancel; do  # btn_exit is drawn in login-box.sh
	[ -f "$LOGIN_SRC/$b.bmp" ] && build/themetool button "$LOGIN_SRC/$b.bmp" "$UIOUT/login_interface/$b.bmp"
	for s in a b; do
		[ -f "$LOGIN_SRC/${b}_$s.bmp" ] && build/themetool button "$LOGIN_SRC/${b}_$s.bmp" "$UIOUT/login_interface/${b}_$s.bmp" hover
	done
done

echo "== login background and loading screens (640x480)"
# The art is 16:9; each screen is a 4:3 crop (geometry on a 1672-wide copy) with the crest.
W=1672
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
magick "$BG" -resize "${W}x" "$TMP/bg.png"
H=$(magick identify -format '%h' "$TMP/bg.png")
CW=$((H * 4 / 3))
magick "$CREST" -resize 160x "$TMP/crest-small.png"
magick "$CREST" -resize 256x "$TMP/crest-big.png"
crop() { # name geometry crest-gravity crest-file
	magick "$TMP/bg.png" -crop "$2" +repage -resize 640x480^ -gravity center -extent 640x480 \
		"$4" -gravity "$3" -geometry +14+12 -composite -strip "$TMP/$1.png"
}
crop loading00 "760x570+0+180" southeast "$TMP/crest-small.png"                    # left: ruins and waterfall
crop loading01 "${CW}x${H}+$(((W - CW) / 2))+0" southeast "$TMP/crest-small.png"   # centre
crop loading02 "${CW}x${H}+$((W - CW))+0" southeast "$TMP/crest-small.png"         # right: characters
crop loading03 "560x420+520+0" southeast "$TMP/crest-small.png"                    # the moon
crop loading04 "800x600+870+200" southeast "$TMP/crest-small.png"                  # the three heroes
crop loading05 "660x495+1012+0" southeast "$TMP/crest-small.png"                   # stained glass and crystals
crop loading06 "720x540+640+380" southeast "$TMP/crest-small.png"                  # the slime
# Full picture letterboxed on the theme's dark violet.
magick "$TMP/bg.png" -resize 640x -background '#0d0a16' -gravity center -extent 640x480 \
	"$TMP/crest-small.png" -gravity southeast -geometry +14+12 -composite -strip "$TMP/loading07.png"
for i in 0 1 2 3 4 5 6 7; do
	magick "$TMP/loading0$i.png" -quality 90 "$UIOUT/loading0$i.jpg"
done
# Old-style login background, crest top-left at 40% width.
magick "$TMP/bg.png" -crop "${CW}x${H}+$(((W - CW) / 2))+0" +repage -resize 640x480! \
	\( "$CREST" -resize 256x \) -gravity northwest -geometry +16+12 -composite \
	-type TrueColor "BMP3:$UIOUT/bgi_temp.bmp"

echo "== login screen background (t_login.jpg, 1024x768) and welcome page (warning2.bmp)"
magick "$TMP/bg.png" -resize 1024x768^ -gravity center -extent 1024x768 \
	\( "$CREST" -resize 380x \) -gravity northwest -geometry +24+18 -composite -quality 92 "$UIOUT/t_login.jpg"
# The client shows warning2.bmp (1920x1440) first with a "Do you agree?" box near the
# middle, so the rules panel sits above it.
FONT=${FONT:-/usr/share/fonts/noto/NotoSerif-Regular.ttf}
FONTB=${FONTB:-/usr/share/fonts/noto/NotoSerif-Bold.ttf}
magick "$BG" -resize 1920x1440^ -gravity center -extent 1920x1440 -fill '#0a0712' -colorize 45% \
	\( "$CREST" -resize 820x \) -gravity north -geometry +0+40 -composite \
	-fill 'rgba(14,10,24,0.86)' -stroke '#b98cff' -strokewidth 3 -draw 'roundrectangle 330,470 1590,830 24,24' \
	-stroke none -fill '#f2ecff' -font "$FONTB" -pointsize 52 -gravity north -annotate +0+500 'Welcome to HuffLabs RO' \
	-fill '#c9a2ff' -font "$FONT" -pointsize 30 -annotate +0+570 'A private Ragnarok Online server for friends. House rules:' \
	-fill '#e9e4f2' -pointsize 32 -gravity northwest \
	-annotate +420+630 '•  Be kind. No harassment, slurs or griefing.' \
	-annotate +420+672 '•  No bots, macros or cheat programs.' \
	-annotate +420+714 "•  Found a bug or exploit? Tell $ADMIN_NAME instead of using it." \
	-annotate +420+756 '•  Never share your account or ask for anyone else'"'"'s password.' \
	-fill '#c9a2ff' -font "$FONTB" -pointsize 30 -gravity north -annotate +0+858 'Press OK to agree and play.' \
	-type TrueColor "BMP3:$UIOUT/login_interface/warning2.bmp"

echo "== login box (drawn new: dark amethyst glass)"
# The client checks the loose folder for every login-box image (Wine file trace).
theme/login-box.sh "$UIOUT/login_interface" "$EMBLEM" "$FONTB"

echo "== icons"
magick "$EMBLEM" -define icon:auto-resize=256,128,64,48,32,16 "$OUT/HuffLabsRO.ico"
magick "$EMBLEM" -resize 256x256 "$OUT/HuffLabsRO.png"

echo "== website images"
magick "$BG" -resize 1600x -quality 82 -strip "$OUT/site/bg.jpg"
magick "$CREST" -resize 900x -strip "$OUT/site/crest.png"
magick "$EMBLEM" -define icon:auto-resize=48,32,16 "$OUT/site/favicon.ico"
cp "$OUT/site/bg.jpg" "$OUT/site/crest.png" "$OUT/site/favicon.ico" internal/web/static/

find "$OUT" -type f | sort | sed "s#^$OUT/##"
