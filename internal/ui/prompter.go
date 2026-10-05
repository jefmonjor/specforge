package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// Prompter asks questions on the terminal.
type Prompter struct {
	In          io.Reader
	Err         io.Writer
	Lang        string
	Interactive bool

	once   sync.Once
	reader *bufio.Reader
}

var _ ports.Prompter = (*Prompter)(nil)

// Ask implements ports.Prompter. With options, typing a number returns that
// option's text; free text is accepted too. An empty answer asks again.
func (p *Prompter) Ask(ctx context.Context, q ports.Question) (string, error) {
	if !p.Interactive {
		return "", ports.ErrNonInteractive
	}
	p.once.Do(func() { p.reader = bufio.NewReader(p.In) })

	fmt.Fprintf(p.Err, "\n  ? %s\n", indent(q.Text))
	if q.Context != "" {
		fmt.Fprintf(p.Err, "    %s\n", indent(q.Context))
	}
	for i, o := range q.Options {
		fmt.Fprintf(p.Err, "    %d) %s\n", i+1, o)
	}

	for {
		fmt.Fprint(p.Err, "  > ")
		line, err := p.readLine(ctx)
		if err != nil {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(q.Options) {
			return q.Options[n-1], nil
		}
		if q.Strict && !matchesOption(line, q.Options) {
			fmt.Fprintf(p.Err, "    %s\n", T(p.Lang, "ask.choose", len(q.Options)))
			continue
		}
		return line, nil
	}
}

// readLine reads a line without ignoring cancellation (Ctrl-C).
func (p *Prompter) readLine(ctx context.Context) (string, error) {
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := p.reader.ReadString('\n')
		if err == io.EOF && line != "" {
			err = nil
		}
		ch <- result{line, err}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		if errors.Is(r.err, io.EOF) {
			return "", ports.ErrNonInteractive
		}
		return r.line, r.err
	}
}

func indent(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), "\n", "\n    ") }

func matchesOption(answer string, options []string) bool {
	for _, o := range options {
		if strings.EqualFold(strings.TrimSpace(answer), o) {
			return true
		}
	}
	return false
}
