package ports

import (
	"context"

	"specforge/internal/domain/quality"
	"specforge/internal/domain/stack"
)

// Gate is one REFACTOR quality check. Check returns a Skipped result, not
// an error, when its tool is unavailable; an error means the context was
// cancelled.
type Gate interface {
	Name() string
	Applies(p stack.Profile) bool
	Check(ctx context.Context, root string, p stack.Profile) (quality.Result, error)
}

// Readiness says whether a tool could run here, found without running the
// check itself: a path lookup, a configuration file, a version.
type Readiness struct {
	Ready bool
	// Detail is what was found (the tool and its version) or what is not.
	Detail string
	// Hint is how to fix it when it is not ready.
	Hint string
}

// Readier tells cheaply whether its tool is installed for a project.
type Readier interface {
	Ready(ctx context.Context, root string, p stack.Profile) Readiness
}

// Probe is a named Readier, such as a quality gate, for `doctor`.
type Probe interface {
	Name() string
	Applies(p stack.Profile) bool
	Readier
}
