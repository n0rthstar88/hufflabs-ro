package update

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"hufflabs-ro/internal/manifest"
)

func TestSelfUpdatePending(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "HuffLabsRO-Launcher.exe")
	p := Plan{Download: []manifest.Entry{{Path: "data/a.bmp"}, {Path: "HuffLabsRO-Launcher.exe"}}}
	if !SelfUpdatePending(exe, dir, p) {
		t.Error("own exe in plan not detected")
	}
	if SelfUpdatePending(exe, dir, Plan{Download: []manifest.Entry{{Path: "data/a.bmp"}}}) {
		t.Error("false positive")
	}
	if SelfUpdatePending(filepath.Join(t.TempDir(), "HuffLabsRO-Launcher.exe"), dir, p) {
		t.Error("exe outside the install folder matched")
	}
}

func TestSwapSelfReplaces(t *testing.T) {
	dir := t.TempDir()
	exe, nw := filepath.Join(dir, "L.exe"), filepath.Join(dir, "new.part")
	os.WriteFile(exe, []byte("A"), 0o755)
	os.WriteFile(nw, []byte("B"), 0o755)
	if err := SwapSelf(exe, nw); err != nil {
		t.Fatal(err)
	}
	if readFile(t, dir, "L.exe") != "B" || readFile(t, dir, "L.exe.old") != "A" {
		t.Errorf("exe %q old %q", readFile(t, dir, "L.exe"), readFile(t, dir, "L.exe.old"))
	}
}

func TestSwapSelfRestoresOnFailure(t *testing.T) {
	dir := t.TempDir()
	exe, nw := filepath.Join(dir, "L.exe"), filepath.Join(dir, "new.part")
	os.WriteFile(exe, []byte("A"), 0o755)
	os.WriteFile(nw, []byte("B"), 0o755)
	calls := 0
	old := rename
	rename = func(a, b string) error {
		calls++
		if calls == 2 {
			return errors.New("boom")
		}
		return os.Rename(a, b)
	}
	defer func() { rename = old }()
	if err := SwapSelf(exe, nw); err == nil {
		t.Fatal("no error")
	}
	if readFile(t, dir, "L.exe") != "A" || readFile(t, dir, "L.exe.old") != "<missing>" {
		t.Errorf("exe %q old %q", readFile(t, dir, "L.exe"), readFile(t, dir, "L.exe.old"))
	}
}

func TestCleanupOld(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "L.exe")
	os.WriteFile(exe+".old", []byte("A"), 0o755)
	CleanupOld(exe)
	if readFile(t, dir, "L.exe.old") != "<missing>" {
		t.Error(".old not removed")
	}
}

func TestApplySwapsSelf(t *testing.T) {
	dir, srv, m := setup(t, map[string]string{"HuffLabsRO-Launcher.exe": "new launcher"})
	writeFile(t, dir, "HuffLabsRO-Launcher.exe", []byte("old launcher"))
	p, st := plan(t, dir, m)
	exe := filepath.Join(dir, "HuffLabsRO-Launcher.exe")
	if !SelfUpdatePending(exe, dir, p) {
		t.Fatal("not pending")
	}
	p.Self = "HuffLabsRO-Launcher.exe"
	if _, err := Apply(t.Context(), srv.Client(), srv.URL, dir, p, st, m, nil); err != nil {
		t.Fatal(err)
	}
	if readFile(t, dir, "HuffLabsRO-Launcher.exe") != "new launcher" || readFile(t, dir, "HuffLabsRO-Launcher.exe.old") != "old launcher" {
		t.Errorf("exe %q old %q", readFile(t, dir, "HuffLabsRO-Launcher.exe"), readFile(t, dir, "HuffLabsRO-Launcher.exe.old"))
	}
}
