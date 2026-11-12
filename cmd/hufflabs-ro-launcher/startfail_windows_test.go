//go:build windows

package main

import (
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestStartDetailWindowsCodes(t *testing.T) {
	for code, want := range map[syscall.Errno]string{
		225:  "blocked " + gameExe + ". Allow it under Protection history",
		226:  "blocked " + gameExe + ". Allow it under Protection history",
		1260: "Smart App Control",
		4551: "Smart App Control",
		5:    "wouldn't let " + gameExe + " run",
		1223: "answered No. Press Retry and choose Yes",
		740:  "needs admin rights to run. Press Retry",
	} {
		err := &os.SyscallError{Syscall: "CreateProcess", Err: code}
		if got := startDetail(err); !strings.Contains(got, want) {
			t.Errorf("code %d: detail %q, want %q", code, got, want)
		}
	}
}
