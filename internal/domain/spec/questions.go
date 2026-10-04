package spec

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	questionLine = regexp.MustCompile(`(?i)^\s*(?:[-*+]|\d+[.)])\s*\[NEEDS CLARIFICATION\]\s*:?\s*(.*)$`)
	placeholder  = regexp.MustCompile(`^<[^>]*>$`)
	commentOpen  = "<!--"
	commentClose = "-->"
)

// OpenQuestions returns the open questions of a specification: list items
// that start with [NEEDS CLARIFICATION]. Prose that merely mentions the
// marker, items inside code blocks or HTML comments, and template
// placeholders such as "<write the question here>" are not questions.
func OpenQuestions(markdown string) []string {
	var out []string
	inFence, inComment := false, false
	for _, line := range strings.Split(normalizeNewlines(markdown), "\n") {
		trimmed := strings.TrimSpace(line)
		if fenceOpen.MatchString(trimmed) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if inComment {
			if strings.Contains(trimmed, commentClose) {
				inComment = false
			}
			continue
		}
		if strings.HasPrefix(trimmed, commentOpen) && !strings.Contains(trimmed, commentClose) {
			inComment = true
			continue
		}
		m := questionLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		q := strings.TrimSpace(m[1])
		if q == "" || placeholder.MatchString(q) {
			continue
		}
		out = append(out, q)
	}
	return out
}

// ResolveQuestion replaces the open question q with its decision, so the
// specification records what was decided and no longer blocks approval:
//
//   - **Decided:** q → answer (note)
//
// ok is false when q is not an open question of markdown.
func ResolveQuestion(markdown, q, answer, note string) (string, bool) {
	lines := strings.Split(normalizeNewlines(markdown), "\n")
	inFence, inComment := false, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case fenceOpen.MatchString(trimmed):
			inFence = !inFence
			continue
		case inFence:
			continue
		case inComment:
			inComment = !strings.Contains(trimmed, commentClose)
			continue
		case strings.HasPrefix(trimmed, commentOpen) && !strings.Contains(trimmed, commentClose):
			inComment = true
			continue
		}
		m := questionLine.FindStringSubmatch(line)
		if m == nil || strings.TrimSpace(m[1]) != strings.TrimSpace(q) {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		decided := fmt.Sprintf("%s- **Decided:** %s → %s", indent, strings.TrimSpace(q), strings.TrimSpace(answer))
		if note != "" {
			decided += " (" + note + ")"
		}
		lines[i] = decided
		return strings.Join(lines, "\n"), true
	}
	return markdown, false
}
