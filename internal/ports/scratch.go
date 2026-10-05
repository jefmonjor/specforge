package ports

import (
	"context"
	"errors"
)

// ErrTooLarge reports a project too large to copy for the verifier.
var ErrTooLarge = errors.New("the project is too large to copy")

// Copies are disposable copies of a project.
type Copies struct {
	// Dir is the copy of the working tree; BaseDir the copy at the base
	// commit ("" outside git or without commits).
	Dir, BaseDir string
	// Remove deletes both copies.
	Remove func() error
}

// Scratch makes disposable copies of a project, so an agent can build,
// run and probe it without touching the real one.
type Scratch interface {
	// Copy copies root's files (tracked and untracked, not ignored) and,
	// when base is not empty, the project at commit base. Dependency
	// directories are linked, not copied.
	Copy(ctx context.Context, root, base string) (Copies, error)
}
