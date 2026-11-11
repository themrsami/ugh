//go:build !windows

package main

import (
	"os"
	"os/exec"
)

func shellCommand(line string) *exec.Cmd {
	sh := os.Getenv("UGH_SHELL")
	if sh == "" {
		sh = os.Getenv("SHELL")
	}
	if sh == "" {
		sh = "/bin/sh"
	}
	return exec.Command(sh, "-c", line)
}

func colorCapable(*os.File) bool { return true }

func defaultRulesDir() string {
	home, _ := os.UserHomeDir()
	return home + "/.config/ugh/rules"
}
