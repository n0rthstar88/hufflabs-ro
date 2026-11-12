//go:build !windows

package main

import "os/exec"

// startProcess: only for tests and previews on Linux; players on Linux run
// the Windows build through Proton.
func startProcess(path, dir string) (gameProcess, error) {
	cmd := exec.Command(path)
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
