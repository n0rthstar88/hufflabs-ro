#!/bin/bash
# Draws the HuffLabs RO login box (dark amethyst glass) at the exact sizes and
# positions of the client's own login_interface images.
# Usage: login-box.sh <out-dir> <emblem.png> <bold-font.ttf>
# The client draws black text for "Integrated Account", "Save ID", "Opening" and
# "Sign Up", so the fields, buttons and the Save ID strip stay light.
set -euo pipefail
OUT=${1:?out dir}
EMBLEM=${2:?emblem png}
FONTB=${3:?bold font}
mkdir -p "$OUT"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

EDGE='#b98cff'     # glowing violet edge
FIELD='#f6f3fb'    # light input fields
FIELD_EDGE='#7d68a8'
SILVER_TOP='#f4f1f9'
SILVER_BOT='#c8bfd9'

# Magenta-keyed BMP: alpha is made all-or-nothing, then flattened on #FF00FF,
# so no pink fringe appears around shapes.
keyed() { # in.png out.bmp
	magick "$1" -channel A -threshold 50% +channel -background '#FF00FF' -flatten -type TrueColor "BMP3:$2"
}

echo "-- bg_login.tga (301x132)"
# Silhouette: main panel, the top band running under the close button, and the
# ring the round Login button sits in (centre 232,71).
magick -size 301x132 xc:none -fill white \
	-draw 'roundrectangle 0,0 250,131 10,10' \
	-draw 'roundrectangle 0,0 283,33 10,10' \
	-draw 'circle 232,71 232,19' "$TMP/mask.png"
magick -size 301x132 gradient:'#22183a-#0d0918' "$TMP/grad.png"
magick "$TMP/grad.png" "$TMP/mask.png" -compose CopyOpacity -composite \
	-channel A -evaluate multiply 0.94 +channel "$TMP/panel.png"
# Violet edge: the silhouette's outline.
magick "$TMP/mask.png" -alpha extract -morphology EdgeIn Diamond:1 -background "$EDGE" -alpha shape "$TMP/edge.png"
magick "$TMP/panel.png" "$TMP/edge.png" -composite \
	-fill none -stroke '#3b2a5e' -strokewidth 4 -draw 'circle 232,71 232,23' \
	-stroke '#8f64e0' -strokewidth 1 -draw 'circle 232,71 232,21' \
	-stroke "$FIELD_EDGE" -strokewidth 1 -fill "$FIELD" \
	-draw 'roundrectangle 13,10 151,31 3,3' \
	-draw 'roundrectangle 13,37 172,57 3,3' \
	-draw 'roundrectangle 13,61 172,81 3,3' \
	-stroke none -fill '#d9d1e8' -draw 'roundrectangle 52,83 116,98 4,4' \
	-fill 'rgba(185,140,255,0.55)' -draw 'rectangle 14,101 172,101' \
	"$TMP/bg_login.png"
# The client expects bottom-up rows and ignores the TGA origin flag; ImageMagick
# writes top-down regardless of its origin option, so themetool writes the TGA.
"${THEMETOOL:-build/themetool}" tga "$TMP/bg_login.png" "$OUT/bg_login.tga"

echo "-- bt_start_* (84x84, round Login button)"
orb() { # name inner outer rim
	# Drawn at 4x and shrunk with Lanczos so edges and the emblem are smooth. The
	# edge is blended into the panel colour behind the button, and only pixels
	# outside the circle become magenta, so there is no jagged pink fringe.
	magick -size 336x336 xc:'#1c1430' \
		\( -size 320x320 radial-gradient:"$2-$3" \( -size 320x320 xc:black -fill white -draw 'circle 160,160 160,0' \) \
			-alpha off -compose CopyOpacity -composite \) -compose Over -geometry +8+8 -composite \
		-fill none -stroke "$4" -strokewidth 7 -draw 'circle 168,168 168,12' \
		-stroke 'rgba(255,255,255,0.25)' -strokewidth 3 -draw 'arc 40,40 296,296 200,290' \
		\( "$EMBLEM" -filter Lanczos -resize 176x176 \) -gravity center -geometry +0+0 -composite \
		-gravity south -font "$FONTB" -pointsize 38 -stroke none -fill 'rgba(20,8,40,0.8)' -annotate +0+38 'LOGIN' \
		-fill white -annotate +0+41 'LOGIN' \
		-filter Lanczos -resize 84x84 "$TMP/$1-rgb.png"
	# Outside the orb, fill with the panel ring's colour out to r=47 so the client's
	# texture smoothing blends magenta into dark ring, not into the orb's bright rim.
	magick "$TMP/$1-rgb.png" \( -size 84x84 xc:none -fill '#2a1d45' -draw 'circle 41.5,41.5 41.5,-5.5' \
		-fill black -draw 'circle 41.5,41.5 41.5,1.5' -transparent black \) -compose Over -composite \
		\( -size 84x84 xc:'#FF00FF' -fill black -draw 'circle 41.5,41.5 41.5,-5.5' -transparent black \) \
		-compose Over -composite -type TrueColor "BMP3:$OUT/$1.bmp"
}
orb bt_start_normal '#9a6cf0' '#3f1a7c' '#e2d0ff'
orb bt_start_over '#b38cff' '#5a2ea6' '#ffffff'
orb bt_start_press '#7a4fd0' '#2c1258' '#cdb6ff'

