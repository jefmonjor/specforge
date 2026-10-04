// Package logging builds the structured logger SpecForge uses everywhere.
//
// Two sinks: a rotating file under the user's state directory and the
// console (stderr). Each has its own level, so --debug makes the file
// verbose without flooding the terminal. Prompts and agent output are logged
// at LevelTrace and only reach the file when tracing is explicitly enabled,
// because they contain the user's source code.
package logging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// LevelTrace sits below Debug and is reserved for full prompt and response
// bodies.
const LevelTrace = slog.Level(-8)

// Options configures New.
type Options struct {
	// Dir is the directory for the log file. It is created with mode 0700.
	Dir string
	// Debug lowers the file level from Info to Debug.
	Debug bool
	// Verbose lowers the console level from Warn to Info (Debug with Debug).
	Verbose bool
	// TraceIO lowers the file level to LevelTrace, recording prompts and
	// agent output.
	TraceIO bool
	// Console receives console records; nil means os.Stderr.
	Console io.Writer
}

// FileName is the name of the active log file inside Options.Dir.
const FileName = "specforge.log"

// New returns the logger and a Closer for the file sink. If the file sink
// cannot be created, the logger still works on the console and the error is
// returned so the caller can report it.
func New(o Options) (*slog.Logger, io.Closer, error) {
	console := o.Console
	if console == nil {
		console = os.Stderr
	}
	consoleLevel := slog.LevelWarn
	if o.Verbose {
		consoleLevel = slog.LevelInfo
		if o.Debug {
			consoleLevel = slog.LevelDebug
		}
	}
	consoleHandler := slog.NewTextHandler(console, &slog.HandlerOptions{
		Level:       consoleLevel,
		ReplaceAttr: dropTime,
	})

	fileLevel := slog.LevelInfo
	switch {
	case o.TraceIO:
		fileLevel = LevelTrace
	case o.Debug:
		fileLevel = slog.LevelDebug
	}

	if o.Dir == "" {
		return slog.New(consoleHandler), nopCloser{}, nil
	}
	if err := os.MkdirAll(o.Dir, 0o700); err != nil {
		return slog.New(consoleHandler), nopCloser{}, fmt.Errorf("creating log directory %s: %w", o.Dir, err)
	}
	sink := &lumberjack.Logger{
		Filename:   filepath.Join(o.Dir, FileName),
		MaxSize:    10, // megabytes
		MaxBackups: 5,
		MaxAge:     30, // days
	}
	fileHandler := slog.NewTextHandler(sink, &slog.HandlerOptions{
		Level:       fileLevel,
		ReplaceAttr: renameTrace,
	})
	return slog.New(fanout{fileHandler, consoleHandler}), sink, nil
}

// Trace records a full prompt or response body at LevelTrace.
func Trace(ctx context.Context, log *slog.Logger, label, body string) {
	log.Log(ctx, LevelTrace, label, "body", body)
}

// Discard returns a logger that drops every record, for tests.
func Discard() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

type fanout []slog.Handler

func (f fanout) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f fanout) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f {
		if h.Enabled(ctx, r.Level) {
			errs = append(errs, h.Handle(ctx, r.Clone()))
		}
	}
	return errors.Join(errs...)
}

func (f fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithAttrs(attrs)
	}
	return out
}

func (f fanout) WithGroup(name string) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithGroup(name)
	}
	return out
}

func dropTime(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 && a.Key == slog.TimeKey {
		return slog.Attr{}
	}
	return renameTrace(groups, a)
}

func renameTrace(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey {
		if l, ok := a.Value.Any().(slog.Level); ok && l == LevelTrace {
			a.Value = slog.StringValue("TRACE")
		}
	}
	return a
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
