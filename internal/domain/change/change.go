// Package change measures what a piece of work changed: which files, and
// how many lines each one gained and lost, as version control reports it.
// Risk, review budgets and delivery slices are all computed from it, never
// from what the agent says it did.
package change

import (
	"bytes"
	"path"
	"strings"
)

// File is one changed file. Binary files carry no line counts.
type File struct {
	Path    string `json:"path"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Binary  bool   `json:"binary,omitempty"`
}

// Lines is how many lines the file changed.
func (f File) Lines() int { return f.Added + f.Deleted }

// Total adds up the lines of files.
func Total(files []File) int {
	n := 0
	for _, f := range files {
		n += f.Lines()
	}
	return n
}

// generated are files that tools write: they change with the work but
// nobody authored them, so they count neither for risk size nor for the
// delivery budget.
var generated = []string{
	"go.sum", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "npm-shrinkwrap.json",
	"poetry.lock", "Pipfile.lock", "uv.lock", "Cargo.lock", "gradle.lockfile", "composer.lock",
}

// generatedDirs are directories of vendored or recorded output.
var generatedDirs = []string{"vendor/", "node_modules/", "testdata/golden/"}

// Generated reports whether p is a lock file or vendored or golden output.
func Generated(p string) bool {
	p = strings.TrimPrefix(path.Clean(strings.ReplaceAll(p, "\\", "/")), "./")
	base := path.Base(p)
	for _, g := range generated {
		if base == g {
			return true
		}
	}
	if strings.HasSuffix(base, ".lock") {
		return true
	}
	for _, d := range generatedDirs {
		if strings.HasPrefix(p, d) || strings.Contains(p, "/"+d) {
			return true
		}
	}
	return false
}

// Authored drops generated files.
func Authored(files []File) []File {
	var out []File
	for _, f := range files {
		if !Generated(f.Path) {
			out = append(out, f)
		}
	}
	return out
}

// binarySniff is how much of a file is read to tell text from binary, the
// way git does: a NUL byte means binary.
const binarySniff = 8000

// FromContent measures a new file from its content: every line is added.
func FromContent(p string, data []byte) File {
	f := File{Path: p}
	if IsBinary(data) {
		f.Binary = true
		return f
	}
	f.Added = bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		f.Added++
	}
	return f
}

// maxCells bounds the line-diff table; beyond it the count is the
// conservative upper bound (every line changed).
const maxCells = 4_000_000

// Between counts the lines that differ between two versions of a file, as
// a line diff would: the lines of old not kept are deleted, the lines of
// new not kept are added.
func Between(p string, old, new []byte) File {
	a, b := Lines(old), Lines(new)
	// The lines both share at the start and the end are kept: only the
	// middle needs the table, which keeps a small edit to a large file cheap.
	for len(a) > 0 && len(b) > 0 && a[0] == b[0] {
		a, b = a[1:], b[1:]
	}
	for len(a) > 0 && len(b) > 0 && a[len(a)-1] == b[len(b)-1] {
		a, b = a[:len(a)-1], b[:len(b)-1]
	}
	if len(a)*len(b) > maxCells {
		return File{Path: p, Added: len(b), Deleted: len(a)}
	}
	kept := lcs(a, b)
	return File{Path: p, Added: len(b) - kept, Deleted: len(a) - kept}
}

// IsBinary reports whether data looks binary, as git decides it: a NUL
// byte near the start.
func IsBinary(data []byte) bool {
	return bytes.IndexByte(data[:min(len(data), binarySniff)], 0) >= 0
}

// Lines splits text into its lines; nil for empty text.
func Lines(data []byte) []string {
	s := strings.TrimSuffix(string(data), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// lcs is the length of the longest common subsequence of a and b.
func lcs(a, b []string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
			} else {
				cur[j] = max(prev[j], cur[j-1])
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
