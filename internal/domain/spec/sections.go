package spec

import (
	"regexp"
	"strings"
)

var heading = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)

// Common section titles, in Spanish and English.
var (
	GlossaryTitle   = regexp.MustCompile(`(?i)lenguaje ubicuo|glosario|glossary|ubiquitous language`)
	InvariantsTitle = regexp.MustCompile(`(?i)invariant`)
)

// Section returns the body under the first Markdown heading whose text
// matches title, up to the next heading of the same or a higher level.
func Section(markdown string, title *regexp.Regexp) string {
	var out []string
	level := 0
	inFence := false
	for _, line := range strings.Split(normalizeNewlines(markdown), "\n") {
		if fenceOpen.MatchString(strings.TrimSpace(line)) {
			inFence = !inFence
		}
		if !inFence {
			if m := heading.FindStringSubmatch(line); m != nil {
				if level > 0 && len(m[1]) <= level {
					break
				}
				if level == 0 && title.MatchString(m[2]) {
					level = len(m[1])
					continue
				}
			}
		}
		if level > 0 {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
