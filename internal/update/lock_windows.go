//go:build windows

package update

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockFile opens path with no sharing: while this handle is open no other
// process can open the file, and Windows (and Wine) closes it if we die.
func lockFile(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return nil, ErrAlreadyRunning
		}
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}
