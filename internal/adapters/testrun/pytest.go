package testrun

import (
	"context"
	"errors"
	"fmt"

	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// pytest exit codes (pytest.ExitCode).
const (
	pytestInterrupted   = 2 // collection errors: a test module failed to import
	pytestInternalError = 3
	pytestUsageError    = 4
	pytestNoTests       = 5
)

func (r *Runner) pytest(ctx context.Context, req ports.TestRequest) (tdd.Outcome, error) {
	launchers := pythonCandidates(req.Root)
	if len(launchers) == 0 {
		return tdd.Outcome{}, fmt.Errorf("%w: pytest (or python3 -m pytest)", ports.ErrToolNotFound)
	}
	report, cleanup, err := tempReport("specforge-pytest-*.xml")
	if err != nil {
		return tdd.Outcome{}, err
	}
	defer cleanup()

	launcher := launchers[0]
	args := append(append([]string{}, launcher[1:]...), "-q", "-p", "no:cacheprovider", "--junitxml="+report)
	if req.Filter != "" {
		args = append(args, "-k", req.Filter)
	}
	res, err := r.run(ctx, req, launcher[0], args...)
	if err != nil {
		return tdd.Outcome{}, err
	}

	switch res.ExitCode {
	case pytestInterrupted:
		return tdd.Outcome{Compiled: false, Exact: true, Output: clip(res.Combined())}, nil
	case pytestNoTests:
		return tdd.Outcome{Compiled: true, Exact: true, Output: clip(res.Combined())}, nil
	case pytestInternalError, pytestUsageError:
		return tdd.Outcome{}, errors.New("pytest could not run: " + clip(res.Combined()))
	}

	passed, failed, skipped, out, ok := junitTotals([]string{report})
	if !ok {
		o := tdd.Outcome{Compiled: true, Exact: false, Output: clip(res.Combined())}
		if !res.Success() {
			o.Failed = 1
		}
		return o, nil
	}
	return tdd.Outcome{Compiled: true, Exact: true, Passed: passed, Failed: failed, Skipped: skipped, Output: clip(out)}, nil
}
