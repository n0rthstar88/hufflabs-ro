//go:build windows

package update

import "syscall"

var inUseErrno, diskFullErrno = syscall.Errno(32), syscall.Errno(112) // sharing violation, disk full
