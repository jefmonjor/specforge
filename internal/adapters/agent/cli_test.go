package agent

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"specforge/internal/adapters/logging"
	"specforge/internal/ports"
)

type fakeProc struct {
	got    ports.Command
	stdin  string
	result ports.CommandResult
	err    error
}

func (f *fakeProc) Run(_ context.Context, c ports.Command) (ports.CommandResult, error) {
	f.got = c
	if c.Stdin != nil {
		b, _ := io.ReadAll(c.Stdin)
		f.stdin = string(b)
	}
	return f.result, f.err
}

func TestRunSendsThePromptThroughStdinNeverArgv(t *testing.T) {
	huge := strings.Repeat("x", 300_000) // larger than Linux's 128 KiB per-argument limit
	for _, flavor := range []Flavor{Claude, Gemini} {
		proc := &fakeProc{result: ports.CommandResult{Stdout: "ok"}}
		out, err := New(flavor, proc, logging.Discard()).Run(context.Background(), ports.AgentRequest{Prompt: huge, Dir: "/repo"})
		if err != nil || out != "ok" {
			t.Fatalf("%s: Run = %q, %v", flavor.Name, out, err)
		}
		if proc.stdin != huge {
			t.Errorf("%s: prompt not sent on stdin", flavor.Name)
		}
		for _, a := range proc.got.Args {
			if len(a) > 1000 {
				t.Errorf("%s: an argument carries the prompt", flavor.Name)
			}
		}
		if proc.got.Name != flavor.Binary || proc.got.Dir != "/repo" {
			t.Errorf("%s: command = %+v", flavor.Name, proc.got)
		}
	}
}

func TestHeadlessFlagsAllowEditsAndModel(t *testing.T) {
	claude := Claude.Headless("opus", nil)
	if !slices.Contains(claude, "acceptEdits") || !slices.Contains(claude, "--model") || !slices.Contains(claude, "opus") || slices.Contains(claude, "--add-dir") {
		t.Errorf("claude args = %v", claude)
	}
	gemini := Gemini.Headless("", nil)
	if !slices.Contains(gemini, "auto_edit") || slices.Contains(gemini, "-m") || slices.Contains(gemini, "--include-directories") {
		t.Errorf("gemini args = %v", gemini)
	}
}

func TestHeadlessReadDirs(t *testing.T) {
	claude := Claude.Headless("", []string{"/legacy/a", "/legacy/b"})
	if got := strings.Join(claude[len(claude)-4:], " "); got != "--add-dir /legacy/a --add-dir /legacy/b" {
		t.Errorf("claude args = %v", claude)
	}
	gemini := Gemini.Headless("", []string{"/legacy/a", "/legacy/b"})
	if got := strings.Join(gemini[len(gemini)-2:], " "); got != "--include-directories /legacy/a,/legacy/b" {
		t.Errorf("gemini args = %v", gemini)
	}
}

func TestRunReportsNonZeroExit(t *testing.T) {
	proc := &fakeProc{result: ports.CommandResult{ExitCode: 2, Stderr: "quota exceeded"}}
	_, err := New(Claude, proc, logging.Discard()).Run(context.Background(), ports.AgentRequest{})
	if err == nil || !strings.Contains(err.Error(), "quota exceeded") {
		t.Fatalf("want exit error with stderr, got %v", err)
	}
}

func TestRunReportsStdoutWhenStderrIsEmpty(t *testing.T) {
	proc := &fakeProc{result: ports.CommandResult{ExitCode: 1, Stdout: "You've hit your usage limit"}}
	_, err := New(Claude, proc, logging.Discard()).Run(context.Background(), ports.AgentRequest{})
	if err == nil || !strings.Contains(err.Error(), "usage limit") {
		t.Fatalf("the reason on stdout is reported: %v", err)
	}
}

func TestRunPropagatesMissingTool(t *testing.T) {
	proc := &fakeProc{err: ports.ErrToolNotFound}
	_, err := New(Gemini, proc, logging.Discard()).Run(context.Background(), ports.AgentRequest{})
	if !errors.Is(err, ports.ErrToolNotFound) {
		t.Fatalf("want ErrToolNotFound, got %v", err)
	}
}

func TestActivityIsStoppedEvenOnError(t *testing.T) {
	started, stopped := 0, 0
	act := WithActivity(func(string) func() { started++; return func() { stopped++ } })
	proc := &fakeProc{err: errors.New("boom")}
	_, _ = New(Claude, proc, logging.Discard(), act).Run(context.Background(), ports.AgentRequest{})
	if started != 1 || stopped != 1 {
		t.Fatalf("started=%d stopped=%d", started, stopped)
	}
}
