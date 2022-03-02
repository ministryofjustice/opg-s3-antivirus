package main

import (
	"os"
	"os/exec"
)

type Freshclam struct{}

func (c *Freshclam) Update() error {
	cmd := exec.Command("freshclam", "--config-file=/etc/freshclam.conf")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	return cmd.Run()
}
