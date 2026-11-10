package update

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestLockSecondInstance(t *testing.T) {
	dir := t.TempDir()
	release, err := Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(dir); !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("second Lock err = %v, want ErrAlreadyRunning", err)
	}
	release()
	r2, err := Lock(dir)
	if err != nil {
		t.Fatalf("Lock after release: %v", err)
	}
	r2()
}

func TestLockTakesOverStale(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, StateDir), 0o755)
	os.WriteFile(filepath.Join(dir, StateDir, "lock"), []byte("999999999"), 0o644) // no such process
	release, err := Lock(dir)
	if err != nil {
		t.Fatalf("stale lock not taken over: %v", err)
	}
	release()
}

// Wine hands out the same small PIDs launch after launch, so a lock left by a
// launcher that died can carry our own PID. It's stale: we didn't write it.
func TestLockWithOwnPIDIsStale(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, StateDir), 0o755)
	os.WriteFile(filepath.Join(dir, StateDir, "lock"), []byte(strconv.Itoa(os.Getpid())), 0o644)
	release, err := Lock(dir)
	if err != nil {
		t.Fatalf("own-PID lock not taken over: %v", err)
	}
	release()
}

// A lock file naming a process that is alive but isn't a launcher (Wine reuses
// PIDs) must not block: only a launcher actually holding the lock counts.
func TestLockFileOfOtherLiveProcessIsStale(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, StateDir), 0o755)
	os.WriteFile(filepath.Join(dir, StateDir, "lock"), []byte(strconv.Itoa(os.Getppid())), 0o644)
	release, err := Lock(dir)
	if err != nil {
		t.Fatalf("lock naming a live non-launcher process blocked: %v", err)
	}
	release()
}
