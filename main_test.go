package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These run the real binary, since most of what main does is plumbing
// between a child process and the terminal.

var bin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ugh-test")
	if err != nil {
		panic(err)
	}
	bin = filepath.Join(dir, "ugh")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		panic(string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func ugh(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	tmp := t.TempDir()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(),
		"UGH_RULES="+filepath.Join(tmp, "rules"),
		"XDG_CACHE_HOME="+filepath.Join(tmp, "cache"),
		"HOME="+tmp,
		"SHELL=/bin/sh",
	)
	cmd.Dir = tmp
	cmd.Stdin = strings.NewReader(stdin)
	var o, e strings.Builder
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return o.String(), e.String(), code
}

func TestSuccessIsSilent(t *testing.T) {
	out, errOut, code := ugh(t, "", "-c", "echo fine")
	if out != "fine\n" || errOut != "" || code != 0 {
		t.Errorf("out=%q err=%q code=%d", out, errOut, code)
	}
}

func TestExitCodeAndOutputPassThrough(t *testing.T) {
	out, errOut, code := ugh(t, "", "-c", "echo to-stdout; echo 'fatal: not a git repository' >&2; exit 42")
	if code != 42 {
		t.Errorf("exit code %d, want 42", code)
	}
	if out != "to-stdout\n" {
		t.Errorf("stdout %q", out)
	}
	if !strings.HasPrefix(errOut, "fatal: not a git repository\n") {
		t.Errorf("child's stderr should come first, got %q", errOut)
	}
	if !strings.Contains(errOut, "git/not-a-repo") {
		t.Errorf("no explanation in %q", errOut)
	}
}

func TestPipe(t *testing.T) {
	in := "Error: listen EADDRINUSE: address already in use :::3000\n"
	out, errOut, code := ugh(t, in)
	if out != in {
		t.Errorf("piped input should be echoed unchanged, got %q", out)
	}
	if code != 0 || !strings.Contains(errOut, "lsof -i :3000") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestMissingBinary(t *testing.T) {
	_, errOut, code := ugh(t, "", "surely-not-a-real-program-xyz")
	if code != 127 {
		t.Errorf("exit code %d, want 127 like a shell", code)
	}
	if !strings.Contains(errOut, "sys/command-not-found") {
		t.Errorf("got %q", errOut)
	}
}

func TestStdinReachesChild(t *testing.T) {
	out, _, code := ugh(t, "hello\n", "cat")
	if out != "hello\n" || code != 0 {
		t.Errorf("out=%q code=%d", out, code)
	}
}

func TestUnknownErrorThenNew(t *testing.T) {
	tmp := t.TempDir()
	env := append(os.Environ(), "UGH_RULES="+filepath.Join(tmp, "rules"), "XDG_CACHE_HOME="+filepath.Join(tmp, "cache"), "HOME="+tmp, "SHELL=/bin/sh")
	run := func(args ...string) (string, int) {
		cmd := exec.Command(bin, args...)
		cmd.Env, cmd.Dir = env, tmp
		b, _ := cmd.CombinedOutput()
		return string(b), cmd.ProcessState.ExitCode()
	}

	out, code := run("-c", "echo 'flux capacitor misaligned' >&2; exit 3")
	if code != 3 || !strings.Contains(out, "No rule for this one yet") {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if out, code = run("--new", "flux/misaligned"); code != 0 {
		t.Fatal(out)
	}
	path := filepath.Join(tmp, "rules", "flux", "misaligned.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "    flux capacitor misaligned\n") {
		t.Errorf("scaffold is missing the captured output:\n%s", b)
	}

	// a half-written rule must not match anything, and must fail --check
	if out, _ = run("-c", "echo unrelated >&2; exit 1"); strings.Contains(out, "flux/misaligned") {
		t.Errorf("empty scaffold matched: %q", out)
	}
	if _, code = run("--check"); code == 0 {
		t.Error("--check passed an empty rule")
	}

	filled := "title: Flux capacitor is off\nmatch:\n  - 'flux capacitor (?P<state>\\w+)'\nwhy: |\n  It is {{state}}.\nexamples:\n  - |\n    flux capacitor misaligned\n"
	if err := os.WriteFile(path, []byte(filled), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code = run("--check"); code != 0 {
		t.Fatalf("--check: %s", out)
	}
	if out, _ = run("-c", "echo 'flux capacitor misaligned' >&2; exit 3"); !strings.Contains(out, "It is misaligned.") {
		t.Errorf("user rule didn't fire: %q", out)
	}
}

func TestTailKeepsTheEnd(t *testing.T) {
	var tl tail
	chunk := []byte(strings.Repeat("x", 1000) + "\n")
	for i := 0; i < 1000; i++ {
		tl.Write(chunk)
	}
	tl.Write([]byte("the actual error\n"))
	s := tl.String()
	if len(s) > 2*tailSize+len(chunk) {
		t.Errorf("buffer grew to %d bytes", len(s))
	}
	if !strings.HasSuffix(s, "the actual error\n") {
		t.Error("lost the end of the output")
	}
}
