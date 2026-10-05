package agent

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/jefmonjor/specforge/v6/internal/adapters/logging"
	"github.com/jefmonjor/specforge/v6/internal/ports"
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
	claude := Claude.Headless(ports.AgentRequest{Model: "opus"})
	if !slices.Contains(claude, "acceptEdits") || !slices.Contains(claude, "--model") || !slices.Contains(claude, "opus") || slices.Contains(claude, "--add-dir") {
		t.Errorf("claude args = %v", claude)
	}
	gemini := Gemini.Headless(ports.AgentRequest{})
	if !slices.Contains(gemini, "auto_edit") || slices.Contains(gemini, "-m") || slices.Contains(gemini, "--include-directories") {
		t.Errorf("gemini args = %v", gemini)
	}
}

func TestHeadlessCommandsOnlyWhenAsked(t *testing.T) {
	if claude := Claude.Headless(ports.AgentRequest{}); slices.Contains(claude, "--allowedTools") {
		t.Errorf("claude pre-approves tools by default: %v", claude)
	}
	if claude := Claude.Headless(ports.AgentRequest{Commands: true}); !slices.Contains(claude, "Bash") || !slices.Contains(claude, "acceptEdits") {
		t.Errorf("claude commands args = %v", claude)
	}
	if gemini := Gemini.Headless(ports.AgentRequest{Commands: true}); !slices.Contains(gemini, "yolo") {
		t.Errorf("gemini commands args = %v", gemini)
	}
}

func TestHeadlessReadDirs(t *testing.T) {
	claude := Claude.Headless(ports.AgentRequest{ReadDirs: []string{"/legacy/a", "/legacy/b"}})
	if got := strings.Join(claude[len(claude)-4:], " "); got != "--add-dir /legacy/a --add-dir /legacy/b" {
		t.Errorf("claude args = %v", claude)
	}
	gemini := Gemini.Headless(ports.AgentRequest{ReadDirs: []string{"/legacy/a", "/legacy/b"}})
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
