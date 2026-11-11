package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/themrsami/ugh/internal/rule"
)

type pen struct{ on bool }

func (p pen) wrap(code, s string) string {
	if !p.on {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}
func (p pen) bold(s string) string { return p.wrap("1", s) }
func (p pen) dim(s string) string  { return p.wrap("2", s) }
func (p pen) cyan(s string) string { return p.wrap("36", s) }

func newPen() pen {
	_, noColor := os.LookupEnv("NO_COLOR")
	if noColor || os.Getenv("TERM") == "dumb" {
		return pen{}
	}
	// Git Bash's terminal looks like a pipe to a Windows program, so
	// there's an override for it. The bash hook sets it when it's safe to.
	if os.Getenv("UGH_COLOR") == "1" {
		return pen{on: true}
	}
	return pen{on: isTerminal(os.Stderr) && colorCapable(os.Stderr)}
}

func render(w io.Writer, m rule.Match, others []rule.Match) {
	p := newPen()
	r := m.Rule

	fmt.Fprintf(w, "\n  %s\n", p.dim("ugh · "+r.ID))
	fmt.Fprintf(w, "  %s\n\n", p.bold(m.Expand(strings.TrimSpace(r.Title))))
	for _, line := range strings.Split(strings.TrimSpace(m.Expand(r.Why)), "\n") {
		fmt.Fprintf(w, "  %s\n", line)
	}

	if len(r.Fix) > 0 {
		fmt.Fprintln(w)
		width := 0
		runs := make([]string, len(r.Fix))
		for i, f := range r.Fix {
			runs[i] = m.Expand(f.Run)
			if f.Note != "" && len(runs[i]) > width && len(runs[i]) <= 48 {
				width = len(runs[i])
			}
		}
		for i, f := range r.Fix {
			note := ""
			if f.Note != "" {
				pad := width - len(runs[i])
				if pad < 0 {
					pad = 0
				}
				note = strings.Repeat(" ", pad) + p.dim("  # "+m.Expand(f.Note))
			}
			fmt.Fprintf(w, "    %s %s%s\n", p.dim("$"), p.cyan(runs[i]), note)
		}
	}

	if len(r.See) > 0 {
		fmt.Fprintln(w)
		for _, s := range r.See {
			fmt.Fprintf(w, "  %s\n", p.dim(s))
		}
	}

	if len(others) > 0 {
		ids := make([]string, len(others))
		for i, o := range others {
			ids[i] = o.Rule.ID
		}
		fmt.Fprintf(w, "\n  %s\n", p.dim("also matched: "+strings.Join(ids, ", ")+"  (ugh --all)"))
	}
	fmt.Fprintln(w)
}

func renderUnknown(w io.Writer) {
	p := newPen()
	fmt.Fprintf(w, "\n  %s\n", p.dim("ugh"))
	fmt.Fprintf(w, "  %s\n\n", p.bold("No rule for this one yet."))
	fmt.Fprintln(w, "  If you figure it out, save the next person the search:")
	fmt.Fprintf(w, "    %s %s\n\n", p.dim("$"), p.cyan("ugh --new <tool>/<short-name>"))
}
