//go:build windows

package main

import (
	"os"
	"testing"
)

// When started as the fake game by TestStartProcessRunsAndWaits, write
// "ran" into the working directory the launcher set, and exit.
func TestMain(m *testing.M) {
	if os.Getenv("HLRO_TEST_CHILD") != "" {
		if err := os.WriteFile("ran", nil, 0o644); err != nil { // relative: proves the working directory
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
