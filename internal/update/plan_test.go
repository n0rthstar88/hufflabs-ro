package update

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"hufflabs-ro/internal/manifest"
)

func paths(es []manifest.Entry) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.Path)
	}
	sort.Strings(out)
	return out
}

func TestPlanFreshInstallHashesEverything(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "same.txt", []byte("same"))
	writeFile(t, dir, "data/same2.txt", []byte("same2"))
	writeFile(t, dir, "changed.txt", []byte("old"))
	m := manifest.Manifest{Serial: 5, Files: []manifest.Entry{
		entry("same.txt", []byte("same")), entry("data/same2.txt", []byte("same2")),
		entry("changed.txt", []byte("new!")), entry("missing.txt", []byte("m")),
	}}
	var last, total int
	p, st, err := MakePlan(dir, State{}, m, func(d, tt int) { last, total = d, tt })
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(p.Download); !reflect.DeepEqual(got, []string{"changed.txt", "missing.txt"}) {
		t.Errorf("Download = %v", got)
	}
	if p.Bytes != 5 {
		t.Errorf("Bytes = %d, want 5", p.Bytes)
	}
	if st.Files["same.txt"].SHA256 != shaHex([]byte("same")) || st.Files["changed.txt"].SHA256 != shaHex([]byte("old")) {
		t.Errorf("state not learned: %+v", st.Files)
	}
	if last != total || total != 4 {
		t.Errorf("progress %d/%d, want 4/4", last, total)
	}
}

func TestPlanTrustsCachedState(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", []byte("aaaa"))
	m := manifest.Manifest{Files: []manifest.Entry{entry("a.txt", []byte("aaaa"))}}
	_, st, err := MakePlan(dir, State{}, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(dir, "a.txt")
	fi, _ := os.Stat(full)
	os.WriteFile(full, []byte("bbbb"), 0o644) // same size
	os.Chtimes(full, time.Now(), fi.ModTime())
	n := 0
	old := hashFile
	hashFile = func(p string) (string, error) { n++; return old(p) }
	defer func() { hashFile = old }()
	p, _, err := MakePlan(dir, st, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || len(p.Download) != 0 {
		t.Errorf("hashed %d files, download %v; want cached, none", n, p.Download)
	}
	os.Chtimes(full, time.Now(), fi.ModTime().Add(time.Second)) // mtime moved: must rehash
	p, _, _ = MakePlan(dir, st, m, nil)
	if n != 1 || len(p.Download) != 1 {
		t.Errorf("after mtime change hashed %d, download %v", n, p.Download)
	}
}

func TestPlanDeletesDroppedFilesButNotUnmanaged(t *testing.T) {
	dir := t.TempDir()
	st := State{Serial: 1, Files: map[string]FileState{"old.txt": {Size: 1}, "savedata/x": {Size: 1}}}
	p, _, err := MakePlan(dir, st, manifest.Manifest{Serial: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Delete, []string{"old.txt"}) {
		t.Errorf("Delete = %v", p.Delete)
	}
}

func TestStateSaveLoad(t *testing.T) {
	dir := t.TempDir()
	st, err := LoadState(dir)
	if err != nil || st.Serial != 0 {
		t.Fatalf("empty LoadState = %+v, %v", st, err)
	}
	st = State{Serial: 9, Files: map[string]FileState{"a": {Size: 1, ModUnix: 2, SHA256: "x"}}}
	if err := st.Save(dir); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(dir)
	if err != nil || !reflect.DeepEqual(got, st) {
		t.Errorf("LoadState = %+v, %v", got, err)
	}
}

func TestCheckSerialRollback(t *testing.T) {
	if err := CheckSerial(State{Serial: 200}, manifest.Manifest{Serial: 199}); !errors.Is(err, ErrRollback) {
		t.Errorf("err = %v, want ErrRollback", err)
	}
	if err := CheckSerial(State{Serial: 200}, manifest.Manifest{Serial: 200}); err != nil {
		t.Errorf("equal serial err = %v", err)
	}
}

// A path whose letter case changed between updates is the same file on Windows:
// deleting the old spelling would delete the new file.
func TestPlanCaseOnlyRenameNotDeleted(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "System/x.lub", []byte("x"))
	st := State{Serial: 1, Files: map[string]FileState{"system/x.lub": {Size: 1}}}
	m := manifest.Manifest{Serial: 2, Files: []manifest.Entry{entry("System/x.lub", []byte("x"))}}
	p, _, err := MakePlan(dir, st, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Delete) != 0 {
		t.Errorf("Delete = %v, want none", p.Delete)
	}
}
