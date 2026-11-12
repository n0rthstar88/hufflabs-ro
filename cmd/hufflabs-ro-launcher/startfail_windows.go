//go:build windows

package main

import (
	"errors"
	"syscall"
)

// ERROR_VIRUS_INFECTED, ERROR_VIRUS_DELETED
func startBlockedByAntivirus(err error) bool {
	return errors.Is(err, syscall.Errno(225)) || errors.Is(err, syscall.Errno(226))
}

// ERROR_ACCESS_DISABLED_BY_POLICY, ERROR_SYSTEM_INTEGRITY_POLICY_VIOLATION (Smart App Control / WDAC)
func startBlockedByPolicy(err error) bool {
	return errors.Is(err, syscall.Errno(1260)) || errors.Is(err, syscall.Errno(4551))
}

// ERROR_CANCELLED: the player answered No on the UAC prompt
func startCancelled(err error) bool { return errors.Is(err, syscall.Errno(1223)) }

// ERROR_ELEVATION_REQUIRED
func startNeedsElevation(err error) bool { return errors.Is(err, syscall.Errno(740)) }

// ERROR_ACCESS_DENIED
func startAccessDenied(err error) bool { return errors.Is(err, syscall.Errno(5)) }
