package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestWritePreview writes PNGs of each screen when HLRO_PREVIEW is a folder.
func TestWritePreview(t *testing.T) {
	out := os.Getenv("HLRO_PREVIEW")
	if out == "" {
		t.Skip("set HLRO_PREVIEW to a folder to write previews")
	}
	for name, v := range demoViews() {
		img := image.NewRGBA(image.Rect(0, 0, W, H))
		render(img, v, uiState{tick: 20})
		f, err := os.Create(filepath.Join(out, "launcher-"+name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, img)
		f.Close()
	}
}
