//go:build windows

package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procShellExecuteExW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

// shellExecuteInfo is SHELLEXECUTEINFOW; Go's field alignment matches the C
// layout on both 386 and amd64.
type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           uintptr
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       uintptr
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      uintptr
	dwHotKey       uint32
	hIconOrMonitor uintptr
	hProcess       windows.Handle
}

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
	seeMaskFlagNoUI       = 0x00000400 // no error boxes; the launcher shows its own (UAC still appears)
	swShowNormal          = 1
)

type winProcess struct{ h windows.Handle }

func (p winProcess) Wait() error {
	defer windows.CloseHandle(p.h)
	_, err := windows.WaitForSingleObject(p.h, windows.INFINITE)
	return err
}

// startProcess runs path the way Explorer would, so an exe that asks for admin
// rights gets the UAC prompt instead of failing with ERROR_ELEVATION_REQUIRED.
// Answering No comes back as ERROR_CANCELLED.
func startProcess(path, dir string) (gameProcess, error) {
	verb, _ := windows.UTF16PtrFromString("open")
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	wd, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return nil, err
	}
	info := shellExecuteInfo{
		fMask:       seeMaskNoCloseProcess | seeMaskNoAsync | seeMaskFlagNoUI,
		lpVerb:      verb,
		lpFile:      file,
		lpDirectory: wd,
		nShow:       swShowNormal,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	ok, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return nil, &os.SyscallError{Syscall: "ShellExecuteEx", Err: callErr}
	}
	if info.hProcess == 0 { // handed to an already running process; nothing to wait for
		return noWait{}, nil
	}
	return winProcess{h: info.hProcess}, nil
}

type noWait struct{}

func (noWait) Wait() error { return nil }
