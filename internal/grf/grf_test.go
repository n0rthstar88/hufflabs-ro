package grf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/korean"
)

type testFile struct {
	name  string // UTF-8; written as CP949
	data  []byte
	flags uint8
}

func z(b []byte) []byte {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write(b)
	w.Close()
	return buf.Bytes()
}

// writeGRF builds a minimal GRF of the given version (0x200 or 0x300).
func writeGRF(t *testing.T, version uint32, files []testFile) string {
	t.Helper()
	var body, table bytes.Buffer
	for _, f := range files {
		name, err := korean.EUCKR.NewEncoder().String(f.name)
		if err != nil {
			t.Fatal(err)
		}
		comp := z(f.data)
		off := body.Len()
		body.Write(comp)
		table.WriteString(name)
		table.WriteByte(0)
		binary.Write(&table, binary.LittleEndian, uint32(len(comp)))
		binary.Write(&table, binary.LittleEndian, uint32(len(comp)))
		binary.Write(&table, binary.LittleEndian, uint32(len(f.data)))
		table.WriteByte(f.flags)
		if version == 0x300 {
			binary.Write(&table, binary.LittleEndian, uint64(off))
		} else {
			binary.Write(&table, binary.LittleEndian, uint32(off))
		}
	}
	ct := z(table.Bytes())
	var out bytes.Buffer
	if version == 0x300 {
		out.WriteString("Event Horizon\x00c\x00")
	} else {
		out.WriteString("Master of Magic\x00")
	}
	out.Write([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14})
	if version == 0x300 {
		binary.Write(&out, binary.LittleEndian, uint64(body.Len()))
		binary.Write(&out, binary.LittleEndian, uint32(len(files)))
	} else {
		binary.Write(&out, binary.LittleEndian, uint32(body.Len()))
		binary.Write(&out, binary.LittleEndian, uint32(0))            // seed
		binary.Write(&out, binary.LittleEndian, uint32(len(files)+7)) // count + seed + 7
	}
	binary.Write(&out, binary.LittleEndian, version)
	out.Write(body.Bytes())
	if version == 0x300 {
		binary.Write(&out, binary.LittleEndian, uint32(0))
	}
	binary.Write(&out, binary.LittleEndian, uint32(len(ct)))
	binary.Write(&out, binary.LittleEndian, uint32(table.Len()))
	out.Write(ct)
	p := filepath.Join(t.TempDir(), "test.grf")
	if err := os.WriteFile(p, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

var sample = []testFile{
	{name: `data\clientinfo.xml`, data: []byte("<clientinfo/>"), flags: 1},
	{name: `data\texture\유저인터페이스\loading00.jpg`, data: bytes.Repeat([]byte{0xFF, 0xD8}, 500), flags: 1},
	{name: `data\texture`, data: nil, flags: 0}, // directory entry
}

func TestRoundTrip(t *testing.T) {
	for _, v := range []uint32{0x200, 0x300} {
		a, err := Open(writeGRF(t, v, sample))
		if err != nil {
			t.Fatalf("v%x: %v", v, err)
		}
		files := a.Files()
		if len(files) != 2 {
			t.Fatalf("v%x: Files() = %d entries, want 2 (directory skipped)", v, len(files))
		}
		if files[1].Name != `data\texture\유저인터페이스\loading00.jpg` || files[1].Size != 1000 {
			t.Errorf("v%x: entry = %+v", v, files[1])
		}
		for _, f := range sample[:2] {
			got, err := a.Read(f.name)
			if err != nil || !bytes.Equal(got, f.data) {
				t.Errorf("v%x: Read(%s) = %d bytes, %v", v, f.name, len(got), err)
			}
		}
		// Lookups ignore case and slash direction, like the client does.
		if _, err := a.Read(`DATA/CLIENTINFO.XML`); err != nil {
			t.Errorf("v%x: case/slash-insensitive lookup: %v", v, err)
		}
		a.Close()
	}
}

func TestEncryptedRejected(t *testing.T) {
	a, err := Open(writeGRF(t, 0x300, []testFile{{name: `data\secret.txt`, data: []byte("x"), flags: 1 | 0x02}}))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err := a.Read(`data\secret.txt`); err == nil {
		t.Error("encrypted entry read without error")
	}
}

func TestNotFound(t *testing.T) {
	a, _ := Open(writeGRF(t, 0x300, sample))
	defer a.Close()
	if _, err := a.Read(`data\nope.txt`); err == nil {
		t.Error("missing file read without error")
	}
}

func TestRejectsNonGRF(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.grf")
	os.WriteFile(p, bytes.Repeat([]byte("x"), 100), 0o644)
	if _, err := Open(p); err == nil {
		t.Error("garbage accepted as GRF")
	}
}

func TestWriteRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "hufflabs.grf")
	in := []File{
		{Name: `data\texture\유저인터페이스\login_interface\bg_login.tga`, Data: bytes.Repeat([]byte{1, 2, 3, 4}, 300)},
		{Name: `data\texture\유저인터페이스\t_login.jpg`, Data: []byte("jpeg bytes")},
	}
	if err := Write(p, in); err != nil {
		t.Fatal(err)
	}
	a, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if len(a.Files()) != 2 {
		t.Fatalf("files = %d", len(a.Files()))
	}
	for _, f := range in {
		got, err := a.Read(f.Name)
		if err != nil || !bytes.Equal(got, f.Data) {
			t.Errorf("Read(%s): %d bytes, %v", f.Name, len(got), err)
		}
	}
	raw, _ := os.ReadFile(p)
	if string(raw[:15]) != "Master of Magic" || binary.LittleEndian.Uint32(raw[42:]) != 0x200 {
		t.Error("not a 0x200 GRF header")
	}
}
