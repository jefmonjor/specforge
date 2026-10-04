// Package lessons keeps the project's curated lessons: one-sentence rules
// the agent wrote after a rejected attempt, tagged with the stack, without
// duplicates and capped so the list stays worth reading.
//
// The file is plain Markdown in specs/LESSONS.md, committed with the code:
// the team reads, edits and deletes lessons like any other document.
package lessons

import (
	"fmt"
	"regexp"
	"strings"
)

// Max is how many lessons are kept; the oldest go first.
const Max = 30

const header = "# Lessons\n\n<!-- Rules the agent wrote after a rejected attempt. SpecForge keeps the latest 30, without duplicates, and shows those of the current stack in every prompt. Edit or delete them freely. -->\n\n"

var entry = regexp.MustCompile(`^- \[([a-z0-9-]+)\] (.+)$`)

// Lesson is one rule.
type Lesson struct {
	Stack string
	Text  string
}

// Parse reads the lessons of a LESSONS.md file; other lines are ignored.
func Parse(markdown string) []Lesson {
	var out []Lesson
	for _, line := range strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n") {
		if m := entry.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			out = append(out, Lesson{Stack: m[1], Text: m[2]})
		}
	}
	return out
}

// Add returns the file with l appended, unless an equivalent lesson is
// already there (added is then false). Only the latest Max are kept.
func Add(markdown string, l Lesson) (out string, added bool) {
	l.Text = strings.Join(strings.Fields(l.Text), " ")
	if l.Text == "" {
		return markdown, false
	}
	all := Parse(markdown)
	for _, old := range all {
		if old.Stack == l.Stack && normal(old.Text) == normal(l.Text) {
			return markdown, false
		}
	}
	all = append(all, l)
	if len(all) > Max {
		all = all[len(all)-Max:]
	}
	var b strings.Builder
	b.WriteString(header)
	for _, x := range all {
		fmt.Fprintf(&b, "- [%s] %s\n", x.Stack, x.Text)
	}
	return b.String(), true
}

// For returns the lessons of stack as a Markdown list.
func For(markdown, stack string) string {
	var lines []string
	for _, l := range Parse(markdown) {
		if l.Stack == stack {
			lines = append(lines, "- "+l.Text)
		}
	}
	return strings.Join(lines, "\n")
}

func normal(s string) string {
	return strings.TrimRight(strings.ToLower(strings.Join(strings.Fields(s), " ")), ".")
}
