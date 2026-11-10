package manifest

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var reserved = map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true}

func init() {
	for _, p := range []string{"COM", "LPT"} {
		for i := '1'; i <= '9'; i++ {
			reserved[p+string(i)] = true
		}
	}
}

// ValidPath reports whether p is safe to create under the install folder on both
// Windows and Linux.
func ValidPath(p string) error {
	switch {
	case p == "":
		return errors.New("empty path")
	case len(p) > 240:
		return errors.New("path too long")
	case !utf8.ValidString(p):
		return errors.New("not UTF-8")
	case strings.ContainsAny(p, "\\:*?\"<>|"):
		return errors.New("character not allowed on Windows")
	case strings.HasPrefix(p, "/"):
		return errors.New("absolute path")
	}
	for _, r := range p {
		if r < 0x20 {
			return errors.New("control character")
		}
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return errors.New("empty, . or .. segment")
		}
		if strings.HasSuffix(seg, ".") || strings.HasSuffix(seg, " ") {
			return errors.New("segment ends in a dot or space")
		}
		base := strings.ToUpper(seg)
		if i := strings.IndexByte(base, '.'); i >= 0 {
			base = base[:i]
		}
		if reserved[strings.TrimRight(base, " ")] {
			return errors.New("reserved Windows name")
		}
	}
	return nil
}

// Unmanaged reports whether p belongs to the player (settings, replays, screenshots,
// /savechat logs, other logs, the launcher's own state) and so is never shipped,
// replaced or deleted.
func Unmanaged(p string) bool {
	l := strings.ToLower(p)
	for _, dir := range []string{"savedata/", "memo/", "replay/", "name/", "screenshot/", "ai/user_ai/", "chat/", ".hufflabs-launcher/"} {
		if strings.HasPrefix(l, dir) {
			return true
		}
	}
	return l == "version" || strings.HasSuffix(l, ".log")
}
