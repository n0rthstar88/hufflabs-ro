// Command themetool recolors RO login bitmaps into the HuffLabs RO theme.
//
//	themetool window <in.bmp> <out.bmp> <titleRows>
//	themetool button <in.bmp> <out.bmp> [hover]
//	themetool loginbox <in.png> <out.png>
//	themetool tga <in.png> <out.tga>   (bottom-up 32-bit, as the client expects)
package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/image/bmp"

	"hufflabs-ro/internal/recolor"
	"hufflabs-ro/internal/tga"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "themetool:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: themetool window <in> <out> <titleRows> | themetool button <in> <out> [hover]")
	}
	in, err := os.Open(args[1])
	if err != nil {
		return err
	}
	defer in.Close()
	decode, encode := bmp.Decode, bmp.Encode
	switch args[0] {
	case "loginbox":
		decode, encode = png.Decode, png.Encode
	case "tga":
		decode, encode = png.Decode, tga.Encode
	}
	src, err := decode(in)
	if err != nil {
		return fmt.Errorf("%s: %w", args[1], err)
	}
	var out image.Image
	switch args[0] {
	case "window":
		if len(args) != 4 {
			return fmt.Errorf("window needs <titleRows>")
		}
		rows, err := strconv.Atoi(args[3])
		if err != nil {
			return err
		}
		out = recolor.Window(src, rows)
	case "loginbox":
		out = recolor.LoginBox(src)
	case "tga":
		out = src
	case "button":
		out = recolor.Button(src, len(args) > 3 && args[3] == "hover")
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	if err := os.MkdirAll(filepath.Dir(args[2]), 0o755); err != nil {
		return err
	}
	f, err := os.Create(args[2])
	if err != nil {
		return err
	}
	if err := encode(f, out); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
