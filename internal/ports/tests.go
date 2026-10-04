package ports

import (
	"context"
	"time"

	"specforge/internal/domain/stack"
	"specforge/internal/domain/tdd"
)

// TestRequest selects which tests to run.
type TestRequest struct {
	Root    string
	Profile stack.Profile
	// Filter is a scenario marker such as SDD_0001_003. Only tests whose
	// name contains it run; empty runs the whole suite.
	Filter  string
	Timeout time.Duration
}

// TestRunner runs a project's tests and reports a parsed outcome. A failing
// test is an Outcome, not an error: Run returns an error only when the
// runner could not execute (ErrToolNotFound, ErrTimeout, cancellation).
type TestRunner interface {
	Run(ctx context.Context, req TestRequest) (tdd.Outcome, error)
}
