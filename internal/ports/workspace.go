package ports

import (
	"context"
	"errors"
)

// Snapshot maps the slash-separated paths of the files that differ from the
// last commit (or every file, outside git) to their SHA-256.
type Snapshot map[string]string

// Changed returns the paths added, removed or modified between two
// snapshots, sorted.
func (before Snapshot) Changed(after Snapshot) []string {
	set := map[string]bool{}
	for p, h := range after {
		if before[p] != h {
			set[p] = true
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			set[p] = true
		}
	}
	return sortedKeys(set)
}

// Workspace observes the project's files so SpecForge can verify what the
// agent really did instead of trusting what it says.
type Workspace interface {
	// Snapshot captures the current state of changed files under root.
	Snapshot(ctx context.Context, root string) (Snapshot, error)
	// HashFiles hashes every file under root whose relative path satisfies
	// match, skipping dependency and build directories.
	HashFiles(root string, match func(rel string) bool) (map[string]string, error)
}

// ErrNonInteractive reports that a question cannot be asked because no
// one is at the terminal (CI, pipes).
var ErrNonInteractive = errors.New("no interactive terminal to ask the developer")

// Question is something only the developer can decide.
type Question struct {
	// Text is the question itself.
	Text string
	// Context explains where it comes from (phase, scenario).
	Context string
	// Options, when set, are numbered choices; free text is still accepted.
	Options []string
}

// Prompter asks the developer a question and returns the answer.
type Prompter interface {
	Ask(ctx context.Context, q Question) (string, error)
}

// Files is the filesystem the application layer reads and writes. Writes
// are atomic: a crash never leaves a half-written file behind.
type Files interface {
	ReadFile(path string) ([]byte, error)
	// WriteFile replaces path atomically, creating parent directories.
	WriteFile(path string, data []byte) error
	// WritePrivate is WriteFile with permissions limited to the owner, for
	// sensitive content such as security findings.
	WritePrivate(path string, data []byte) error
	// AppendFile appends data, creating the file and its parents.
	AppendFile(path string, data []byte) error
	Exists(path string) bool
}
