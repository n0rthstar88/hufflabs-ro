package update

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

var ErrAlreadyRunning = errors.New("the launcher is already running")

// Lock makes sure only one launcher works on dir at a time. The lock is an OS
// lock held on an open file for as long as the launcher runs, so the system
// drops it if the launcher dies; the PID written in the file is only for people
// reading it.
func Lock(dir string) (release func(), err error) {
	if err := os.MkdirAll(filepath.Join(dir, StateDir), 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, StateDir, "lock")
	f, err := lockFile(path)
	if err != nil {
		return nil, err
	}
	f.Truncate(0)
	f.WriteString(strconv.Itoa(os.Getpid()))
	return func() { f.Close() }, nil
}
