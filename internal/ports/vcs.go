package ports

import (
	"context"
	"errors"
)

// ErrNotARepository reports a project outside git.
var ErrNotARepository = errors.New("not a git repository")

// VCS reads and records the project's version control.
type VCS interface {
	// DefaultBase returns the ref a feature branch is compared with
	// (origin/main, origin/master, main or master, whichever exists).
	DefaultBase(ctx context.Context, root string) (string, error)
	// Diff returns the changes between base and the working tree. base is
	// validated as a commit and can never be read as an option.
	Diff(ctx context.Context, root, base string) (string, error)
	// Files lists tracked and untracked, non-ignored files.
	Files(ctx context.Context, root string) ([]string, error)
	// Commit records exactly paths (additions, changes and deletions) with
	// message, leaving anything else staged untouched, and returns the new
	// commit's hash. It returns "" when paths hold no change.
	Commit(ctx context.Context, root, message string, paths []string) (string, error)
}
