package main

import (
	"bufio"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"hufflabs-ro/internal/manifest"
)

// readExcludes reads rsync-style patterns, one per line; # comments and blanks skipped.
func readExcludes(file string) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out, sc.Err()
}

// excluded applies the subset of rsync's rules the excludes file uses: a leading
// "/" anchors the pattern to the client root, a trailing "/" matches directories
// only, a pattern without "/" matches the last path element anywhere, and "*"
// matches within one path element.
func excluded(pats []string, rel string, isDir bool) bool {
	for _, p := range pats {
		dirOnly := strings.HasSuffix(p, "/")
		p = strings.TrimSuffix(p, "/")
		if dirOnly && !isDir {
			continue
		}
		target := rel
		if strings.HasPrefix(p, "/") {
			p = p[1:]
		} else if !strings.Contains(p, "/") {
			target = path.Base(rel)
		}
		if ok, _ := path.Match(p, target); ok {
			return true
		}
	}
	return false
}

// BuildTree maps each manifest path to the file that provides it: the client
// folder minus excludes and player files, with the built theme laid over it, the
// shipped defaults, a generated DATA.INI and the launcher. Same content as
// client/build-archive.sh stages, minus VERSION.
func BuildTree(clientDir, themeDir, defaultsDir, launcherExe string, pats []string, genDir, siteURL string) (map[string]string, error) {
	tree := map[string]string{}
	add := func(rel, src string) error {
		if err := manifest.ValidPath(rel); err != nil {
			return fmt.Errorf("%s: %v", rel, err)
		}
		if manifest.Unmanaged(rel) {
			return nil
		}
		tree[rel] = src
		return nil
	}
	walk := func(root, prefix string, skip func(rel string, d fs.DirEntry) bool) error {
		return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			r, _ := filepath.Rel(root, p)
			if r == "." {
				return nil
			}
			rel := path.Join(prefix, filepath.ToSlash(r))
			if skip(rel, d) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if manifest.Unmanaged(rel + "/") {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			return add(rel, p)
		})
	}
	if err := walk(clientDir, "", func(rel string, d fs.DirEntry) bool { return excluded(pats, rel, d.IsDir()) }); err != nil {
		return nil, err
	}
	if err := walk(filepath.Join(themeDir, "data"), "data", func(string, fs.DirEntry) bool { return false }); err != nil {
		return nil, err
	}
	for _, f := range []string{"HuffLabsRO.exe", "HuffLabsRO.ico", "HuffLabsRO.png"} {
		if _, err := os.Stat(filepath.Join(themeDir, f)); err == nil {
			if err := add(f, filepath.Join(themeDir, f)); err != nil {
				return nil, err
			}
		}
	}
	if err := walk(defaultsDir, "defaults", func(string, fs.DirEntry) bool { return false }); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		return nil, err
	}
	ini := filepath.Join(genDir, "DATA.INI")
	if err := os.WriteFile(ini, []byte("[Data]\r\n1=data.grf\r\n"), 0o644); err != nil {
		return nil, err
	}
	if err := add("DATA.INI", ini); err != nil {
		return nil, err
	}
	// The in-game Sign Up button: same rewrite as client/build-archive.sh. The
	// client files are EUC-KR; only ASCII bytes inside the tag change.
	for _, f := range []string{"data/clientinfo.xml", "data/sclientinfo.xml"} {
		src, ok := tree[f]
		if !ok {
			continue
		}
		b, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		out, n := signUpRe.ReplaceAll(b, []byte("<registrationweb>"+strings.TrimRight(siteURL, "/")+"/</registrationweb>")), len(signUpRe.FindAllIndex(b, -1))
		if n != 1 {
			return nil, fmt.Errorf("%s: want one <registrationweb> tag, found %d", f, n)
		}
		gen := filepath.Join(genDir, path.Base(f))
		if err := os.WriteFile(gen, out, 0o644); err != nil {
			return nil, err
		}
		tree[f] = gen
	}
	if _, err := os.Stat(launcherExe); err != nil {
		return nil, fmt.Errorf("launcher: %w (run launcher/build.sh)", err)
	}
	return tree, add("HuffLabsRO-Launcher.exe", launcherExe)
}

var signUpRe = regexp.MustCompile(`<registrationweb>[^<]*</registrationweb>`)

func sha256hex(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// hasher remembers file hashes by path, size and mtime, so a publish only reads
// files that changed.
type hasher struct {
	path  string
	cache map[string]cached
	dirty bool
}

type cached struct {
	Size, Mod int64
	SHA256    string
}

func (h *hasher) sum(p string) (string, error) {
	if h.cache == nil {
		h.cache = map[string]cached{}
		if b, err := os.ReadFile(h.path); err == nil {
			json.Unmarshal(b, &h.cache)
		}
	}
	fi, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	abs, _ := filepath.Abs(p)
	if c, ok := h.cache[abs]; ok && c.Size == fi.Size() && c.Mod == fi.ModTime().UnixNano() {
		return c.SHA256, nil
	}
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sh := sha256.New()
	if _, err := io.CopyBuffer(sh, f, make([]byte, 1<<20)); err != nil {
		return "", err
	}
	s := hex.EncodeToString(sh.Sum(nil))
	h.cache[abs] = cached{Size: fi.Size(), Mod: fi.ModTime().UnixNano(), SHA256: s}
	h.dirty = true
	return s, nil
}

func (h *hasher) save() error {
	if !h.dirty {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(h.cache)
	if err != nil {
		return err
	}
	tmp := h.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, h.path)
}

// checkLauncherBuild refuses a launcher exe that wasn't built for this site and
// this signing key (launcher/build.sh embeds both as plain strings). Shipping a
// test build would self-update every friend into a launcher that rejects all
// later updates.
func checkLauncherBuild(exe string, pub ed25519.PublicKey, site string) error {
	b, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	if !bytes.Contains(b, []byte(base64.StdEncoding.EncodeToString(pub))) {
		return fmt.Errorf("%s doesn't trust the key you're signing with; rebuild it with launcher/build.sh", exe)
	}
	if !bytes.Contains(b, []byte(strings.TrimRight(site, "/"))) {
		return fmt.Errorf("%s wasn't built for %s; rebuild it with launcher/build.sh", exe, site)
	}
	return nil
}
