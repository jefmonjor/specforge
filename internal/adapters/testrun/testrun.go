// Package testrun runs a project's tests for each supported stack and turns
// the runner's machine-readable report into a tdd.Outcome.
//
// The distinction that matters is compiled-but-failing versus not compiling:
// only the former is a valid RED. Exit codes cannot tell them apart; the
// reports can.
package testrun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"specforge/internal/adapters/process"
	"specforge/internal/domain/stack"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// maxOutput bounds the failure text kept for prompts and reports.
const maxOutput = 12_000

// Runner is the ports.TestRunner for every supported stack.
type Runner struct {
	proc ports.CommandRunner
	now  func() time.Time
}

var _ ports.TestRunner = (*Runner)(nil)

// New returns a test runner that executes commands through proc.
func New(proc ports.CommandRunner) *Runner {
	return &Runner{proc: proc, now: time.Now}
}

// Run implements ports.TestRunner.
func (r *Runner) Run(ctx context.Context, req ports.TestRequest) (tdd.Outcome, error) {
	switch req.Profile.Runner {
	case stack.RunnerGo:
		return r.goTest(ctx, req)
	case stack.RunnerMaven:
		return r.maven(ctx, req)
	case stack.RunnerGradle:
		return r.gradle(ctx, req)
	case stack.RunnerVitest:
		return r.jsonJS(ctx, req, []string{"--no-install", "vitest", "run", "--reporter=json"}, "--outputFile=", "-t")
	case stack.RunnerJest:
		return r.jsonJS(ctx, req, []string{"--no-install", "jest", "--ci", "--json"}, "--outputFile=", "-t")
	case stack.RunnerNPM:
		return r.npm(ctx, req)
	case stack.RunnerPytest:
		return r.pytest(ctx, req)
	default:
		return tdd.Outcome{}, fmt.Errorf("%w: %s", tdd.ErrUnsupportedStack, req.Profile.Name())
	}
}

func (r *Runner) run(ctx context.Context, req ports.TestRequest, name string, args ...string) (ports.CommandResult, error) {
	return r.proc.Run(ctx, ports.Command{Name: name, Args: args, Dir: req.Root, Timeout: req.Timeout})
}

// tempReport returns a path for a runner's report file and a cleanup func.
func tempReport(pattern string) (string, func(), error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, fmt.Errorf("creating test report file: %w", err)
	}
	name := f.Name()
	// Only the unique name is needed: the runner creates the file, and an
	// empty one would look like a report.
	if err := errors.Join(f.Close(), os.Remove(name)); err != nil {
		return "", nil, fmt.Errorf("preparing test report file: %w", err)
	}
	return name, func() { _ = os.Remove(name) }, nil
}

// clip keeps the beginning and the end of long output, where compilers and
// test runners put the information that matters.
func clip(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxOutput {
		return s
	}
	head := maxOutput / 5
	tail := maxOutput - head
	return s[:head] + "\n[… output elided …]\n" + s[len(s)-tail:]
}

// newerThan lists files under root matching glob in any directory named
// dirName, modified at or after since. Stale reports from previous runs are
// ignored.
func newerThan(root, dirName, suffix string, since time.Time) []string {
	var out []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(filepath.Dir(p)) != dirName && !strings.Contains(filepath.ToSlash(p), "/"+dirName+"/") {
			return nil
		}
		if !strings.HasSuffix(p, suffix) {
			return nil
		}
		if info, err := d.Info(); err == nil && !info.ModTime().Before(since.Add(-time.Second)) {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// pythonCandidates returns the ways to launch pytest, most direct first.
func pythonCandidates() [][]string {
	var out [][]string
	if process.Available("pytest") {
		out = append(out, []string{"pytest"})
	}
	for _, py := range []string{"python3", "python", "py"} {
		if process.Available(py) {
			out = append(out, []string{py, "-m", "pytest"})
		}
	}
	return out
}
