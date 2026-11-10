//go:build windows

package update

import (
	"errors"
	"syscall"
)

func isDiskFull(err error) bool {
	return errors.Is(err, syscall.Errno(112)) || errors.Is(err, syscall.Errno(39)) // ERROR_DISK_FULL, ERROR_HANDLE_DISK_FULL
}

func isInUse(err error) bool {
	// ERROR_SHARING_VIOLATION, ERROR_LOCK_VIOLATION, ERROR_ACCESS_DENIED (replacing a running exe)
	return errors.Is(err, syscall.Errno(32)) || errors.Is(err, syscall.Errno(33)) || errors.Is(err, syscall.Errno(5))
}
