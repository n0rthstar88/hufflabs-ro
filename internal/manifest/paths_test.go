package manifest

import (
	"strings"
	"testing"
)

func TestValidPath(t *testing.T) {
	for _, p := range []string{themePath, "HuffLabsRO.exe", "System/itemInfo.lub", "R&D #1/it's.txt"} {
		if err := ValidPath(p); err != nil {
			t.Errorf("ValidPath(%q) = %v, want ok", p, err)
		}
	}
	for _, p := range []string{"../x", "a/../b", "./a", "a//b", "/etc/passwd", "C:/x", `a\b`, "con.txt", "data/AUX", "x/LPT1.log",
		"COM9", "dir./f", "f ", "", strings.Repeat("a", 241), "\xff", "a/\x00b"} {
		if err := ValidPath(p); err == nil {
			t.Errorf("ValidPath(%q) accepted", p)
		}
	}
}

func TestUnmanaged(t *testing.T) {
	for _, p := range []string{"savedata/OptionInfo.lua", "SaveData/x", "Replay/a.rrf", "memo/x", "Name/x", "ScreenShot/s.jpg",
		"AI/USER_AI/AI.lua", "Chat/Chat_General.txt", "chat/x.txt", "debug.log", "x/errors.LOG", ".hufflabs-launcher/state.json", "VERSION"} {
		if !Unmanaged(p) {
			t.Errorf("Unmanaged(%q) = false", p)
		}
	}
	for _, p := range []string{"AI/AI.lua", "data.grf", "System/LuaFiles514/x.lub", "data/VERSION", "savedatax/y"} {
		if Unmanaged(p) {
			t.Errorf("Unmanaged(%q) = true", p)
		}
	}
}
