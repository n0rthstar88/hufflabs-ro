//go:build !windows

package main

import (
	"errors"
	"syscall"
)

func startBlockedByAntivirus(error) bool { return false }
func startBlockedByPolicy(error) bool    { return false }
func startAccessDenied(err error) bool   { return errors.Is(err, syscall.EACCES) }
