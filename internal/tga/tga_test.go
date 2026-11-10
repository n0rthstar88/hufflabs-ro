package tga

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestEncodeBottomUpLikeTheClient(t *testing.T) {
	m := image.NewNRGBA(image.Rect(0, 0, 2, 3))
	m.SetNRGBA(0, 0, color.NRGBA{10, 20, 30, 255}) // top-left
	m.SetNRGBA(0, 2, color.NRGBA{40, 50, 60, 128}) // bottom-left
	var buf bytes.Buffer
	if err := Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if len(b) != 18+2*3*4 {
		t.Fatalf("len %d", len(b))
	}
	if b[2] != 2 || b[16] != 32 || b[17] != 0x08 || b[12] != 2 || b[14] != 3 {
		t.Errorf("header % x", b[:18])
	}
	// First stored pixel is the bottom-left one, as BGRA.
	if got := b[18:22]; !bytes.Equal(got, []byte{60, 50, 40, 128}) {
		t.Errorf("first stored pixel % x, want bottom-left BGRA", got)
	}
	// Last stored row is the top row.
	if got := b[18+2*2*4 : 18+2*2*4+4]; !bytes.Equal(got, []byte{30, 20, 10, 255}) {
		t.Errorf("top-left pixel stored as % x", got)
	}
}
