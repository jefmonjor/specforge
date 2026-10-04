// Package process implements ports.CommandRunner on top of os/exec.
package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"time"

	"specforge/internal/ports"
)

// maxCapturedBytes bounds how much of each stream is kept in memory. When a
// process writes more, the head is dropped and the tail is kept, because test
// runners and compilers print the verdict last.
const maxCapturedBytes = 16 << 20

// waitDelay is how long Run waits for I/O to drain after the process is
// killed, so a child that leaks its pipes cannot hang SpecForge.
const waitDelay = 5 * time.Second

// Runner is the os/exec backed ports.CommandRunner.
type Runner struct {
	log *slog.Logger
}

var _ ports.CommandRunner = (*Runner)(nil)

// NewRunner returns a Runner that logs every invocation at debug level.
func NewRunner(log *slog.Logger) *Runner {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Runner{log: log}
}

// Run executes cmd and waits for it to finish. See ports.CommandRunner.
func (r *Runner) Run(ctx context.Context, cmd ports.Command) (ports.CommandResult, error) {
	path, err := Resolve(cmd.Name)
	if err != nil {
		return ports.CommandResult{}, err
	}

	runCtx := ctx
	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}

	stdout := newTailBuffer(maxCapturedBytes)
	stderr := newTailBuffer(maxCapturedBytes)

	c := exec.CommandContext(runCtx, path, cmd.Args...)
	c.Dir = cmd.Dir
	c.Stdin = cmd.Stdin
	c.WaitDelay = waitDelay
	if len(cmd.Env) > 0 {
		c.Env = append(os.Environ(), cmd.Env...)
	}
	if cmd.Stream != nil {
		c.Stdout = io.MultiWriter(stdout, cmd.Stream)
		c.Stderr = io.MultiWriter(stderr, cmd.Stream)
	} else {
		c.Stdout = stdout
		c.Stderr = stderr
	}

	start := time.Now()
	runErr := c.Run()
	result := ports.CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}

	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		result.ExitCode = 0
	case runCtx.Err() != nil:
		result.ExitCode = -1
		if ctx.Err() == nil && errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			runErr = fmt.Errorf("%w after %s: %s", ports.ErrTimeout, cmd.Timeout, cmd)
		} else {
			runErr = fmt.Errorf("%s: %w", cmd, ctx.Err())
		}
	case errors.As(runErr, &exitErr):
		result.ExitCode = exitErr.ExitCode()
		runErr = nil
	default:
		result.ExitCode = -1
		runErr = fmt.Errorf("running %s: %w", cmd, runErr)
	}

	r.log.Debug("process finished",
		"cmd", cmd.String(), "dir", cmd.Dir, "exit", result.ExitCode,
		"elapsed", time.Since(start).Round(time.Millisecond), "err", runErr)
	return result, runErr
}

// Resolve finds the executable for name on PATH. On Windows it also tries the
// .cmd, .bat and .exe shims that npm, npx and mvn install.
func Resolve(name string) (string, error) {
	candidates := []string{name}
	if runtime.GOOS == "windows" {
		candidates = append([]string{name + ".cmd", name + ".bat", name + ".exe"}, candidates...)
	}
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ports.ErrToolNotFound, name)
}

// Available reports whether name resolves to an executable.
func Available(name string) bool {
	_, err := Resolve(name)
	return err == nil
}
