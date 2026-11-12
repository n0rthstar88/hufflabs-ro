package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

const gameExe = "HuffLabsRO.exe"

// startGame starts the client from the install folder and returns its process
// handle; the launcher waits for it (holding the lock, so no update runs while
// the game is open).
func startGame(dir string) (*exec.Cmd, error) {
	path := filepath.Join(dir, gameExe)
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	cmd := exec.Command(path)
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// startDetail turns a failed start into the line the player sees. Antivirus
// often flags the patched client, so the usual causes get plain words.
func startDetail(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return gameExe + " is missing. Antivirus may have removed it. Press Retry to download it again."
	case startBlockedByAntivirus(err):
		return "Windows Security or your antivirus blocked " + gameExe + ". Allow it under Protection history, then press Retry."
	case startBlockedByPolicy(err):
		return "Windows blocked " + gameExe + " (Smart App Control or a security policy). Allow it, then press Retry."
	case startAccessDenied(err):
		return "Windows wouldn't let " + gameExe + " run. Antivirus may be blocking it. Allow it, then press Retry."
	}
	if inner := errors.Unwrap(err); inner != nil {
		err = inner // drop the "fork/exec C:\\...\\HuffLabsRO.exe:" prefix
	}
	return fmt.Sprintf("Windows said: %v. Press Retry to check the files and try again.", err)
}
