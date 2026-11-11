package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/themrsami/ugh/internal/rule"
	"github.com/themrsami/ugh/rules"
)

var version = "dev"

const usage = `ugh - explains the error you just got

  ugh                    re-run your last command and explain why it failed
                         (needs the shell hook, see --init)
  ugh <command> [args]   run a command, explain it if it fails
  ugh -c '<command>'     same, but through your shell (pipes, &&, etc.)
  <command> 2>&1 | ugh   explain output you already have

  ugh --list [prefix]    list rules
  ugh --new <tool/name>  start a new rule from the last error ugh didn't know
  ugh --check [dir]      test rules against their own examples
  ugh --init <shell>     print the shell hook: bash, zsh, fish or powershell
  ugh --all ...          show every matching rule, not just the best one

Rules are plain YAML. Your own go in ` + "`ugh --dir`" + ` and win over the built-ins.
ugh never runs a fix for you. It only prints.
`

func main() { os.Exit(realMain(os.Args[1:])) }

func realMain(args []string) int {
	all := false
	if len(args) > 0 && (args[0] == "--all" || args[0] == "-a") {
		all, args = true, args[1:]
	}

	if len(args) == 0 {
		if isTerminal(os.Stdin) {
			fmt.Fprint(os.Stderr, usage)
			fmt.Fprintln(os.Stderr, "\n(to make a bare `ugh` re-run your last command, set up the shell hook: ugh --init)")
			return 2
		}
		return pipe(all)
	}

	switch args[0] {
	case "-h", "--help":
		fmt.Print(usage)
		return 0
	case "--version":
		fmt.Println("ugh", buildVersion())
		return 0
	case "--dir":
		fmt.Println(userDir())
		return 0
	case "--init":
		return initShell(args[1:])
	case "--list":
		return list(args[1:])
	case "--check":
		return check(args[1:])
	case "--new":
		return newRule(args[1:])
	case "-c":
		// The PowerShell hook passes the command in the environment,
		// because PowerShell mangles quotes in arguments to programs.
		line := os.Getenv("UGH_CMD")
		if len(args) == 2 {
			line = args[1]
		} else if len(args) != 1 || line == "" {
			fmt.Fprintln(os.Stderr, "ugh: -c takes exactly one argument, the command line in quotes")
			return 2
		}
		return run(shellCommand(line), line, all)
	case "--":
		args = args[1:]
		if len(args) == 0 {
			return 2
		}
	}
	if strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(os.Stderr, "ugh: unknown option %s (try --help)\n", args[0])
		return 2
	}
	return run(exec.Command(args[0], args[1:]...), strings.Join(args, " "), all)
}

func load() *rule.Set {
	s := rule.NewSet()
	if err := s.Load(rules.FS); err != nil {
		// built-ins are tested in CI, so this is a broken build
		panic(err)
	}
	dir := userDir()
	if _, err := os.Stat(dir); err == nil {
		if err := s.Load(os.DirFS(dir)); err != nil {
			fmt.Fprintf(os.Stderr, "ugh: skipping your rules in %s: %v\n", dir, err)
		}
	}
	return s
}

func userDir() string {
	if d := os.Getenv("UGH_RULES"); d != "" {
		return d
	}
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "ugh", "rules")
	}
	return defaultRulesDir()
}

// tail keeps the end of the output. The error is nearly always in the last
// few screens, and nobody wants a 2GB build log held in memory.
type tail struct {
	mu  sync.Mutex
	buf []byte
}

const tailSize = 128 << 10

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > 2*tailSize {
		t.buf = append(t.buf[:0], t.buf[len(t.buf)-tailSize:]...)
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

func run(cmd *exec.Cmd, cmdline string, all bool) int {
	var t tail
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, &t)
	cmd.Stderr = io.MultiWriter(os.Stderr, &t)

	err := cmd.Run()
	if err == nil {
		return 0
	}
	code := 1
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else {
		// never started: not on PATH, not executable, that sort of thing
		code = 127
		msg := fmt.Sprintf("ugh: %v\n", err)
		fmt.Fprint(os.Stderr, msg)
		t.Write([]byte(msg))
	}
	explain(t.String(), cmdline, all)
	return code
}

func pipe(all bool) int {
	var t tail
	if _, err := io.Copy(io.MultiWriter(os.Stdout, &t), os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, "ugh:", err)
		return 1
	}
	explain(t.String(), "", all)
	return 0
}

func explain(out, cmdline string, all bool) {
	ms := load().Explain(out, cmdline)
	if len(ms) == 0 {
		remember(out, cmdline)
		renderUnknown(os.Stderr)
		return
	}
	if !all {
		render(os.Stderr, ms[0], ms[1:])
		return
	}
	for _, m := range ms {
		render(os.Stderr, m, nil)
	}
}

func list(args []string) int {
	prefix := ""
	if len(args) > 0 {
		prefix = args[0]
	}
	n := 0
	for _, r := range load().All() {
		if strings.HasPrefix(r.ID, prefix) {
			fmt.Printf("%-34s %s\n", r.ID, strings.NewReplacer("{{", "<", "}}", ">").Replace(r.Title))
			n++
		}
	}
	if n == 0 {
		fmt.Fprintf(os.Stderr, "ugh: no rules under %q\n", prefix)
		return 1
	}
	return 0
}

func check(args []string) int {
	s := rule.NewSet()
	if err := s.Load(rules.FS); err != nil {
		fmt.Fprintln(os.Stderr, "ugh:", err)
		return 1
	}
	dir := userDir()
	if len(args) > 0 {
		dir = args[0]
	}
	if _, err := os.Stat(dir); err == nil {
		if err := s.Load(os.DirFS(dir)); err != nil {
			fmt.Fprintln(os.Stderr, "ugh:", err)
			return 1
		}
	} else if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "ugh:", err)
		return 1
	}
	problems := s.Check()
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p)
	}
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d problem(s)\n", len(problems))
		return 1
	}
	fmt.Printf("%d rules, all good\n", len(s.All()))
	return 0
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

// Release builds get the version stamped in with -ldflags. For a plain
// `go install ...@v0.1.0` the module version is the next best thing.
func buildVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}
