package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func powershell(t *testing.T, line string) (string, int) {
	t.Helper()
	tmp := t.TempDir()
	cmd := exec.Command(bin, "-c")
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(), "UGH_SHELL=powershell", "UGH_CMD="+line, "UGH_RULES="+tmp)
	b, _ := cmd.CombinedOutput()
	return string(b), cmd.ProcessState.ExitCode()
}

func TestPowerShellNativeExitCode(t *testing.T) {
	out, code := powershell(t, `cmd /c "echo fatal: not a git repository 1>&2 & exit 7"`)
	if code != 7 {
		t.Errorf("exit code %d, want 7\n%s", code, out)
	}
	if !strings.Contains(out, "git/not-a-repo") {
		t.Errorf("no explanation:\n%s", out)
	}
}

func TestPowerShellQuotesSurvive(t *testing.T) {
	out, code := powershell(t, `Write-Output "it's ""quoted"" & fine"`)
	if code != 0 || !strings.Contains(out, `it's "quoted" & fine`) {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestPowerShellUnknownCommand(t *testing.T) {
	out, code := powershell(t, "surely-not-a-real-program-xyz --version")
	if code == 0 {
		t.Error("a failed cmdlet lookup should not exit 0")
	}
	if !strings.Contains(out, "windows/command-not-found") {
		t.Errorf("got:\n%s", out)
	}
}

func TestCmd(t *testing.T) {
	tmp := t.TempDir()
	cmd := exec.Command(bin, "-c", `surely-not-a-real-program-xyz "a b" & exit 5`)
	cmd.Dir = tmp
	env := []string{"UGH_RULES=" + tmp}
	for _, kv := range os.Environ() {
		// no SHELL and no UGH_SHELL means cmd.exe
		if !strings.HasPrefix(kv, "SHELL=") && !strings.HasPrefix(kv, "UGH_SHELL=") {
			env = append(env, kv)
		}
	}
	cmd.Env = env
	b, _ := cmd.CombinedOutput()
	if code := cmd.ProcessState.ExitCode(); code != 5 {
		t.Errorf("exit code %d, want 5\n%s", code, b)
	}
	if !strings.Contains(string(b), "windows/command-not-found") {
		t.Errorf("got:\n%s", b)
	}
}
