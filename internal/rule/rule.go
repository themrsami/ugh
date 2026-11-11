// Package rule loads rules and matches them against command output.
package rule

import (
	"bytes"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Fix struct {
	Run  string `yaml:"run"`
	Note string `yaml:"note"`
}

// A fix can be written as a bare string when it doesn't need a note.
func (f *Fix) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		f.Run = n.Value
		return nil
	}
	type plain Fix
	return n.Decode((*plain)(f))
}

type Rule struct {
	ID       string   `yaml:"-"`
	Title    string   `yaml:"title"`
	Cmd      string   `yaml:"cmd"`
	Match    []string `yaml:"match"`
	Why      string   `yaml:"why"`
	Fix      []Fix    `yaml:"fix"`
	See      []string `yaml:"see"`
	Examples []string `yaml:"examples"`

	cmd   *regexp.Regexp
	match []*regexp.Regexp
}

type Match struct {
	Rule  *Rule
	Vars  map[string]string
	score int
}

type Set struct {
	rules map[string]*Rule
}

func NewSet() *Set { return &Set{rules: map[string]*Rule{}} }

// Load reads every .yml under fsys. Rules loaded later replace earlier ones
// with the same id, which is how a user's own rules override the built-ins.
func (s *Set) Load(fsys fs.FS) error {
	var first error
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".yml") {
			return err
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		r, err := Parse(strings.TrimSuffix(p, ".yml"), b)
		if err != nil {
			// one broken file shouldn't take the rest down with it
			if first == nil {
				first = err
			}
			return nil
		}
		s.rules[r.ID] = r
		return nil
	})
	if err != nil {
		return err
	}
	return first
}

func Parse(id string, b []byte) (*Rule, error) {
	r := &Rule{ID: id}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(r); err != nil {
		return nil, fmt.Errorf("%s: %w", id, err)
	}
	if r.Cmd != "" {
		re, err := regexp.Compile(r.Cmd)
		if err != nil {
			return nil, fmt.Errorf("%s: cmd: %w", id, err)
		}
		r.cmd = re
	}
	for _, m := range r.Match {
		if strings.TrimSpace(m) == "" {
			// an empty pattern matches everything; a half-written rule
			// should match nothing instead
			r.match = nil
			r.Match = nil
			break
		}
		// (?m) so ^ and $ mean "line", which is what everyone expects here.
		re, err := regexp.Compile("(?m)" + m)
		if err != nil {
			return nil, fmt.Errorf("%s: match: %w", id, err)
		}
		r.match = append(r.match, re)
	}
	return r, nil
}

func (s *Set) All() []*Rule {
	out := make([]*Rule, 0, len(s.rules))
	for _, r := range s.rules {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Try reports whether the rule applies. Every pattern has to match. An empty
// cmdline means we don't know what was run (piped input), so cmd is skipped.
func (r *Rule) Try(out, cmdline string) (Match, bool) {
	m := Match{Rule: r, Vars: map[string]string{}}
	if len(r.match) == 0 {
		return m, false
	}
	if r.cmd != nil && cmdline != "" {
		if !r.cmd.MatchString(cmdline) {
			return m, false
		}
		m.score += 1000
	}
	for _, re := range r.match {
		loc := re.FindStringSubmatchIndex(out)
		if loc == nil {
			return m, false
		}
		m.score += loc[1] - loc[0]
		for i, name := range re.SubexpNames() {
			if name == "" || loc[2*i] < 0 {
				continue
			}
			// Go won't let two alternatives share a group name, so port_2
			// fills in port.
			m.Vars[alias.ReplaceAllString(name, "")] = printable(out[loc[2*i]:loc[2*i+1]])
		}
	}
	return m, true
}

// Explain returns every matching rule, most specific first. "Specific" is
// just how much text the patterns matched, which works better than it sounds:
// a rule that recognises a whole line beats one that spotted two words in it.
func (s *Set) Explain(out, cmdline string) []Match {
	out = Clean(out)
	var ms []Match
	for _, r := range s.rules {
		if m, ok := r.Try(out, cmdline); ok {
			ms = append(ms, m)
		}
	}
	sort.Slice(ms, func(i, j int) bool {
		if ms[i].score != ms[j].score {
			return ms[i].score > ms[j].score
		}
		return ms[i].Rule.ID < ms[j].Rule.ID
	})
	return ms
}

var alias = regexp.MustCompile(`_\d+$`)

var placeholder = regexp.MustCompile(`\{\{\s*(\w+)\s*\}\}`)

// Expand fills {{name}} from the named capture groups. Unknown names are
// left alone so a typo in a rule is visible instead of silently empty.
func (m Match) Expand(s string) string {
	return placeholder.ReplaceAllStringFunc(s, func(p string) string {
		name := placeholder.FindStringSubmatch(p)[1]
		if v, ok := m.Vars[name]; ok {
			return v
		}
		return p
	})
}

var ansi = regexp.MustCompile(`\x1b(\[[0-9;?]*[ -/]*[@-~]|\][^\x07\x1b]*(\x07|\x1b\\)|[@-Z\\-_])`)

// Clean strips colour codes and turns progress-bar carriage returns into
// newlines, so rules only ever see plain text.
func Clean(s string) string {
	s = ansi.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// Captured text ends up on someone's terminal, so no control characters.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// Check lints the set and runs every rule against its own examples. A rule
// must not only match its example, it must win it: if some other rule scores
// higher on your example, one of the two is too greedy.
func (s *Set) Check() []string {
	var problems []string
	bad := func(r *Rule, format string, a ...any) {
		problems = append(problems, r.ID+": "+fmt.Sprintf(format, a...))
	}
	for _, r := range s.All() {
		if !strings.Contains(r.ID, "/") {
			bad(r, "rules live one folder deep, like git/no-upstream")
		}
		if strings.TrimSpace(r.Title) == "" {
			bad(r, "missing title")
		}
		if strings.TrimSpace(r.Why) == "" {
			bad(r, "missing why")
		}
		if len(r.Match) == 0 {
			bad(r, "missing match (or one of the patterns is empty)")
		}
		if len(r.Examples) == 0 {
			bad(r, "needs at least one example of the real error output")
		}
		for i, ex := range r.Examples {
			ms := s.Explain(ex, "")
			if len(ms) == 0 || !contains(ms, r) {
				bad(r, "example %d doesn't match its own rule", i+1)
				continue
			}
			if ms[0].Rule != r {
				bad(r, "example %d is claimed by %s first", i+1, ms[0].Rule.ID)
				continue
			}
			text := r.Title + r.Why
			for _, f := range r.Fix {
				text += f.Run + f.Note
			}
			if left := placeholder.FindString(ms[0].Expand(text)); left != "" {
				bad(r, "example %d leaves %s unfilled", i+1, left)
			}
		}
	}
	return problems
}

func contains(ms []Match, r *Rule) bool {
	for _, m := range ms {
		if m.Rule == r {
			return true
		}
	}
	return false
}
