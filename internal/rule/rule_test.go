package rule

import (
	"testing"

	"github.com/themrsami/ugh/rules"
)

func library(t *testing.T) *Set {
	t.Helper()
	s := NewSet()
	if err := s.Load(rules.FS); err != nil {
		t.Fatal(err)
	}
	return s
}

// Every rule in the library has to pass its own examples.
func TestLibrary(t *testing.T) {
	s := library(t)
	if len(s.All()) == 0 {
		t.Fatal("no rules loaded")
	}
	for _, p := range s.Check() {
		t.Error(p)
	}
}

func TestCaptures(t *testing.T) {
	ms := library(t).Explain("Error: listen EADDRINUSE: address already in use :::3000\n", "npm run dev")
	if len(ms) == 0 {
		t.Fatal("no match")
	}
	m := ms[0]
	if m.Rule.ID != "node/eaddrinuse" {
		t.Fatalf("got %s", m.Rule.ID)
	}
	if got := m.Expand("lsof -i :{{port}} {{nope}}"); got != "lsof -i :3000 {{nope}}" {
		t.Errorf("got %q", got)
	}
}

func TestCmdFilter(t *testing.T) {
	r, err := Parse("x/y", []byte("cmd: '^git\\b'\nmatch: ['boom']\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Try("boom", "npm install"); ok {
		t.Error("matched a command it shouldn't have")
	}
	if _, ok := r.Try("boom", "git push"); !ok {
		t.Error("didn't match git")
	}
	if _, ok := r.Try("boom", ""); !ok {
		t.Error("unknown command should skip the cmd filter")
	}
}

func TestClean(t *testing.T) {
	in := "\x1b[31merror\x1b[0m: nope\r\n 10%\r 50%\rdone"
	want := "error: nope\n 10%\n 50%\ndone"
	if got := Clean(in); got != want {
		t.Errorf("got %q", got)
	}
}

func TestCapturesAreSanitised(t *testing.T) {
	r, _ := Parse("x/y", []byte("match: ['file (?P<f>.+) missing']\n"))
	m, ok := r.Try("file a\x1b]0;pwned\x07b missing", "")
	if !ok {
		t.Fatal("no match")
	}
	if m.Vars["f"] != "a]0;pwnedb" {
		t.Errorf("got %q", m.Vars["f"])
	}
}

func TestUnknownFieldIsAnError(t *testing.T) {
	if _, err := Parse("x/y", []byte("titel: oops\n")); err == nil {
		t.Error("typo in a field name should fail loudly")
	}
}
