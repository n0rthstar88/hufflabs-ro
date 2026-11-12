package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const gameExe = "HuffLabsRO.exe"

// gameProcess is the running client; the launcher waits for it (holding the
// lock, so no update runs while the game is open).
type gameProcess interface{ Wait() error }

// startGame starts the client from the install folder. The client's manifest
// says requireAdministrator, so on Windows it goes through ShellExecuteEx,
// which shows the UAC prompt (CreateProcess just fails with "requires
// elevation"); see game_windows.go.
func startGame(dir string) (gameProcess, error) {
	path := filepath.Join(dir, gameExe)
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return startProcess(path, dir)
}

// startDetail turns a failed start into the line the player sees. Antivirus
// often flags the patched client, so the usual causes get plain words.
func startDetail(err error) string {
	switch {
	case startCancelled(err):
		return "HuffLabs RO needs admin rights to run, and the Windows prompt was answered No. Press Retry and choose Yes."
	case startNeedsElevation(err):
		return "HuffLabs RO needs admin rights to run. Press Retry and choose Yes on the Windows prompt."
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
