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
