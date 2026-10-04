package spec

import (
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
