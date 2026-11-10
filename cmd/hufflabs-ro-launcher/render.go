package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Window size and layout, in pixels. The look follows the website (site.css).
const W, H = 640, 360

var (
	panelRect = image.Rect(40, 186, 600, 340)
	barRect   = image.Rect(64, 266, 576, 278)
	closeRect = image.Rect(W-38, 8, W-8, 38)
	playRect  = image.Rect(436, 294, 576, 326)
	retryRect = image.Rect(316, 294, 428, 326)
)

var (
	ink      = color.RGBA{0xe9, 0xe4, 0xf2, 0xff}
	muted    = color.RGBA{0xa9, 0x9f, 0xbd, 0xff}
	edge     = color.RGBA{0xb9, 0x8c, 0xff, 0xff}
	btnTop   = color.RGBA{0x7a, 0x3f, 0xd1, 0xff}
	btnBot   = color.RGBA{0x4b, 0x1f, 0x8f, 0xff}
	glass    = color.NRGBA{14, 10, 24, 219}
	field    = color.NRGBA{11, 8, 20, 217}
	fieldEdg = color.RGBA{0x5d, 0x54, 0x70, 0xff}
	ghostBg  = color.NRGBA{20, 16, 30, 153}
	ghostEdg = color.RGBA{0x8d, 0x88, 0x99, 0xff}
)

//go:embed assets/bg.jpg
var bgJPG []byte

//go:embed assets/crest.png
var crestPNG []byte

var (
	bgImg, crestImg image.Image
	titleFace       font.Face
	bodyFace        font.Face
	btnFace         font.Face
)

func init() {
	bgImg = mustDecode(bgJPG)
	crestImg = mustDecode(crestPNG)
	titleFace = mustFace(gobold.TTF, 18)
	bodyFace = mustFace(goregular.TTF, 13)
	btnFace = mustFace(gobold.TTF, 13)
}

func mustDecode(b []byte) image.Image {
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	return img
}

