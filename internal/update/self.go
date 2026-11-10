package update

import (
	"os"
	"path/filepath"
	"strings"
)

// selfRel is exePath relative to dir with forward slashes, or "" if outside it.
func selfRel(exePath, dir string) string {
	rel, err := filepath.Rel(dir, exePath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(rel)
}

// SelfUpdatePending reports whether p replaces the running launcher. If so, the
// caller sets p.Self so Apply swaps it with SwapSelf, then restarts the launcher.
func SelfUpdatePending(exePath, dir string, p Plan) bool {
	rel := selfRel(exePath, dir)
	if rel == "" {
		return false
	}
	for _, e := range p.Download {
		if strings.EqualFold(e.Path, rel) {
			return true
		}
	}
	return false
}

// SwapSelf puts newPath in place of the running exe. Windows can't overwrite a
// running exe but can rename it, so the old one moves aside to <exe>.old first;
// if the second step fails the old exe is put back.
func SwapSelf(exePath, newPath string) error {
	old := exePath + ".old"
	os.Remove(old)
	if err := rename(exePath, old); err != nil {
		return err
	}
	if err := rename(newPath, exePath); err != nil {
		if rerr := rename(old, exePath); rerr != nil {
			return rerr
		}
		return err
	}
	return nil
}

// CleanupOld removes the exe left behind by the last self-update.
func CleanupOld(exePath string) { os.Remove(exePath + ".old") }
