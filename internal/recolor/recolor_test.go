package recolor

import (
	"image"
	"image/color"
	"testing"
)

func lum(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 65535
}

// src is a 4x20 test window: column 0 magenta, 1 white, 2 field grey, 3 label blue.
func src() *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, 4, 20))
	for y := 0; y < 20; y++ {
		m.Set(0, y, color.RGBA{255, 0, 255, 255})
		m.Set(1, y, color.RGBA{255, 255, 255, 255})
		m.Set(2, y, color.RGBA{242, 242, 242, 255})
		m.Set(3, y, color.RGBA{74, 90, 154, 255})
	}
	return m
}

func TestWindowKeepsMagenta(t *testing.T) {
	out := Window(src(), 5)
	for y := 0; y < 20; y++ {
		if c := out.RGBAAt(0, y); c != (color.RGBA{255, 0, 255, 255}) {
			t.Fatalf("magenta at y=%d became %v", y, c)
		}
	}
}

func TestWindowBodyGoesDarkLabelsLight(t *testing.T) {
	out := Window(src(), 5)
	if l := lum(out.At(1, 10)); l > 0.15 {
		t.Errorf("white body luminance %.2f, want dark (<0.15)", l)
	}
	if lum(out.At(3, 10)) <= lum(out.At(1, 10))+0.3 {
		t.Errorf("label not clearly lighter than body: %.2f vs %.2f", lum(out.At(3, 10)), lum(out.At(1, 10)))
	}
}

func TestWindowFieldStaysLight(t *testing.T) {
	out := Window(src(), 5)
	if l := lum(out.At(2, 10)); l < 0.8 {
		t.Errorf("input field luminance %.2f, want light (>=0.8) so typed black text shows", l)
	}
}

func TestWindowTitleIsPurple(t *testing.T) {
	out := Window(src(), 5)
	c := out.RGBAAt(1, 2) // white in the title rows
	if !(c.B > c.G+40 && c.R > c.G+20) {
		t.Errorf("title color %v isn't violet", c)
	}
}

func TestButtonTextLightOnPurple(t *testing.T) {
	m := image.NewRGBA(image.Rect(0, 0, 3, 1))
	m.Set(0, 0, color.RGBA{255, 255, 255, 255}) // button face
	m.Set(1, 0, color.RGBA{0, 0, 0, 255})       // text
	m.Set(2, 0, color.RGBA{255, 0, 255, 255})
	out := Button(m, false)
	face, text := out.RGBAAt(0, 0), out.RGBAAt(1, 0)
	if !(face.B > face.G+40) || lum(face) > 0.5 {
		t.Errorf("face %v isn't a dark violet", face)
	}
	if lum(text) < 0.9 {
		t.Errorf("text %v isn't light", text)
	}
	if out.RGBAAt(2, 0) != (color.RGBA{255, 0, 255, 255}) {
		t.Error("magenta not kept")
	}
	if lum(Button(m, true).At(0, 0)) <= lum(face) {
		t.Error("hover face should be lighter than normal")
	}
}

func TestLoginBox(t *testing.T) {
	m := image.NewNRGBA(image.Rect(0, 0, 5, 1))
	m.SetNRGBA(0, 0, color.NRGBA{255, 255, 255, 128}) // translucent panel
	m.SetNRGBA(1, 0, color.NRGBA{255, 255, 255, 255}) // input field
	m.SetNRGBA(2, 0, color.NRGBA{120, 170, 240, 255}) // blue Login bubble
	m.SetNRGBA(3, 0, color.NRGBA{0, 0, 0, 0})         // fully transparent
	m.SetNRGBA(4, 0, color.NRGBA{240, 150, 150, 255}) // pink poring
	out := LoginBox(m)
	panel := out.NRGBAAt(0, 0)
	if lum(color.RGBA{panel.R, panel.G, panel.B, 255}) > 0.15 || panel.A < 200 {
		t.Errorf("panel = %v, want dark and mostly opaque", panel)
	}
	if out.NRGBAAt(1, 0) != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("field changed to %v", out.NRGBAAt(1, 0))
	}
	b := out.NRGBAAt(2, 0)
	if !(b.R > b.G+30 && b.B > b.G+30) {
		t.Errorf("bubble %v isn't violet", b)
	}
	if out.NRGBAAt(3, 0).A != 0 {
		t.Error("transparent pixel became visible")
	}
	if out.NRGBAAt(4, 0) != (color.NRGBA{240, 150, 150, 255}) {
		t.Errorf("poring changed to %v", out.NRGBAAt(4, 0))
	}
}
