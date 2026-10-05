package ports

import (
	"context"
	"errors"

	"github.com/jefmonjor/specforge/v6/internal/domain/change"
)

// ErrNotARepository reports a project outside git.
var ErrNotARepository = errors.New("not a git repository")

// DiffSource reads a branch's changes, for reviewers of a whole branch
// (the security audit, the review lenses).
type DiffSource interface {
	// DefaultBase returns the ref a feature branch is compared with
	// (origin/main, origin/master, main or master, whichever exists).
	DefaultBase(ctx context.Context, root string) (string, error)
	// Diff returns the changes between base and the working tree. base is
	// validated as a commit and can never be read as an option.
	Diff(ctx context.Context, root, base string) (string, error)
	// Files lists tracked and untracked, non-ignored files.
	Files(ctx context.Context, root string) ([]string, error)
}

// Recorder records work as commits.
type Recorder interface {
	// Commit records exactly paths (additions, changes and deletions) with
	// message, leaving anything else staged untouched, and returns the new
	// commit's hash. It returns "" when paths hold no change.
	Commit(ctx context.Context, root, message string, paths []string) (string, error)
}

// Measurer counts what changed, line by line.
type Measurer interface {
	// Changes measures paths in the working tree against the last commit,
	// untracked files included, with their added and deleted lines.
	Changes(ctx context.Context, root string, paths []string) ([]change.File, error)
	// CommitChanges measures the files a commit changed.
	CommitChanges(ctx context.Context, root, commit string) ([]change.File, error)
	// Patch is the unified diff of paths in the working tree against the
	// last commit, untracked files included as new files.
	Patch(ctx context.Context, root string, paths []string) (string, error)
}

// VCS is the project's version control: every capability above. Use cases
// depend on the narrowest one they need.
type VCS interface {
	DiffSource
	Recorder
	Measurer
}