echo "-- text buttons (Opening 67x21, Sign Up 84x21) and close buttons"
pill() { # name w h top bottom edge
	magick -size "$2x$3" gradient:"$4-$5" \
		\( -size "$2x$3" xc:black -fill white -draw "roundrectangle 0,0 $(($2 - 1)),$(($3 - 1)) 5,5" \) \
		-alpha off -compose CopyOpacity -composite -compose Over \
		-fill none -stroke "$6" -strokewidth 1 -draw "roundrectangle 0,0 $(($2 - 1)),$(($3 - 1)) 5,5" "$TMP/$1.png"
	keyed "$TMP/$1.png" "$OUT/$1.bmp"
}
for b in bt_close bt_join; do
	w=67
	[ "$b" = bt_join ] && w=84
	pill "${b}_normal" $w 21 "$SILVER_TOP" "$SILVER_BOT" "$EDGE"
	pill "${b}_over" $w 21 '#f1e8ff' '#c4a6f5' '#d9c2ff'
	pill "${b}_press" $w 21 '#d5c4f2' '#a487de' "$EDGE"
done
closex() { # name disc x-color
	magick -size 19x19 xc:none -fill "$2" -stroke "$EDGE" -strokewidth 1 -draw 'circle 9,9 9,1' \
		-stroke "$3" -strokewidth 2 -draw 'line 6,6 12,12' -draw 'line 12,6 6,12' "$TMP/$1.png"
	keyed "$TMP/$1.png" "$OUT/$1.bmp"
}
closex bt_close2_normal '#1d1430' '#e2d0ff'
closex bt_close2_over '#3a2566' '#ffffff'
closex bt_close2_press '#120c1f' '#b98cff'

echo "-- dropdown arrow (22x22) and checkboxes (12x12)"
drop() { # name top bottom
	magick -size 22x22 gradient:"$2-$3" -fill none -stroke "$FIELD_EDGE" -strokewidth 1 -draw 'rectangle 0,0 21,21' \
		-stroke none -fill '#4b1f8f' -draw 'polygon 6,8 16,8 11,14' -type TrueColor "BMP3:$OUT/$1.bmp"
}
drop btn_dropdown_out "$SILVER_TOP" "$SILVER_BOT"
drop btn_dropdown_over '#f1e8ff' '#c4a6f5'
drop btn_dropdown_press '#d5c4f2' '#a487de'
magick -size 12x12 xc:"$FIELD" -fill none -stroke "$FIELD_EDGE" -draw 'rectangle 0,0 11,11' -type TrueColor "BMP3:$OUT/checkbox_off.bmp"
magick -size 12x12 xc:"$FIELD" -fill none -stroke "$FIELD_EDGE" -draw 'rectangle 0,0 11,11' \
	-stroke '#5a2ea6' -strokewidth 2 -draw 'polyline 3,6 5,9 9,3' -type TrueColor "BMP3:$OUT/checkbox_on.bmp"

echo "-- message box (win_msgbox 280x120) and its OK/cancel buttons"
# Title is part of the bitmap (white on violet). The client prints the question in
# black, so the body stays light; the footer is dark glass for the buttons.
UIDIR=$(dirname "$OUT")
FONTS=${FONTS:-/usr/share/fonts/noto/NotoSans-Bold.ttf}
magick -size 280x120 xc:'#f5f1fc' \
	\( -size 280x17 gradient:'#5a2ca8-#2c1758' \) -geometry +0+0 -composite \
	\( -size 280x31 gradient:'#22183a-#0d0918' \) -geometry +0+89 -composite \
	-fill '#b98cff' -draw 'rectangle 0,17 279,17' -draw 'rectangle 0,88 279,88' \
	-fill none -stroke '#b98cff' -strokewidth 1 -draw 'rectangle 0,0 279,119' \
	-stroke none -fill '#e9dcff' -draw 'circle 9,8 9,4' -fill '#5a2ca8' -draw 'circle 9,8 9,6' \
	-font "$FONTS" -pointsize 10.5 -fill white -gravity northwest -annotate +18+2 'Message' \
	-type TrueColor "BMP3:$UIDIR/win_msgbox.bmp"
