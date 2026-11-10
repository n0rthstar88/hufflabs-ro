package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestExcludesMatchBuildArchive(t *testing.T) {
	pats, err := readExcludes("../../client/excludes.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct {
		path string
		dir  bool
	}{{"win32", true}, {"Patches", true}, {".warpgate.json", false}, {"2025-07-16_Ragexe_1.exe", false},
		{"data/a.orig", false}, {"System/b.bak", false}, {"savedata", true}, {"hufflabs.grf", false}, {"README.md", false}, {"x.yml", false}} {
		if !excluded(pats, p.path, p.dir) {
			t.Errorf("%s not excluded", p.path)
		}
	}
	for _, p := range []struct {
		path string
		dir  bool
	}{{"data.grf", false}, {"System/x.lub", false}, {"data/texture/x.bmp", false}, {"data/README.md", false}, {"System", true}, {"data/win32", true}} {
		if excluded(pats, p.path, p.dir) {
			t.Errorf("%s excluded", p.path)
		}
	}
}

func mk(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const themeDir = "data/texture/À¯ÀúÀÎÅÍÆäÀÌ½º/login_interface"

func TestBuildTreeOverlaysThemeAndSkipsUnmanaged(t *testing.T) {
	client, theme, defaults := t.TempDir(), t.TempDir(), t.TempDir()
	launcher := filepath.Join(t.TempDir(), "L.exe")
	os.WriteFile(launcher, []byte("launcher"), 0o755)
	mk(t, client, "data.grf", "grf")
	mk(t, client, themeDir+"/win_login.bmp", "client version")
	mk(t, client, "Replay/r.rrf", "r")
	mk(t, client, "debug.log", "l")
	mk(t, client, "VERSION", "1")
	mk(t, client, "win32/WARP.exe", "w")
	mk(t, client, "DATA.INI", "[Data]\r\n1=hufflabs.grf\r\n2=data.grf\r\n")
	mk(t, theme, themeDir+"/win_login.bmp", "theme version")
	mk(t, theme, "HuffLabsRO.exe", "patched exe")
	mk(t, theme, "HuffLabsRO.ico", "ico")
	mk(t, theme, "site/bg.jpg", "site only")
	mk(t, defaults, "OptionInfo.lua", "opts")
	mk(t, client, "data/clientinfo.xml", "<x>\xb1\xb9</x><registrationweb>www.ragnarok.com</registrationweb>")
	mk(t, client, "data/sclientinfo.xml", "<registrationweb>www.ragnarok.com</registrationweb>")
	pats, _ := readExcludes("../../client/excludes.txt")
	tree, err := BuildTree(client, theme, defaults, launcher, pats, t.TempDir(), "https://ro.example")
	if err != nil {
		t.Fatal(err)
	}
	read := func(k string) string { b, _ := os.ReadFile(tree[k]); return string(b) }
	if read(themeDir+"/win_login.bmp") != "theme version" {
		t.Error("theme didn't win over the client file")
	}
	for _, k := range []string{"Replay/r.rrf", "debug.log", "VERSION", "win32/WARP.exe", "site/bg.jpg"} {
		if _, ok := tree[k]; ok {
			t.Errorf("%s shipped", k)
		}
	}
	if read("DATA.INI") != "[Data]\r\n1=data.grf\r\n" || read("HuffLabsRO-Launcher.exe") != "launcher" ||
		read("defaults/OptionInfo.lua") != "opts" || read("HuffLabsRO.exe") != "patched exe" || read("data.grf") != "grf" {
		t.Errorf("tree %v", tree)
	}
	found := false
	for k := range tree {
		if k == themeDir+"/win_login.bmp" {
			found = true
		}
	}
	if !found {
		t.Error("mojibake theme path not byte-identical")
	}
}

// The in-game Sign Up button must point at our site in what ships (as the zip
// build does), or the launcher reverts it on every install.
func TestBuildTreeRewritesSignUpURL(t *testing.T) {
	client, theme, defaults := t.TempDir(), t.TempDir(), t.TempDir()
	launcher := filepath.Join(t.TempDir(), "L.exe")
	os.WriteFile(launcher, []byte("l"), 0o755)
	mk(t, client, "data/clientinfo.xml", "<x>\xb1\xb9</x><registrationweb>www.ragnarok.com</registrationweb><y/>")
	mk(t, client, "data/sclientinfo.xml", "<registrationweb></registrationweb>")
	os.MkdirAll(filepath.Join(theme, "data"), 0o755)
	tree, err := BuildTree(client, theme, defaults, launcher, nil, t.TempDir(), "https://ro.example")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(tree["data/clientinfo.xml"])
	if string(b) != "<x>\xb1\xb9</x><registrationweb>https://ro.example/</registrationweb><y/>" {
		t.Errorf("clientinfo = %q", b)
	}
	b, _ = os.ReadFile(tree["data/sclientinfo.xml"])
	if string(b) != "<registrationweb>https://ro.example/</registrationweb>" {
		t.Errorf("sclientinfo = %q", b)
	}
	if orig, _ := os.ReadFile(filepath.Join(client, "data/clientinfo.xml")); string(orig) != "<x>\xb1\xb9</x><registrationweb>www.ragnarok.com</registrationweb><y/>" {
		t.Error("the client folder's own file was changed")
	}
	mk(t, client, "data/clientinfo.xml", "<nothing/>")
	if _, err := BuildTree(client, theme, defaults, launcher, nil, t.TempDir(), "https://ro.example"); err == nil {
		t.Error("clientinfo without <registrationweb> accepted")
	}
}

// Publishing a launcher built with other keys or another site would make every
// friend's launcher update itself into one that rejects all later updates.
func TestCheckLauncherBuild(t *testing.T) {
	pub, _ := keyPair(t)
	other, _ := keyPair(t)
	b64 := base64.StdEncoding.EncodeToString
	exe := func(content string) string {
		p := filepath.Join(t.TempDir(), "L.exe")
		os.WriteFile(p, []byte(content), 0o755)
		return p
	}
	good := exe("MZ...https://ro.example..." + b64(other) + "," + b64(pub) + "...")
	if err := checkLauncherBuild(good, pub, "https://ro.example"); err != nil {
		t.Errorf("good build refused: %v", err)
	}
	if err := checkLauncherBuild(exe("MZ https://ro.example "+b64(other)), pub, "https://ro.example"); err == nil {
		t.Error("build without the signing key accepted")
	}
	if err := checkLauncherBuild(exe("MZ https://test.invalid "+b64(pub)), pub, "https://ro.example"); err == nil {
		t.Error("build for another site accepted")
	}
}
