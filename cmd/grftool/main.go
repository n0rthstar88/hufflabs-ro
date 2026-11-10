// Command grftool lists and extracts files from a Ragnarok Online GRF.
//
//	grftool list <grf>
//	grftool extract <grf> <name> <out>
//	grftool pack <out.grf> <dir>   (dir holds data/..., names stored with backslashes)
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hufflabs-ro/internal/grf"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "grftool:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: grftool list <grf> | grftool extract <grf> <name> <out>")
	}
	if args[0] == "pack" {
		if len(args) != 3 {
			return fmt.Errorf("usage: grftool pack <out.grf> <dir>")
		}
		return pack(args[1], args[2])
	}
	a, err := grf.Open(args[1])
	if err != nil {
		return err
	}
	defer a.Close()
	switch args[0] {
	case "list":
		for _, e := range a.Files() {
			fmt.Printf("%d\t%s\n", e.Size, e.Name)
		}
		return nil
	case "extract":
		if len(args) != 4 {
			return fmt.Errorf("usage: grftool extract <grf> <name> <out>")
		}
		b, err := a.Read(args[2])
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(args[3]), 0o755); err != nil {
			return err
		}
		return os.WriteFile(args[3], b, 0o644)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func pack(out, dir string) error {
	var files []grf.File
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files = append(files, grf.File{Name: strings.ReplaceAll(rel, "/", `\`), Data: b})
		return nil
	})
	if err != nil {
		return err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	if err := grf.Write(out, files); err != nil {
		return err
	}
	for _, f := range files {
		fmt.Printf("%d\t%s\n", len(f.Data), f.Name)
	}
	return nil
}
