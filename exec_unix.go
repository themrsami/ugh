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

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	// /dev/null is a character device too, and it's what stdin is in CI
	if null, err := os.Stat(os.DevNull); err == nil && os.SameFile(st, null) {
		return false
	}
	return true
}
