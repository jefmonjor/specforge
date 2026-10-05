// Package review judges what a code review claims against what really
// changed. A finding is kept only when its proof points at a line the
// change added or modified; it blocks only when it is severe, caused by the
// change and backed by evidence. Everything here is pure: the diff comes
// in as text, the findings as values.
package review

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/jefmonjor/specforge/v6/internal/domain/change"
)

// Hunk is a run of lines the change added or modified, on the new side of
// the diff. A pure deletion is a hunk of no lines at the point where the
// lines were removed (Start == End).
type Hunk struct {
	Path       string `json:"path"`
	Start, End int    `json:"-"`
}

// Contains reports whether line of path is inside the hunk. A deletion
// point accepts the line before and after it.
func (h Hunk) Contains(path string, line int) bool {
	if h.Path != path {
		return false
	}
	if h.Start == h.End {
		return line >= h.Start-1 && line <= h.End+1
	}
	return line >= h.Start && line < h.End
}

// Diff is a parsed unified diff.
type Diff struct {
	Hunks []Hunk
	// Files are the paths the diff changes; New are those it creates.
	Files []string
	New   []string
}

// Changed reports whether line of path is in a hunk.
func (d Diff) Changed(path string, line int) bool {
	return slices.ContainsFunc(d.Hunks, func(h Hunk) bool { return h.Contains(path, line) })
}

// IsNew reports whether the diff creates path.
func (d Diff) IsNew(path string) bool { return slices.Contains(d.New, path) }

var (
	fileHeader = regexp.MustCompile(`^\+\+\+ (?:b/)?(.+?)\s*$`)
	hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)
)

// ParseDiff reads a unified diff with any amount of context (git diff,
// git diff -U0). Binary files carry no hunks. Renames are read by their
// new path.
func ParseDiff(unified string) (Diff, error) {
	var d Diff
	path, newFile := "", false
	line := 0   // next line number on the new side
	start := -1 // first line of the current run of added lines
	removed := false
	inHunk := false
	flush := func() {
		switch {
		case start >= 0:
			d.Hunks = append(d.Hunks, Hunk{Path: path, Start: start, End: line})
		case removed:
			d.Hunks = append(d.Hunks, Hunk{Path: path, Start: line, End: line})
		}
		start, removed = -1, false
	}
	for _, raw := range strings.Split(strings.ReplaceAll(unified, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(raw, "diff --git "):
			flush()
			path, newFile, inHunk = "", false, false
		case strings.HasPrefix(raw, "new file mode"):
			newFile = true
		case strings.HasPrefix(raw, "--- ") && !inHunk:
			if strings.TrimSpace(raw) == "--- /dev/null" {
				newFile = true
			}
		case strings.HasPrefix(raw, "+++ ") && !inHunk:
			m := fileHeader.FindStringSubmatch(raw)
			if m == nil || m[1] == "/dev/null" {
				path = ""
				continue
			}
			path = m[1]
			d.Files = append(d.Files, path)
			if newFile {
				d.New = append(d.New, path)
			}
		case strings.HasPrefix(raw, "@@"):
			flush()
			m := hunkHeader.FindStringSubmatch(raw)
			if m == nil {
				return Diff{}, fmt.Errorf("malformed hunk header %q", raw)
			}
			line, _ = strconv.Atoi(m[3])
			if m[4] == "0" {
				line++ // a deletion "+N,0" sits after line N
			}
			inHunk = path != ""
		case !inHunk:
		case strings.HasPrefix(raw, "+"):
			if start < 0 {
				start = line
			}
			line++
		case strings.HasPrefix(raw, "-"):
			if start >= 0 {
				flush()
			}
			removed = true
		case strings.HasPrefix(raw, `\`):
			// "\ No newline at end of file"
		default:
			flush()
			line++
		}
	}
	flush()
	return d, nil
}

// NewFileDiff renders a file the change creates as a unified diff, for
// files version control does not track yet. Binary content gets git's
// binary line and no hunk.
func NewFileDiff(path string, content []byte) string {
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\nnew file mode 100644\n--- /dev/null\n+++ b/%s\n", path, path, path)
	if change.IsBinary(content) {
		fmt.Fprintf(&b, "Binary files /dev/null and b/%s differ\n", path)
		return b.String()
	}
	lines := change.Lines(content)
	if len(lines) == 0 {
		return b.String()
	}
	fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", len(lines))
	for _, l := range lines {
		b.WriteString("+" + l + "\n")
	}
	return b.String()
}

// Changes measures the diff per file: the lines each one adds or
// modifies (pure deletions count as one line), for classifying its risk.
func (d Diff) Changes() []change.File {
	lines := map[string]int{}
	for _, h := range d.Hunks {
		lines[h.Path] += max(h.End-h.Start, 1)
	}
	out := make([]change.File, 0, len(d.Files))
	for _, f := range d.Files {
		out = append(out, change.File{Path: f, Added: lines[f]})
	}
	return out
}
