// Package layout knows where SpecForge keeps things inside a project.
//
// Everything a reviewer should see is committed under specs/: the sealed
// specification, and next to it a directory with its decisions, open
// questions and delivery report. Machine state lives in .specforge/, which
// setup adds to .gitignore.
package layout

import (
	"path/filepath"
	"strings"
)

// Layout resolves paths for one project root.
type Layout struct {
	Root string
}

// SpecsDir is where specifications live.
func (l Layout) SpecsDir() string { return filepath.Join(l.Root, "specs") }

// LocalDir is SpecForge's private, git-ignored working directory.
func (l Layout) LocalDir() string { return filepath.Join(l.Root, ".specforge") }

// SpecDir is the artifacts directory of a specification:
// specs/0001-reset.md → specs/0001-reset/.
func (l Layout) SpecDir(specPath string) string {
	return strings.TrimSuffix(specPath, filepath.Ext(specPath))
}

// Decisions is the log of questions the developer answered.
func (l Layout) Decisions(specPath string) string {
	return filepath.Join(l.SpecDir(specPath), "decisions.md")
}

// Approvals is the history of a specification's approvals.
func (l Layout) Approvals(specPath string) string {
	return filepath.Join(l.SpecDir(specPath), "approvals.md")
}

// Plan is the technical plan of a specification.
func (l Layout) Plan(specPath string) string {
	return filepath.Join(l.SpecDir(specPath), "plan.md")
}

// Questions holds questions asked when nobody was at the terminal.
func (l Layout) Questions(specPath string) string {
	return filepath.Join(l.SpecDir(specPath), "questions.md")
}

// State is the loop state of a specification.
func (l Layout) State(specPath string) string {
	base := strings.TrimSuffix(filepath.Base(specPath), filepath.Ext(specPath))
	return filepath.Join(l.LocalDir(), "state", base+".json")
}

// ReviewRecord is where the lens review of one scenario is kept.
func (l Layout) ReviewRecord(specPath, marker string) string {
	return filepath.Join(l.SpecDir(specPath), "review", marker+".json")
}

// Lessons is the project's curated list of lessons learned.
func (l Layout) Lessons() string { return filepath.Join(l.SpecsDir(), "LESSONS.md") }

// Rel returns path relative to the root with forward slashes, or path
// unchanged when it is outside the root.
func (l Layout) Rel(path string) string {
	rel, err := filepath.Rel(l.Root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// Abs resolves a slash-separated project-relative path.
func (l Layout) Abs(rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(l.Root, filepath.FromSlash(rel))
}
