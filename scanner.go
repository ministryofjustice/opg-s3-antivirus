package main

import (
	"fmt"
	"os/exec"
)

type ClamAvScanner struct {
}

func (s *ClamAvScanner) ScanFile(path string) (bool, error) {
	pass := true
	cmd := exec.Command("./bin/clamscan", path)
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 {
				pass = false
			} else {
				return false, fmt.Errorf("failed to scan file, %w", err)
			}
		} else {
			return false, fmt.Errorf("failed to scan file, %w", err)
		}
	}

	return pass, nil
}
