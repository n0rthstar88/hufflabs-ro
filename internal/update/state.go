// Package update keeps an install folder in sync with a signed manifest: it works out
// which files differ, downloads them with resume, swaps them in atomically, and
// remembers what it installed in .hufflabs-launcher/state.json.
package update

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// StateDir is the launcher's own folder inside the install folder.
const StateDir = ".hufflabs-launcher"

// FileState is what the launcher knows about one installed file. If the file's size
// and mtime still match, its hash is trusted without reading it again.
type FileState struct {
	Size    int64
	ModUnix int64 // mtime in Unix nanoseconds
	SHA256  string
}

type State struct {
	Serial int64
	Files  map[string]FileState
}

func statePath(dir string) string { return filepath.Join(dir, StateDir, "state.json") }

// LoadState reads the saved state; a missing file is an empty state.
func LoadState(dir string) (State, error) {
	b, err := os.ReadFile(statePath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return State{Files: map[string]FileState{}}, nil
	} else if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		// a damaged state only costs a full re-check
		return State{Files: map[string]FileState{}}, nil
	}
	if s.Files == nil {
		s.Files = map[string]FileState{}
	}
	return s, nil
}

// Save writes the state atomically.
func (s State) Save(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, StateDir), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := statePath(dir) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, statePath(dir))
}

func (s State) clone() State {
	c := State{Serial: s.Serial, Files: make(map[string]FileState, len(s.Files))}
	for k, v := range s.Files {
		c.Files[k] = v
	}
	return c
}
