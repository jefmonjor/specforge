package legacy

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Citation is a reference to legacy code: a path, optionally with a line
// or a line range.
type Citation struct {
	Path      string
	From, To  int // 0 when no line was given
	Reference string
}

// citation matches `src/Foo.java`, `src/Foo.java:42` and `src/Foo.java:42-57`
// inside backticks: the form the prompts ask for. Only source and
// configuration extensions count, so `java.util.Vector` or `1.6` in a
// sentence is not taken for a file.
var citation = regexp.MustCompile("`([^`\\s]+\\.(?i:" + sourceExtensions + "))(?::(\\d+)(?:-(\\d+))?)?`")

const sourceExtensions = "java|jsp|jspf|jspx|tag|tld|xml|xsd|wsdl|properties|yaml|yml|json|sql|groovy|kt|scala|gradle|js|jsx|ts|tsx|vue|html|htm|css|py|go|cs|vb|cbl|cob|sh|bat|ftl|vm|txt|md|mf"

// Citations returns the legacy references in a document, in order and
// without duplicates.
func Citations(markdown string) []Citation {
	var out []Citation
	seen := map[string]bool{}
	for _, m := range citation.FindAllStringSubmatch(markdown, -1) {
		if seen[m[0]] {
			continue
		}
		seen[m[0]] = true
		c := Citation{Path: strings.TrimPrefix(path.Clean(m[1]), "./"), Reference: strings.Trim(m[0], "`")}
		c.From, _ = strconv.Atoi(m[2])
		c.To, _ = strconv.Atoi(m[3])
		if c.To == 0 {
			c.To = c.From
		}
		out = append(out, c)
	}
	return out
}

// Verify checks citations against the legacy codebase: every file must
// exist and every line must be inside it. A citation that does not resolve
// is an invented source, and the document is sent back.
func Verify(fsys fs.FS, cs []Citation) []string {
	var problems []string
	lines := map[string]int{}
	for _, c := range cs {
		n, ok := lines[c.Path]
		if !ok {
			data, err := fs.ReadFile(fsys, c.Path)
			if err != nil {
				problems = append(problems, fmt.Sprintf("`%s` does not exist in the legacy code", c.Reference))
				lines[c.Path] = -1
				continue
			}
			n = strings.Count(string(data), "\n") + 1
			lines[c.Path] = n
		}
		if n < 0 {
			continue
		}
		if c.From > 0 && (c.To > n || c.From > c.To) {
			problems = append(problems, fmt.Sprintf("`%s` points past the end of the file (%d lines)", c.Reference, n))
		}
	}
	return problems
}
