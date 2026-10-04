package specs

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"specforge/internal/domain/spec"
)

// Change is how a scenario changed between two approvals.
type Change string

const (
	Added     Change = "ADDED"
	Modified  Change = "MODIFIED"
	Unchanged Change = "UNCHANGED"
	Removed   Change = "REMOVED"
)

// ScenarioChange is one line of an approval record.
type ScenarioChange struct {
	Change      Change
	Index       int
	Title       string
	Fingerprint string // the first 12 hex characters
}

var approvalLine = regexp.MustCompile("^- (ADDED|MODIFIED|UNCHANGED) · (\\d+) · (.+) · `([0-9a-f]{12})`$")

// Delta compares the scenarios of a new approval with the last recorded
// one. A scenario keeps its identity by title: same title and content is
// UNCHANGED, same title with new content is MODIFIED.
func Delta(previous []ScenarioChange, doc *spec.Document) []ScenarioChange {
	before := map[string]string{}
	for _, p := range previous {
		before[p.Title] = p.Fingerprint
	}
	var out []ScenarioChange
	seen := map[string]bool{}
	for _, sc := range doc.Scenarios {
		fp := sc.Fingerprint()[:12]
		c := ScenarioChange{Change: Added, Index: sc.Index, Title: sc.Title, Fingerprint: fp}
		if old, ok := before[sc.Title]; ok {
			c.Change = Modified
			if old == fp {
				c.Change = Unchanged
			}
		}
		seen[sc.Title] = true
		out = append(out, c)
	}
	for _, p := range previous {
		if !seen[p.Title] {
			out = append(out, ScenarioChange{Change: Removed, Title: p.Title})
		}
	}
	return out
}

// lastApproval reads the scenarios of the latest record in approvals.md.
func lastApproval(log string) []ScenarioChange {
	var last []ScenarioChange
	for _, line := range strings.Split(log, "\n") {
		if strings.HasPrefix(line, "### ") {
			last = nil
			continue
		}
		if m := approvalLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			idx, err := strconv.Atoi(m[2])
			if err != nil {
				continue
			}
			last = append(last, ScenarioChange{Change: Change(m[1]), Index: idx, Title: m[3], Fingerprint: m[4]})
		}
	}
	return last
}

// recordApproval appends the approval and its scenario delta to
// specs/NNNN-slug/approvals.md, the history of what was approved when.
func (s Service) recordApproval(e Entry, content, by, hash string, at time.Time) ([]ScenarioChange, error) {
	doc, err := spec.Parse(content, spec.ParseOptions{Languages: []string{s.Language}})
	if err != nil {
		return nil, err
	}
	path := s.Layout.Approvals(e.Path)
	var previous []ScenarioChange
	if data, err := s.Files.ReadFile(path); err == nil {
		previous = lastApproval(string(data))
	}
	delta := Delta(previous, doc)
	var b strings.Builder
	fmt.Fprintf(&b, "### %s · %s · %s:%s\n\n", at.UTC().Format(time.RFC3339), by, spec.SealV1, hash)
	for _, c := range delta {
		if c.Change == Removed {
			fmt.Fprintf(&b, "- %s · %s\n", c.Change, c.Title)
			continue
		}
		fmt.Fprintf(&b, "- %s · %d · %s · `%s`\n", c.Change, c.Index, c.Title, c.Fingerprint)
	}
	b.WriteString("\n")
	return delta, s.Files.AppendFile(path, []byte(b.String()))
}
