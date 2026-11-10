// Package grf reads Ragnarok Online GRF archives (versions 0x200 and 0x300), for
// listing and extracting the client's UI textures, and writes small 0x200 archives
// (grftool pack). The released theme ships as loose files under data/texture, not
// as a GRF; packing stays for experiments.
package grf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/text/encoding/korean"
)

const headerSize = 46

const (
	flagFile     = 0x01
	flagMixCrypt = 0x02
	flagDES      = 0x04
)

type Entry struct {
	Name     string // UTF-8, backslash-separated as stored
	Size     uint32
	CompSize uint32
	Flags    uint8
	offset   uint64
	alloc    uint32
}

type Archive struct {
	f       *os.File
	files   []Entry
	byLower map[string]int
}

func key(name string) string { return strings.ToLower(strings.ReplaceAll(name, "/", `\`)) }

func Open(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	a, err := parse(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return a, nil
}

func parse(f *os.File) (*Archive, error) {
	h := make([]byte, headerSize)
	if _, err := io.ReadFull(f, h); err != nil {
		return nil, errors.New("not a GRF (too short)")
	}
	magic := string(h[:15])
	if magic != "Master of Magic" && !strings.HasPrefix(magic, "Event Horizon") {
		return nil, errors.New("not a GRF (bad magic)")
	}
	version := binary.LittleEndian.Uint32(h[42:])
	var tableOff uint64
	var count uint32
	switch version {
	case 0x200:
		tableOff = uint64(binary.LittleEndian.Uint32(h[30:]))
		seed, n := binary.LittleEndian.Uint32(h[34:]), binary.LittleEndian.Uint32(h[38:])
		count = n - seed - 7
	case 0x300:
		tableOff = binary.LittleEndian.Uint64(h[30:])
		count = binary.LittleEndian.Uint32(h[38:])
	default:
		return nil, fmt.Errorf("unsupported GRF version %#x", version)
	}
	if _, err := f.Seek(int64(headerSize+tableOff), io.SeekStart); err != nil {
		return nil, err
	}
	if version == 0x300 {
		if _, err := f.Seek(4, io.SeekCurrent); err != nil { // 0x300 table has 4 extra bytes first
			return nil, err
		}
	}
	var sizes [2]uint32
	if err := binary.Read(f, binary.LittleEndian, &sizes); err != nil {
		return nil, fmt.Errorf("file table: %w", err)
	}
	zr, err := zlib.NewReader(io.LimitReader(f, int64(sizes[0])))
	if err != nil {
		return nil, fmt.Errorf("file table: %w", err)
	}
	table := make([]byte, sizes[1])
	if _, err := io.ReadFull(zr, table); err != nil {
		return nil, fmt.Errorf("file table: %w", err)
	}
	entrySize := 17
	if version == 0x300 {
		entrySize = 21
	}
	dec := korean.EUCKR.NewDecoder()
	a := &Archive{f: f, byLower: make(map[string]int, count)}
	for p := 0; p < len(table); {
		end := bytes.IndexByte(table[p:], 0)
		if end < 0 || p+end+1+entrySize > len(table) {
			return nil, errors.New("file table truncated")
		}
		rawName := table[p : p+end]
		p += end + 1
		e := Entry{
			CompSize: binary.LittleEndian.Uint32(table[p:]),
			alloc:    binary.LittleEndian.Uint32(table[p+4:]),
			Size:     binary.LittleEndian.Uint32(table[p+8:]),
			Flags:    table[p+12],
		}
		if version == 0x300 {
			e.offset = binary.LittleEndian.Uint64(table[p+13:])
		} else {
			e.offset = uint64(binary.LittleEndian.Uint32(table[p+13:]))
		}
		p += entrySize
		if e.Flags&flagFile == 0 {
			continue // directory entry
		}
		name, err := dec.Bytes(rawName)
		if err != nil {
			name = rawName
		}
		e.Name = string(name)
		a.byLower[key(e.Name)] = len(a.files)
		a.files = append(a.files, e)
	}
	return a, nil
}

func (a *Archive) Close() error { return a.f.Close() }

// Files lists file entries (directories skipped) in table order.
func (a *Archive) Files() []Entry { return a.files }

// Read returns a file's contents. Lookups ignore case and slash direction.
func (a *Archive) Read(name string) ([]byte, error) {
	i, ok := a.byLower[key(name)]
	if !ok {
		return nil, fmt.Errorf("%s: not in GRF", name)
	}
	e := a.files[i]
	if e.Flags&(flagMixCrypt|flagDES) != 0 {
		return nil, fmt.Errorf("%s: encrypted entries aren't supported", name)
	}
	comp := make([]byte, e.alloc)
	if _, err := a.f.ReadAt(comp, int64(headerSize+e.offset)); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if e.CompSize == e.Size && e.CompSize == e.alloc && !looksZlib(comp) {
		return comp, nil // stored uncompressed
	}
	zr, err := zlib.NewReader(bytes.NewReader(comp[:e.CompSize]))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	out := make([]byte, e.Size)
	if _, err := io.ReadFull(zr, out); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return out, nil
}

func looksZlib(b []byte) bool {
	return len(b) >= 2 && b[0]&0x0f == 8 && (uint16(b[0])<<8|uint16(b[1]))%31 == 0
}
