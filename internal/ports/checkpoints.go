package ports

import (
	"context"
	"time"
)

// Checkpoint is a saved state of a project's working tree.
type Checkpoint struct {
	ID    string    `json:"id"`
	Label string    `json:"label"`
	At    time.Time `json:"at"`
}

// Checkpoints keep a project's work recoverable whatever an agent runs: a
// snapshot of the working tree (uncommitted and untracked files included,
// ignored ones not) kept outside the branch, the index and the working
// tree, so taking one changes nothing the developer sees.
type Checkpoints interface {
	// Save records the working tree under label. Outside a repository it
	// returns ErrNotARepository; when nothing changed since the last
	// checkpoint, that one.
	Save(ctx context.Context, root, label string) (Checkpoint, error)
	// List returns the checkpoints, newest first.
	List(ctx context.Context, root string) ([]Checkpoint, error)
	// Restore brings back the files of checkpoint id as they were: it
	// rewrites what changed and recreates what was deleted, and never
	// deletes a file created since.
	Restore(ctx context.Context, root, id string) error
}
