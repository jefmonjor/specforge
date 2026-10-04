// Package cmd is SpecForge's command-line interface and composition root:
// it parses flags, builds the adapters and hands them to the use cases in
// internal/app. Nothing here decides anything a use case should decide.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"specforge/internal/adapters/agent"
	"specforge/internal/adapters/browser"
	"specforge/internal/adapters/logging"
	"specforge/internal/adapters/process"
	"specforge/internal/config"
	"specforge/internal/ports"
	"specforge/internal/ui"
)

// App holds the process streams and the factories of every adapter, so a
// test can run any command against fakes.
type App struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
	// Interactive is true when a person can answer questions (stdin and
	// stderr are terminals).
	Interactive bool
	Getwd       func() (string, error)
	Now         func() time.Time
	UserDirs    func() (config.Dirs, error)

	NewProcess    func(log *slog.Logger) ports.CommandRunner
	NewAgent      func(name string, proc ports.CommandRunner, log *slog.Logger) (ports.Agent, error)
	LaunchBrowser func(ctx context.Context, o browser.Options) (ports.Browser, error)

	flags  globalFlags
	log    *slog.Logger
	closer io.Closer
	lang   string
	// ask is shared: its buffered reader must not lose typed-ahead answers.
	ask *ui.Prompter
}

type globalFlags struct {
	debug, verbose, traceIO, quiet, nonInteractive, json bool
}

// Main runs SpecForge with the process arguments and returns the exit code.
// Ctrl-C cancels the context: every step stops, the state already saved
// stays valid and the command exits with 130.
func Main() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return New().Run(ctx, os.Args[1:])
}

// New returns an App wired to the real process, terminal and adapters.
func New() *App {
	a := &App{
		In:          os.Stdin,
		Out:         os.Stdout,
		Err:         os.Stderr,
		Interactive: ui.IsTerminal(os.Stdin) && ui.IsTerminal(os.Stderr),
		Getwd:       os.Getwd,
		Now:         time.Now,
		UserDirs:    config.UserDirs,
		NewProcess:  func(log *slog.Logger) ports.CommandRunner { return process.NewRunner(log) },
		LaunchBrowser: func(ctx context.Context, o browser.Options) (ports.Browser, error) {
			return browser.Launch(ctx, o)
		},
	}
	a.NewAgent = a.defaultAgent
	return a
}

// Run executes one command line and maps its error to an exit code.
func (a *App) Run(ctx context.Context, args []string) int {
	a.log, a.lang = logging.Discard(), config.DefaultLanguage
	root := a.rootCommand()
	root.SetArgs(args)
	root.SetIn(a.In)
	root.SetOut(a.Out)
	root.SetErr(a.Err)

	err := root.ExecuteContext(ctx)
	defer a.closeLog()
	if err == nil {
		return ui.ExitOK
	}
	d := ui.Diagnose(a.lang, err)
	// Info, not Error: the diagnosis below is the console rendering of it.
	a.log.Info("command failed", "err", err, "exit", d.Code)
	if a.flags.json {
		_ = json.NewEncoder(a.Err).Encode(d)
	} else {
		ui.PrintDiagnosis(a.Err, a.lang, d)
	}
	return d.Code
}

// startLogging opens the log file; a log that cannot be opened is reported
// once and never stops the command.
func (a *App) startLogging() {
	dirs, err := a.UserDirs()
	if err == nil {
		var log *slog.Logger
		log, a.closer, err = logging.New(logging.Options{
			Dir:     dirs.Logs,
			Debug:   a.flags.debug,
			Verbose: a.flags.verbose,
			TraceIO: a.flags.traceIO,
			Console: a.Err,
		})
		if err == nil {
			a.log = log
			return
		}
	}
	fmt.Fprintf(a.Err, "  ⚠ logging disabled: %v\n", err)
}

func (a *App) closeLog() {
	if a.closer != nil {
		_ = a.closer.Close()
		a.closer = nil
	}
}

func (a *App) console() *ui.Console {
	return &ui.Console{Out: a.Out, Err: a.Err, Lang: a.lang, Quiet: a.flags.quiet}
}

func (a *App) prompter() *ui.Prompter {
	if a.ask == nil {
		a.ask = &ui.Prompter{In: a.In, Err: a.Err}
	}
	a.ask.Lang, a.ask.Interactive = a.lang, a.canAsk()
	return a.ask
}

// canAsk is false in CI or with --non-interactive: questions then go to a
// file and the command exits with code 5.
func (a *App) canAsk() bool { return a.Interactive && !a.flags.nonInteractive }

func (a *App) defaultAgent(name string, proc ports.CommandRunner, log *slog.Logger) (ports.Agent, error) {
	flavor, ok := agent.Flavors[name]
	if !ok {
		return nil, fmt.Errorf("unknown agent %q (supported: %v)", name, config.Agents)
	}
	if !process.Available(flavor.Binary) {
		return nil, fmt.Errorf("%w: %s (install it and sign in, or choose another agent with `specforge init`)", ports.ErrToolNotFound, flavor.Binary)
	}
	var opts []agent.Option
	if f, ok := a.Err.(*os.File); ok && a.flags.verbose {
		opts = append(opts, agent.WithStream(f))
	}
	return agent.New(flavor, proc, log, opts...), nil
}

// emit writes v as JSON to stdout when --json is set and reports whether
// it did; commands print their human output otherwise.
func (a *App) emit(v any) (bool, error) {
	if !a.flags.json {
		return false, nil
	}
	enc := json.NewEncoder(a.Out)
	enc.SetIndent("", "  ")
	return true, enc.Encode(v)
}
