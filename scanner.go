package main

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
)

type ClamAvScanner struct {
}

func (s *ClamAvScanner) ScanFile(path string) (bool, error) {
	cmd := exec.Command("./bin/clamscan", "--verbose", "--stdout", "-d", "/tmp/usr/clamav", path)

	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			log.Print(outb.String())
			log.Print(errb.String())
			return false, nil
		}

		log.Print(outb.String())
		log.Print(errb.String())
		return false, fmt.Errorf("failed to scan file, %w", err)
	}

	log.Print(outb.String())
	log.Print(errb.String())
	return true, nil
}
