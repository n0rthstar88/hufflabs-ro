//go:build !windows

package update

import (
	"errors"
	"syscall"
)

func isDiskFull(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT)
}

func isInUse(err error) bool {
	return errors.Is(err, syscall.ETXTBSY) || errors.Is(err, syscall.EBUSY)
}