textbtn() { # name label top bottom
	magick -size 42x20 gradient:"$3-$4" \
		\( -size 42x20 xc:black -fill white -draw 'roundrectangle 0,0 41,19 5,5' \) \
		-alpha off -compose CopyOpacity -composite -compose Over \
		-fill none -stroke '#b98cff' -strokewidth 1 -draw 'roundrectangle 0,0 41,19 5,5' \
		-stroke none -font "$FONTS" -pointsize 11 -fill '#2a1550' -gravity center -annotate +0+0 "$2" "$TMP/$1.png"
	keyed "$TMP/$1.png" "$UIDIR/$1.bmp"
}
for b in ok cancel; do
	label=OK
	[ $b = cancel ] && label=Cancel
	textbtn "btn_$b" "$label" "$SILVER_TOP" "$SILVER_BOT"
	textbtn "btn_${b}_a" "$label" '#f1e8ff' '#c4a6f5'
	textbtn "btn_${b}_b" "$label" '#d5c4f2' '#a487de'
done
textbtn_in() { # dir name label top bottom
	local d=$UIDIR
	UIDIR=$1 textbtn "$2" "$3" "$4" "$5"
	UIDIR=$d
}
for s in "" _a _b; do
	case $s in "") top=$SILVER_TOP bot=$SILVER_BOT ;; _a) top='#f1e8ff' bot='#c4a6f5' ;; _b) top='#d5c4f2' bot='#a487de' ;; esac
	textbtn_in "$OUT" "btn_exit$s" Exit "$top" "$bot"
done

echo "-- generic window frame (Select Service, quit and other dialogs)"
# Title text is printed black by the client, so the title bar is light silver with
# a violet edge; the bottom button bar is dark glass under silver buttons.
BI="$UIDIR/basic_interface"
mkdir -p "$BI"
title() { # name left-edge right-edge
	magick -size 12x17 xc:'#7d5cc4' \( -size 12x16 gradient:'#f3eefc-#cbbde6' \) -geometry +0+0 -composite \
		-fill "$EDGE" -draw 'line 0,0 11,0' "$TMP/$1.png"
	[ "$2" = 1 ] && magick "$TMP/$1.png" -fill "$EDGE" -draw 'line 0,0 0,16' -fill '#FF00FF' -draw 'point 0,0' -draw 'point 1,0' -draw 'point 0,1' "$TMP/$1.png"
	[ "$3" = 1 ] && magick "$TMP/$1.png" -fill "$EDGE" -draw 'line 11,0 11,16' -fill '#FF00FF' -draw 'point 11,0' -draw 'point 10,0' -draw 'point 11,1' "$TMP/$1.png"
	magick "$TMP/$1.png" -type TrueColor "BMP3:$BI/$1.bmp"
}
title titlebar_left 1 0
title titlebar_mid 0 0
title titlebar_right 0 1
bar() { # name left-edge right-edge
	magick -size 21x29 gradient:'#22183a-#0d0918' -fill "$EDGE" -draw 'line 0,0 20,0' -draw 'line 0,28 20,28' "$TMP/$1.png"
	[ "$2" = 1 ] && magick "$TMP/$1.png" -fill "$EDGE" -draw 'line 0,0 0,28' -fill '#FF00FF' -draw 'point 0,28' -draw 'point 1,28' -draw 'point 0,27' "$TMP/$1.png"
	[ "$3" = 1 ] && magick "$TMP/$1.png" -fill "$EDGE" -draw 'line 20,0 20,28' -fill '#FF00FF' -draw 'point 20,28' -draw 'point 19,28' -draw 'point 20,27' "$TMP/$1.png"
	magick "$TMP/$1.png" -type TrueColor "BMP3:$BI/$1.bmp"
}
bar btnbar_left2 1 0
bar btnbar_mid2 0 0
bar btnbar_right2 0 1
magick -size 44x44 xc:none \( -size 36x36 radial-gradient:'#c9a2ff-#4b1f8f' \( -size 36x36 xc:black -fill white -draw 'circle 18,18 18,0' \) \
	-alpha off -compose CopyOpacity -composite \) -compose Over -geometry +4+4 -composite -filter Lanczos -resize 11x11 "$TMP/sysicon.png"
magick -size 11x11 xc:'#f3eefc' "$TMP/sysicon.png" -composite -fill none -stroke '#FF00FF' -draw 'rectangle 0,0 10,10' \
	-type TrueColor "BMP3:$BI/sys_base_off.bmp"
