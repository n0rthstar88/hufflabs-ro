package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"hufflabs-ro/internal/manifest"
)

var ErrRollback = errors.New("server manifest is older than the installed one")

// Plan is what an update has to do.
type Plan struct {
	Download []manifest.Entry
	Delete   []string
	Bytes    int64
	// Self is the running launcher's path in the manifest when the plan replaces
	// it; Apply then swaps it with SwapSelf instead of a plain rename.
	Self string
}

var hashFile = func(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.CopyBuffer(h, f, make([]byte, 64<<10)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// MakePlan compares dir with m. Files whose size and mtime match st are trusted;
// others are hashed (progress reports that pass). It returns the plan and st updated
// with what it learned.
func MakePlan(dir string, st State, m manifest.Manifest, progress func(done, total int)) (Plan, State, error) {
	st = st.clone()
	var p Plan
	want := make(map[string]bool, len(m.Files))
	for i, e := range m.Files {
		want[strings.ToLower(e.Path)] = true // Windows paths are case-insensitive
		fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(e.Path)))
		switch {
		case errors.Is(err, os.ErrNotExist):
			delete(st.Files, e.Path)
		case err != nil:
			return Plan{}, st, err
		case !fi.Mode().IsRegular():
			return Plan{}, st, fmt.Errorf("%s is not a regular file", e.Path)
		default:
			known, ok := st.Files[e.Path]
			if !ok || known.Size != fi.Size() || known.ModUnix != fi.ModTime().UnixNano() {
				sum, err := hashFile(filepath.Join(dir, filepath.FromSlash(e.Path)))
				if err != nil {
					return Plan{}, st, err
				}
				known = FileState{Size: fi.Size(), ModUnix: fi.ModTime().UnixNano(), SHA256: sum}
				st.Files[e.Path] = known
			}
			if known.SHA256 == e.SHA256 && known.Size == e.Size {
				if progress != nil {
					progress(i+1, len(m.Files))
				}
				continue
			}
		}
		p.Download = append(p.Download, e)
		p.Bytes += e.Size
		if progress != nil {
			progress(i+1, len(m.Files))
		}
	}
	for path := range st.Files {
		if !want[strings.ToLower(path)] && !manifest.Unmanaged(path) {
			p.Delete = append(p.Delete, path)
		}
	}
	sort.Strings(p.Delete)
	return p, st, nil
}

// CheckSerial refuses a manifest older than what's installed.
func CheckSerial(st State, m manifest.Manifest) error {
	if m.Serial < st.Serial {
		return fmt.Errorf("%w (server %d, installed %d)", ErrRollback, m.Serial, st.Serial)
	}
	return nil
}