func mustFace(ttf []byte, size float64) font.Face {
	f, err := opentype.Parse(ttf)
	if err != nil {
		panic(err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	return face
}

type button struct {
	r      image.Rectangle
	choice Choice
	label  string
	main   bool
}

// buttons are the clickable areas for v; the close button is always there.
func buttons(v View) []button {
	b := []button{{r: closeRect, choice: Quit, label: "×"}}
	switch {
	case v.CanRetry && v.CanPlay:
		b = append(b, button{r: retryRect, choice: Retry, label: "Retry"}, button{r: playRect, choice: PlayAnyway, label: "Play anyway", main: true})
	case v.CanRetry:
		b = append(b, button{r: playRect, choice: Retry, label: "Retry", main: true})
	case v.CanPlay:
		b = append(b, button{r: playRect, choice: PlayAnyway, label: "Play anyway", main: true})
	}
	return b
}

func hit(bs []button, p image.Point) (Choice, bool) {
	for _, b := range bs {
		if p.In(b.r) {
			return b.choice, true
		}
	}
	return 0, false
}

type uiState struct {
	hover Choice
	tick  int // animation frame, for the busy bar
}

func render(dst *image.RGBA, v View, ui uiState) {
	draw.Draw(dst, dst.Bounds(), bgImg, image.Point{}, draw.Src)
	// darken towards the bottom like the site's poster gradient
	for y := 0; y < H; y++ {
		a := uint8(40 + 150*float64(y)/H)
		draw.Draw(dst, image.Rect(0, y, W, y+1), image.NewUniform(color.NRGBA{10, 7, 18, a}), image.Point{}, draw.Over)
	}
	cb := crestImg.Bounds()
	cx := (W - cb.Dx()) / 2
	draw.Draw(dst, image.Rect(cx, 22, cx+cb.Dx(), 22+cb.Dy()), crestImg, cb.Min, draw.Over)

	// glass panel with a soft purple glow
	for i := 6; i >= 1; i-- {
		roundRect(dst, panelRect.Inset(-i), 10+i, color.NRGBA{150, 90, 255, uint8(14 - 2*i + 4)})
	}
	roundRect(dst, panelRect, 10, color.NRGBA{185, 140, 255, 153})
	roundRect(dst, panelRect.Inset(1), 9, glass)

	text(dst, titleFace, ink, image.Pt(64, 222), v.Title)
	text(dst, bodyFace, muted, image.Pt(64, 246), v.Detail)

	if !(v.CanRetry || v.CanPlay) || v.Fraction >= 0 {
		roundRect(dst, barRect, 6, color.NRGBA{fieldEdg.R, fieldEdg.G, fieldEdg.B, 255})
		roundRect(dst, barRect.Inset(1), 5, field)
		fill := barRect
		if v.Fraction >= 0 {
			fill.Max.X = fill.Min.X + int(math.Round(float64(barRect.Dx())*math.Min(v.Fraction, 1)))
		} else { // busy: a segment sweeping across
			seg := 120
			pos := (ui.tick * 8) % (barRect.Dx() + seg)
			fill.Min.X = barRect.Min.X + pos - seg
			fill.Max.X = fill.Min.X + seg
			fill = fill.Intersect(barRect)
		}
		if !fill.Empty() {
			gradient(dst, fill, btnTop, btnBot)
		}
	}

	if v.Phase == Downloading || v.Phase == Scanning {
		text(dst, bodyFace, muted, image.Pt(64, 314), "HuffLabs RO starts by itself when this finishes.")
	}

	for _, b := range buttons(v) {
		hover := ui.hover == b.choice
		switch {
		case b.choice == Quit:
			bg := color.NRGBA{14, 10, 24, 150}
			if hover {
				bg = color.NRGBA{122, 63, 209, 220}
			}
			roundRect(dst, b.r, b.r.Dx()/2, bg)
			centered(dst, titleFace, ink, b.r, b.label)
			continue
		case b.main:
			roundRect(dst, b.r.Inset(-3), 9, color.NRGBA{170, 110, 255, 70})
			roundRect(dst, b.r, 6, color.NRGBA{edge.R, edge.G, edge.B, 255})
			inner := b.r.Inset(1)
			top, bot := btnTop, btnBot
			if hover {
				top, bot = brighten(top), brighten(bot)
			}
			gradient(dst, inner, top, bot)
			centered(dst, btnFace, color.RGBA{255, 255, 255, 255}, b.r, b.label)
		default:
			roundRect(dst, b.r, 6, color.NRGBA{ghostEdg.R, ghostEdg.G, ghostEdg.B, 255})
			bg := ghostBg
			if hover {
				bg.A = 220
			}
			roundRect(dst, b.r.Inset(1), 5, bg)
			centered(dst, btnFace, color.RGBA{0xe0, 0xdb, 0xe9, 0xff}, b.r, b.label)
		}
	}
}

func brighten(c color.RGBA) color.RGBA {
	f := func(v uint8) uint8 { return uint8(math.Min(255, float64(v)*1.15)) }
	return color.RGBA{f(c.R), f(c.G), f(c.B), c.A}
}

// roundRect blends c over r with rounded corners of radius rad (anti-aliased).
func roundRect(dst *image.RGBA, r image.Rectangle, rad int, c color.Color) {
	r = r.Intersect(dst.Bounds())
	src := image.NewUniform(c)
	mask := image.NewAlpha(r)
	fr := float64(rad)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			dx := math.Max(0, math.Max(float64(r.Min.X)+fr-px, px-(float64(r.Max.X)-fr)))
			dy := math.Max(0, math.Max(float64(r.Min.Y)+fr-py, py-(float64(r.Max.Y)-fr)))
			d := math.Hypot(dx, dy)
			a := math.Max(0, math.Min(1, fr-d+0.5))
			mask.SetAlpha(x, y, color.Alpha{uint8(a * 255)})
		}
	}
	draw.DrawMask(dst, r, src, image.Point{}, mask, r.Min, draw.Over)
}

func gradient(dst *image.RGBA, r image.Rectangle, top, bot color.RGBA) {
	h := r.Dy()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		t := 0.0
		if h > 1 {
			t = float64(y-r.Min.Y) / float64(h-1)
		}
		lerp := func(a, b uint8) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }
		c := color.RGBA{lerp(top.R, bot.R), lerp(top.G, bot.G), lerp(top.B, bot.B), 255}
		draw.Draw(dst, image.Rect(r.Min.X, y, r.Max.X, y+1), image.NewUniform(c), image.Point{}, draw.Src)
	}
}

func text(dst *image.RGBA, f font.Face, c color.Color, at image.Point, s string) {
	d := font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: f, Dot: fixed.P(at.X, at.Y)}
	d.DrawString(s)
}

func centered(dst *image.RGBA, f font.Face, c color.Color, r image.Rectangle, s string) {
	w := font.MeasureString(f, s).Round()
	m := f.Metrics()
	h := (m.Ascent + m.Descent).Round()
	x := r.Min.X + (r.Dx()-w)/2
	y := r.Min.Y + (r.Dy()-h)/2 + m.Ascent.Round()
	text(dst, f, c, image.Pt(x, y), s)
}
