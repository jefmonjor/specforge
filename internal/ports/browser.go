package ports

import (
	"context"

	"specforge/internal/domain/e2e"
)

// Browser drives a real browser for E2E verification. Every call is bounded
// by its context and by a per-action timeout.
type Browser interface {
	Navigate(ctx context.Context, url string) error
	Snapshot(ctx context.Context) (e2e.Snapshot, error)
	Do(ctx context.Context, a e2e.Action) error
	Screenshot(ctx context.Context, path string) error
	Close() error
}
