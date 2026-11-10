//go:build !windows

package update

import "syscall"

var inUseErrno, diskFullErrno = syscall.ETXTBSY, syscall.ENOSPC
