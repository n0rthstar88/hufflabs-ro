// Package recolor turns RO's light-blue login bitmaps into the HuffLabs RO
// "dark amethyst glass" theme. Pure magenta (RO's transparency key) is kept.
package recolor

import (
	"image"
	"image/color"
)

var (
	magenta = color.RGBA{255, 0, 255, 255}

	bodyDark  = color.RGBA{0x16, 0x0f, 0x24, 255} // what white body becomes
	bodyLight = color.RGBA{0xec, 0xe2, 0xff, 255} // what black text becomes
	field     = color.RGBA{0xee, 0xe8, 0xf8, 255} // input boxes stay light for black typed text
	titleDeep = color.RGBA{0x5a, 0x2c, 0xa8, 255} // light title-bar pixels
	btnFace   = color.RGBA{0x55, 0x2a, 0x9c, 255}
	btnHover  = color.RGBA{0x7c, 0x48, 0xd6, 255}
	white     = color.RGBA{255, 255, 255, 255}
)

func luminance(c color.RGBA) float64 {
	return (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255
}

// lerp goes from a (t=0) to b (t=1).
func lerp(a, b color.RGBA, t float64) color.RGBA {
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	f := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return color.RGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), 255}
}

func isField(c color.RGBA) bool {
	near := func(v uint8) bool { return v >= 236 && v <= 246 }
	return near(c.R) && near(c.G) && near(c.B) && c.R == c.G && c.G == c.B
}

func each(src image.Image, f func(x, y int, c color.RGBA) color.RGBA) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.RGBAModel.Convert(src.At(x, y)).(color.RGBA)
			c.A = 255
			if c == magenta {
				out.SetRGBA(x, y, c)
				continue
			}
			out.SetRGBA(x, y, f(x-b.Min.X, y-b.Min.Y, c))
		}
	}
	return out
}

// Window recolors a window bitmap. The top titleRows rows are the title bar.
func Window(src image.Image, titleRows int) *image.RGBA {
	return each(src, func(_, y int, c color.RGBA) color.RGBA {
		l := luminance(c)
		switch {
		case y < titleRows:
			return lerp(white, titleDeep, l) // dark title text -> white, light bar -> violet
		case isField(c):
			return field
		default:
			return lerp(bodyLight, bodyDark, l) // swap light and dark
		}
	})
}

// Button recolors a button bitmap: light face -> violet, dark text -> white.
func Button(src image.Image, hover bool) *image.RGBA {
	face := btnFace
	if hover {
		face = btnHover
	}
	return each(src, func(_, _ int, c color.RGBA) color.RGBA {
		return lerp(white, face, luminance(c))
	})
}

// LoginBox recolors the new-style login box (bg_login.tga, has alpha): the
// translucent white panel becomes mostly-opaque dark violet glass, opaque white
// input fields stay white, blue tones (the Login bubble) turn violet, and
// everything else (the Poring, outlines) is kept.
func LoginBox(src image.Image) *image.NRGBA {
	b := src.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
			switch {
			case c.A == 0:
				// keep fully transparent
			case c.A < 250 && c.R > 200 && c.G > 200 && c.B > 200:
				a := int(c.A) * 7 / 4
				if a > 235 {
					a = 235
				}
				c = color.NRGBA{bodyDark.R, bodyDark.G, bodyDark.B, uint8(a)}
			case int(c.B) > int(c.R)+15 && int(c.B) >= int(c.G):
				c = toViolet(c)
			}
			out.SetNRGBA(x, y, c)
		}
	}
	return out
}

// toViolet moves a blue-ish color's hue to violet (about 275°), keeping its
// lightness and saturation.
func toViolet(c color.NRGBA) color.NRGBA {
	r, g, bl := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx, mn := max(r, g, bl), min(r, g, bl)
	l := (mx + mn) / 2
	if mx == mn {
		return c
	}
	d := mx - mn
	s := d / (1 - abs(2*l-1))
	h := 275.0 / 360
	q := l + s - l*s
	if l < 0.5 {
		q = l * (1 + s)
	}
	p := 2*l - q
	hue := func(t float64) float64 {
		if t < 0 {
			t++
		} else if t > 1 {
			t--
		}
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	u := func(v float64) uint8 { return uint8(v*255 + 0.5) }
	return color.NRGBA{u(hue(h + 1.0/3)), u(hue(h)), u(hue(h - 1.0/3)), c.A}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
