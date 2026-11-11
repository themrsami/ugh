package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/themrsami/ugh/internal/rule"
)

// When nothing matches we keep the output around so `ugh --new` can drop it
// straight into the new rule as its first example. It never leaves the machine.

func lastFile() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "ugh", "last")
}

func remember(out, cmdline string) {
	f := lastFile()
	if f == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
		return
	}
	os.WriteFile(f, []byte(cmdline+"\n"+rule.Clean(out)), 0o600)
}

func recall() (cmdline string, lines []string) {
	b, err := os.ReadFile(lastFile())
	if err != nil {
		return "", nil
	}
	cmdline, out, _ := strings.Cut(string(b), "\n")
	lines = strings.Split(strings.TrimRight(out, "\n \t"), "\n")
	// errors live at the bottom; a 400-line example helps nobody
	if len(lines) > 12 {
		lines = lines[len(lines)-12:]
	}
	return cmdline, lines
}

var idRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9-]*$`)

func newRule(args []string) int {
	if len(args) != 1 || !idRE.MatchString(args[0]) {
		fmt.Fprintln(os.Stderr, "ugh: name the rule like <tool>/<short-name>, e.g. ugh --new git/no-upstream")
		return 2
	}
	id := args[0]

	// Inside a checkout of the ugh repo, write into the library itself.
	// Everywhere else, into the user's own rules.
	dir := userDir()
	if _, err := os.Stat("rules/rules.go"); err == nil {
		dir = "rules"
	}
	path := filepath.Join(dir, filepath.FromSlash(id)+".yml")
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "ugh: %s already exists\n", path)
		return 1
	}

	cmdline, lines := recall()
	tool := strings.SplitN(id, "/", 2)[0]

	var b strings.Builder
	// "|2" pins the block's indentation, otherwise YAML takes it from the
	// first line and chokes on output that starts with spaces (tracebacks).
	b.WriteString("title: \n")
	if cmdline != "" {
		fmt.Fprintf(&b, "# you ran: %s\n", cmdline)
	}
	fmt.Fprintf(&b, "# cmd: '^%s\\b'   # uncomment if this only makes sense for one command\n", tool)
	b.WriteString("match:\n  - ''\nwhy: |\n  \nfix:\n  - run: \n    note: \nexamples:\n  - |2\n")
	if len(lines) == 0 {
		b.WriteString("    paste the real error output here\n")
	}
	for _, l := range lines {
		b.WriteString(strings.TrimRight("    "+l, " ") + "\n")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "ugh:", err)
		return 1
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "ugh:", err)
		return 1
	}
	fmt.Println(path)
	if dir == "rules" {
		fmt.Fprintln(os.Stderr, "fill it in, then: go run . --check rules")
	} else {
		fmt.Fprintln(os.Stderr, "fill it in, then: ugh --check")
	}
	return 0
}
