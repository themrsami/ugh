package main

import (
	"encoding/base64"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

// Windows has three shells worth caring about and each takes a command
// line differently. UGH_SHELL is set by the PowerShell hook; $SHELL only
// exists inside Git Bash and MSYS2; with neither we assume cmd.
func shellCommand(line string) *exec.Cmd {
	sh := os.Getenv("UGH_SHELL")
	if sh == "" && os.Getenv("SHELL") != "" {
		// $SHELL is something like /usr/bin/bash, a path only MSYS
		// programs understand. bash.exe is on PATH in those shells.
		sh = "bash"
	}
	if sh == "" {
		sh = "cmd"
	}

	switch strings.TrimSuffix(strings.ToLower(filepath.Base(sh)), ".exe") {
	case "powershell", "pwsh":
		// -EncodedCommand sidesteps every quoting problem there is.
		// PowerShell doesn't pass a native program's exit code on by
		// itself, hence the tail.
		script := "$ProgressPreference = 'SilentlyContinue'\n" + line +
			"\n$ughOK = $?\nif ($LASTEXITCODE) { exit $LASTEXITCODE }\nif (-not $ughOK) { exit 1 }\n"
		u := utf16.Encode([]rune(script))
		b := make([]byte, 2*len(u))
		for i, c := range u {
			binary.LittleEndian.PutUint16(b[2*i:], c)
		}
		return exec.Command(sh, "-NoLogo", "-NoProfile", "-EncodedCommand", base64.StdEncoding.EncodeToString(b))
	case "cmd":
		// cmd has its own quoting rules, so hand it the line verbatim
		// instead of letting Go escape it.
		c := exec.Command(sh)
		c.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd /S /C "` + line + `"`}
		return c
	}
	return exec.Command(sh, "-c", line)
}

// Older consoles print escape codes literally unless asked not to.
func colorCapable(f *os.File) bool {
	const enableVirtualTerminalProcessing = 0x0004
	k32 := syscall.NewLazyDLL("kernel32.dll")
	var mode uint32
	if r, _, _ := k32.NewProc("GetConsoleMode").Call(f.Fd(), uintptr(unsafe.Pointer(&mode))); r == 0 {
		return false
	}
	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}
	r, _, _ := k32.NewProc("SetConsoleMode").Call(f.Fd(), uintptr(mode|enableVirtualTerminalProcessing))
	return r != 0
}

func defaultRulesDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir, _ = os.UserHomeDir()
	}
	return filepath.Join(dir, "ugh", "rules")
}
