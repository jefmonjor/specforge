// Package restore brings back a project's working tree from a checkpoint
// the loop saved before an agent turn. What an agent destroyed, by any
// means, comes back; what was written since is kept. The state just before
// the restore is saved first, so a restore can be undone the same way.
package restore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// Latest names the newest checkpoint.
const Latest = "latest"

// ErrNone means the project has no checkpoint yet.
var ErrNone = errors.New("no checkpoint yet: the loop saves one before every agent turn")

// Result is what a restore did.
type Result struct {
	// Restored is the checkpoint brought back.
	Restored ports.Checkpoint `json:"restored"`
	// Before is the checkpoint of the working tree just before, to undo it.
	Before ports.Checkpoint `json:"before"`
}

// Run restores checkpoint id (or Latest) in root.
func Run(ctx context.Context, cps ports.Checkpoints, root, id string) (Result, error) {
	list, err := cps.List(ctx, root)
	if err != nil {
		return Result{}, err
	}
	if len(list) == 0 {
		return Result{}, ErrNone
	}
	target, ok := find(list, id)
	if !ok {
		return Result{}, fmt.Errorf("no checkpoint %q: run `specforge restore` to list them", id)
	}
	before, err := cps.Save(ctx, root, "before restoring "+target.ID)
	if err != nil {
		return Result{}, fmt.Errorf("saving the working tree before restoring: %w", err)
	}
	if err := cps.Restore(ctx, root, target.ID); err != nil {
		return Result{}, err
	}
	return Result{Restored: target, Before: before}, nil
}

func find(list []ports.Checkpoint, id string) (ports.Checkpoint, bool) {
	if id == Latest {
		return list[0], true
	}
	for _, c := range list {
		if c.ID == id {
			return c, true
		}
	}
	return ports.Checkpoint{}, false
}
