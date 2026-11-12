//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The real start path: ShellExecuteEx a copy of this test binary as the
// "game" (it exits at once, see TestMain) and wait for it.
func TestStartProcessRunsAndWaits(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	game := filepath.Join(dir, gameExe)
	b, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(game, b, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "ran")
	t.Setenv("HLRO_TEST_CHILD", "1")
	p, err := startGame(dir)
	if err != nil {
		t.Fatalf("startGame: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- p.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Wait didn't return")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("child didn't run in the game folder: %v", err)
	}
}
