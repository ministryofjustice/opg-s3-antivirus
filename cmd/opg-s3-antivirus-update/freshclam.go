package main

import (
	"os"
	"os/exec"
)

type Freshclam struct{}

func (c *Freshclam) Update() error {
	cmd := exec.Command("./bin/freshclam")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	return cmd.Run()
}
