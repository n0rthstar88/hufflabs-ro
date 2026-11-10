// Package tga writes uncompressed 32-bit TGA files the way the RO client expects:
// rows stored bottom-up (descriptor 0x08). The client ignores the origin flag, so
// a top-down file shows upside down.
package tga

import (
	"encoding/binary"
	"image"
	"image/color"
	"io"
)

func Encode(w io.Writer, m image.Image) error {
	b := m.Bounds()
	wd, ht := b.Dx(), b.Dy()
	h := make([]byte, 18)
	h[2] = 2 // uncompressed true-color
	binary.LittleEndian.PutUint16(h[12:], uint16(wd))
	binary.LittleEndian.PutUint16(h[14:], uint16(ht))
	h[16] = 32
	h[17] = 0x08 // 8 alpha bits, bottom-left origin
	if _, err := w.Write(h); err != nil {
		return err
	}
	row := make([]byte, wd*4)
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		for x := 0; x < wd; x++ {
			c := color.NRGBAModel.Convert(m.At(b.Min.X+x, y)).(color.NRGBA)
			row[x*4], row[x*4+1], row[x*4+2], row[x*4+3] = c.B, c.G, c.R, c.A
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
