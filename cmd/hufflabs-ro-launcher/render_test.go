package main

import (
	"image"
	"image/color"
	"testing"
)

func TestButtonsFollowView(t *testing.T) {
	if b := buttons(View{}); len(b) != 1 || b[0].choice != Quit {
		t.Errorf("busy view buttons %+v, want only close", b)
	}
	b := buttons(View{CanRetry: true, CanPlay: true})
	if len(b) != 3 {
		t.Fatalf("buttons %+v", b)
	}
	var retry, play image.Rectangle
	for _, x := range b {
		switch x.choice {
		case Retry:
			retry = x.r
		case PlayAnyway:
			play = x.r
		}
		if !x.r.In(image.Rect(0, 0, W, H)) {
			t.Errorf("%v outside window", x.r)
		}
	}
	if retry.Empty() || play.Empty() || retry.Overlaps(play) {
		t.Errorf("retry %v play %v", retry, play)
	}
	if c, ok := hit(b, retry.Min.Add(image.Pt(2, 2))); !ok || c != Retry {
		t.Errorf("hit = %v %v", c, ok)
	}
	if _, ok := hit(b, image.Pt(5, 300)); ok {
		t.Error("hit on empty space")
	}
}

func TestRenderAllPhases(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	for _, v := range []View{
		{Phase: Checking, Title: "Checking for updates…", Fraction: -1},
		{Phase: Downloading, Title: "Updating HuffLabs RO", Detail: "Downloading 3 of 12 files — 41 MB of 120 MB", Fraction: 0.5},
		{Phase: Offline, Title: "Can't reach the HuffLabs server", Detail: "Connect Tailscale, then press Retry.", Fraction: -1, CanRetry: true, CanPlay: true},
	} {
		render(img, v, uiState{hover: Retry, tick: 3})
	}
	// half-full bar: a pixel at 25% of the track is bar purple, one at 75% isn't
	render(img, View{Phase: Downloading, Fraction: 0.5}, uiState{})
	y := barRect.Min.Y + barRect.Dy()/2
	in := img.RGBAAt(barRect.Min.X+barRect.Dx()/4, y)
	out := img.RGBAAt(barRect.Min.X+barRect.Dx()*3/4, y)
	if !purple(in) || purple(out) {
		t.Errorf("bar pixels in=%v out=%v", in, out)
	}
}

func purple(c color.RGBA) bool { return c.B > 120 && c.R > 60 && c.G < c.R && c.G < c.B }
