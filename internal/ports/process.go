package ports

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"
)

// ErrToolNotFound reports that the executable of a Command is not installed
// or not on PATH. Callers use it to tell "the tool could not run" apart from
// "the tool ran and failed", which is the difference between a skipped gate
// and a failed one.
var ErrToolNotFound = errors.New("tool not found")

// ErrTimeout reports that a Command exceeded its own Timeout.
var ErrTimeout = errors.New("command timed out")

// Command describes one external process invocation.
type Command struct {
	// Name is the executable, resolved through PATH.
	Name string
	Args []string
	// Dir is the working directory; empty means the current one.
	Dir string
	// Env holds extra KEY=VALUE pairs appended to the inherited environment.
	Env []string
	// Stdin feeds the process; nil means no input.
	Stdin io.Reader
	// Timeout bounds this invocation on top of the context; zero means none.
	Timeout time.Duration
	// Stream, when set, receives stdout and stderr as they are produced.
	Stream io.Writer
}

// String renders the command line for logs and error messages.
func (c Command) String() string {
	return strings.TrimSpace(c.Name + " " + strings.Join(c.Args, " "))
}

// CommandResult is the outcome of a process that ran to completion, whatever
// its exit code.
type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Success reports whether the process exited with code zero.
func (r CommandResult) Success() bool { return r.ExitCode == 0 }

// Combined returns stdout followed by stderr, trimmed.
func (r CommandResult) Combined() string {
	out := strings.TrimSpace(r.Stdout)
	errOut := strings.TrimSpace(r.Stderr)
	switch {
	case out == "":
		return errOut
	case errOut == "":
		return out
	default:
		return out + "\n" + errOut
	}
}

// CommandRunner runs external processes.
//
// A non-zero exit code is not an error: it is reported in CommandResult.
// Run returns an error only when the process could not run to completion:
// ErrToolNotFound when the executable is missing, ErrTimeout when
// Command.Timeout elapsed, or the context error when ctx was cancelled.
type CommandRunner interface {
	Run(ctx context.Context, cmd Command) (CommandResult, error)
}
