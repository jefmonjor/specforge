// Package ui is the only package that talks to the terminal. Status and
// progress go to stderr; data a script may capture goes to stdout.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Console writes human-readable status.
type Console struct {
	Out   io.Writer // data
	Err   io.Writer // status and progress
	Lang  string
	Quiet bool
}

// IsTerminal reports whether w is an interactive terminal.
func IsTerminal(w any) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// T returns the localized text for key.
func (c *Console) T(key string, args ...any) string { return T(c.Lang, key, args...) }

// Title prints a section title.
func (c *Console) Title(text string) {
	if !c.Quiet {
		fmt.Fprintf(c.Err, "\n%s\n%s\n", text, strings.Repeat("─", min(len([]rune(text)), 72)))
	}
}

// Info prints a neutral line.
func (c *Console) Info(text string) {
	if !c.Quiet {
		fmt.Fprintln(c.Err, "  "+text)
	}
}

// OK prints a success line.
func (c *Console) OK(text string) {
	if !c.Quiet {
		fmt.Fprintln(c.Err, "  ✓ "+text)
	}
}

// Warn prints a warning; never silenced.
func (c *Console) Warn(text string) { fmt.Fprintln(c.Err, "  ⚠ "+text) }

// Bad prints a failure line; never silenced.
func (c *Console) Bad(text string) { fmt.Fprintln(c.Err, "  ✗ "+text) }

// Detail prints indented, possibly multi-line text.
func (c *Console) Detail(text string) {
	if c.Quiet || strings.TrimSpace(text) == "" {
		return
	}
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		fmt.Fprintln(c.Err, "      "+line)
	}
}

// Data prints to stdout, for output meant to be captured.
func (c *Console) Data(text string) { fmt.Fprintln(c.Out, text) }
