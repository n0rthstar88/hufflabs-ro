package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartDetailMissingExe(t *testing.T) {
	_, err := startGame(t.TempDir())
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err %v, want not-exist", err)
	}
	if got := startDetail(err); !strings.Contains(got, "is missing") || !strings.Contains(got, "download it again") {
		t.Errorf("detail %q", got)
	}
}

func TestStartDetailOtherError(t *testing.T) {
	err := &os.PathError{Op: "fork/exec", Path: filepath.Join("x", gameExe), Err: errors.New("exec format error")}
	if got := startDetail(err); got != "Windows said: exec format error. Press Retry to check the files and try again." {
		t.Errorf("detail %q", got)
	}
	if got := startDetail(fmt.Errorf("plain")); !strings.HasPrefix(got, "Windows said: ") {
		t.Errorf("detail %q", got)
	}
}
