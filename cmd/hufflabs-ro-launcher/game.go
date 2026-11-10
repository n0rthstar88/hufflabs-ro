package main

import (
	"os/exec"
	"path/filepath"
)

const gameExe = "HuffLabsRO.exe"

// startGame starts the client from the install folder and returns its process
// handle; the launcher waits for it (holding the lock, so no update runs while
// the game is open).
func startGame(dir string) (*exec.Cmd, error) {
	cmd := exec.Command(filepath.Join(dir, gameExe))
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
