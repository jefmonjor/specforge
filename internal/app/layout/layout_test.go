package layout

import (
	"path/filepath"
	"testing"
)

func TestLayout(t *testing.T) {
	root := filepath.FromSlash("/repo")
	l := Layout{Root: root}
	spec := filepath.Join(root, "specs", "0001-reset.md")
	cases := map[string]string{
		l.SpecDir(spec):   "specs/0001-reset",
		l.Decisions(spec): "specs/0001-reset/decisions.md",
		l.Questions(spec): "specs/0001-reset/questions.md",
		l.State(spec):     ".specforge/state/0001-reset.json",
		l.Lessons():       "specs/LESSONS.md",
	}
	for got, want := range cases {
		if l.Rel(got) != want {
			t.Errorf("got %s, want %s", l.Rel(got), want)
		}
	}
	if l.Abs("specs/x.md") != filepath.Join(root, "specs", "x.md") {
		t.Error("Abs")
	}
}
