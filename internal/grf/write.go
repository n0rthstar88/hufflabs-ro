package grf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"os"

	"golang.org/x/text/encoding/korean"
)

// File is one entry to write. Name is UTF-8 with backslashes, e.g. data\texture\...
type File struct {
	Name string
	Data []byte
}

// Write creates a version 0x200 GRF holding files (grftool pack). The released
// theme doesn't use it (see the package doc); data.grf is never rewritten.
func Write(path string, files []File) error {
	enc := korean.EUCKR.NewEncoder()
	var body, table bytes.Buffer
	for _, f := range files {
		name, err := enc.String(f.Name)
		if err != nil {
			return fmt.Errorf("%s: name not encodable as CP949: %w", f.Name, err)
		}
		var c bytes.Buffer
		zw, _ := zlib.NewWriterLevel(&c, zlib.BestCompression)
		zw.Write(f.Data)
		zw.Close()
		comp := c.Bytes()
		off := body.Len()
		body.Write(comp)
		table.WriteString(name)
		table.WriteByte(0)
		binary.Write(&table, binary.LittleEndian, uint32(len(comp))) // compressed
		binary.Write(&table, binary.LittleEndian, uint32(len(comp))) // aligned (no DES padding)
		binary.Write(&table, binary.LittleEndian, uint32(len(f.Data)))
		table.WriteByte(flagFile)
		binary.Write(&table, binary.LittleEndian, uint32(off))
	}
	var ct bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&ct, zlib.BestCompression)
	zw.Write(table.Bytes())
	zw.Close()

	var out bytes.Buffer
	out.WriteString("Master of Magic\x00")
	out.Write(make([]byte, 14)) // unencrypted key
	binary.Write(&out, binary.LittleEndian, uint32(body.Len()))
	binary.Write(&out, binary.LittleEndian, uint32(0))            // seed
	binary.Write(&out, binary.LittleEndian, uint32(len(files)+7)) // file count + seed + 7
	binary.Write(&out, binary.LittleEndian, uint32(0x200))
	out.Write(body.Bytes())
	binary.Write(&out, binary.LittleEndian, uint32(ct.Len()))
	binary.Write(&out, binary.LittleEndian, uint32(table.Len()))
	out.Write(ct.Bytes())
	return os.WriteFile(path, out.Bytes(), 0o644)
}
