package main

import (
	"fmt"
	"os/exec"
)

type ClamAvScanner struct {
}

func (s *ClamAvScanner) ScanFile(path string) (bool, error) {
	cmd := exec.Command("./bin/clamscan", path)
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return false, nil
		}

		return false, fmt.Errorf("failed to scan file, %w", err)
	}

	return true, nil
}
