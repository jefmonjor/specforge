// Package agent drives coding agent CLIs (Claude Code, Gemini CLI).
//
// Both agents are the same adapter configured by a small table: what
// changes between them is the binary and its flags, nothing else.
package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"specforge/internal/adapters/logging"
	"specforge/internal/adapters/process"
	"specforge/internal/ports"
)

// stdinInstruction is the positional prompt; the real prompt arrives on
// stdin, which has no size limit and never shows up in `ps`.
const stdinInstruction = "Follow the instructions provided on standard input."

// Flavor is the per-agent part of the adapter.
type Flavor struct {
	Name   string
	Binary string
	// Headless returns the arguments for a non-interactive run that reads
	// the prompt from stdin and may edit files without asking. dirs are
	// extra directories the agent may read, such as a legacy repository.
	Headless func(model string, dirs []string) []string
	// Interactive returns the arguments for a terminal session seeded with
	// seed.
	Interactive func(seed, model string) []string
}

// Claude is Claude Code. acceptEdits lets the headless agent write files;
// shell commands still need approval, which never comes in -p mode, so the
// agent cannot run arbitrary commands: SpecForge runs the tests itself.
var Claude = Flavor{
	Name:   "claude",
	Binary: "claude",
	Headless: func(model string, dirs []string) []string {
		args := []string{"-p", stdinInstruction, "--permission-mode", "acceptEdits", "--no-session-persistence"}
		if model != "" {
			args = append(args, "--model", model)
		}
		for _, d := range dirs {
			args = append(args, "--add-dir", d)
		}
		return args
	},
	Interactive: func(seed, model string) []string {
		args := []string{}
		if model != "" {
			args = append(args, "--model", model)
		}
		return append(args, seed)
	},
}

// Gemini is Gemini CLI. auto_edit lets the headless agent write files.
var Gemini = Flavor{
	Name:   "gemini",
	Binary: "gemini",
	Headless: func(model string, dirs []string) []string {
		args := []string{"-p", stdinInstruction, "--approval-mode", "auto_edit"}
		if model != "" {
			args = append(args, "-m", model)
		}
		if len(dirs) > 0 {
			args = append(args, "--include-directories", strings.Join(dirs, ","))
		}
		return args
	},
	Interactive: func(seed, model string) []string {
		args := []string{"-i", seed}
		if model != "" {
			args = append(args, "-m", model)
		}
		return args
	},
}

// Flavors lists the supported agents by name.
var Flavors = map[string]Flavor{Claude.Name: Claude, Gemini.Name: Gemini}

// CLI is a ports.Agent backed by an agent's command-line tool.
type CLI struct {
	flavor Flavor
	proc   ports.CommandRunner
	log    *slog.Logger
	// activity shows progress while the agent works; it returns a stop func.
	activity func(label string) func()
	// stream, when set, receives the agent output live (--verbose).
	stream *os.File
}

var _ ports.Agent = (*CLI)(nil)

// Option configures a CLI.
type Option func(*CLI)

// WithActivity installs a progress indicator.
func WithActivity(f func(label string) func()) Option {
	return func(c *CLI) { c.activity = f }
}

// WithStream mirrors the agent output to f as it is produced.
func WithStream(f *os.File) Option {
	return func(c *CLI) { c.stream = f }
}

// New returns the agent adapter for flavor.
func New(flavor Flavor, proc ports.CommandRunner, log *slog.Logger, opts ...Option) *CLI {
	c := &CLI{flavor: flavor, proc: proc, log: log, activity: func(string) func() { return func() {} }}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Name implements ports.Agent.
func (c *CLI) Name() string { return c.flavor.Name }

// Run implements ports.Agent.
func (c *CLI) Run(ctx context.Context, req ports.AgentRequest) (string, error) {
	logging.Trace(ctx, c.log, "agent prompt", req.Prompt)

	cmd := ports.Command{
		Name:    c.flavor.Binary,
		Args:    c.flavor.Headless(req.Model, req.ReadDirs),
		Dir:     req.Dir,
		Env:     req.Env,
		Stdin:   strings.NewReader(req.Prompt),
		Timeout: req.Timeout,
	}
	if c.stream != nil {
		cmd.Stream = c.stream
	}

	stop := c.activity(c.flavor.Name)
	res, err := c.proc.Run(ctx, cmd)
	stop()

	logging.Trace(ctx, c.log, "agent output", res.Stdout)
	if err != nil {
		return "", fmt.Errorf("%s: %w", c.flavor.Name, err)
	}
	if !res.Success() {
		return "", fmt.Errorf("%s exited with code %d: %s", c.flavor.Name, res.ExitCode, firstLines(res.Stderr, 20))
	}
	return res.Stdout, nil
}

// Interactive implements ports.Agent. The prompt goes to a private temp
// file and the agent is told to read it, because a long prompt cannot be
// passed on the command line (128 KiB per argument on Linux, 32 KiB per
// command line on Windows) and would be visible to every user.
func (c *CLI) Interactive(ctx context.Context, req ports.AgentRequest) error {
	path, err := process.Resolve(c.flavor.Binary)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "specforge-prompt-*.md")
	if err != nil {
		return fmt.Errorf("creating prompt file: %w", err)
	}
	defer func() { _ = os.Remove(f.Name()) }() // best effort: a temp file
	if _, err := f.WriteString(req.Prompt); err != nil {
		return errors.Join(fmt.Errorf("writing prompt file: %w", err), f.Close())
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("writing prompt file: %w", err)
	}

	seed := fmt.Sprintf("Read the file %s and follow its instructions exactly.", f.Name())
	cmd := exec.CommandContext(ctx, path, c.flavor.Interactive(seed, req.Model)...)
	cmd.Dir = req.Dir
	if len(req.Env) > 0 {
		cmd.Env = append(os.Environ(), req.Env...)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s interactive session: %w", c.flavor.Name, err)
	}
	return nil
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = append(lines[:n], "…")
	}
	return strings.Join(lines, "\n")
}
