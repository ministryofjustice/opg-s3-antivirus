package main

import (
	"fmt"
	"os"
	"os/exec"
)

type ClamAvScanner struct {
}

func (s *ClamAvScanner) StartDaemon() (error) {
	cmd := exec.Command("clamd", "--config-file", "/etc/clamd.conf")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to scan file, %w", err)
	}

	return nil
}

func (s *ClamAvScanner) ScanFile(path string) (bool, error) {
	cmd := exec.Command("clamdscan", "--config-file", "/etc/clamd.conf", "--stdout", path)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return false, nil
		}

		return false, fmt.Errorf("failed to scan file, %w", err)
	}

	return true, nil
}
