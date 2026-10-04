package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"specforge/internal/app/clarify"
	"specforge/internal/app/specs"
	"specforge/internal/config"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

func TestDiagnoseExitCodes(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{errors.New("boom"), ExitError},
		{fmt.Errorf("wrapped: %w", context.Canceled), ExitInterrupted},
		{&clarify.PendingQuestionError{File: "q.md"}, ExitQuestion},
		{spec.ErrNotSealed, ExitSpec},
		{&spec.TamperedError{Expected: "a", Actual: "b"}, ExitSpec},
		{&tdd.TamperingError{Phase: tdd.PhaseGreen, Changed: []string{"x_test.go"}}, ExitSpec},
		{&specs.LintError{Path: "specs/0001-a.md", Issues: []spec.Issue{{Rule: spec.RulePlaceholder, Line: 3, Message: "TODO", Blocking: true}}}, ExitSpec},
		{&specs.AmbiguousError{Candidates: []string{"a", "b"}}, ExitSpec},
		{specs.ErrNoSpecs, ExitSpec},
		{tdd.ErrAttemptsExhausted, ExitGate},
		{&tdd.AgentBlockedError{Reason: "no pytest", SuggestedAction: "install it"}, ExitGate},
		{fmt.Errorf("x: %w", ports.ErrToolNotFound), ExitEnvironment},
		{config.ErrNoAgent, ExitEnvironment},
		{ports.ErrTimeout, ExitEnvironment},
	}
	for _, c := range cases {
		if d := Diagnose("en", c.err); d.Code != c.code {
			t.Errorf("Diagnose(%v) = %d, want %d", c.err, d.Code, c.code)
		}
	}
	d := Diagnose("es", &tdd.AgentBlockedError{Reason: "no pytest", SuggestedAction: "install it"})
	if d.Cause != "no pytest" || d.Action != "install it" {
		t.Fatalf("blocked diagnosis %+v", d)
	}
}

func TestPrintDiagnosis(t *testing.T) {
	var b bytes.Buffer
	PrintDiagnosis(&b, "es", Diagnose("es", spec.ErrNotSealed))
	out := b.String()
	for _, want := range []string{"✗ La especificación no está aprobada", "specforge spec approve", "(exit 3)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestMessagesHaveBothLanguages(t *testing.T) {
	for key := range messages["en"] {
		if _, ok := messages["es"][key]; !ok {
			t.Errorf("key %q has no Spanish text", key)
		}
	}
	for key := range messages["es"] {
		if _, ok := messages["en"][key]; !ok {
			t.Errorf("key %q has no English text", key)
		}
	}
	if T("fr", "init.next") != messages["en"]["init.next"] {
		t.Error("an unknown language must fall back to English")
	}
	if T("en", "no.such.key") != "no.such.key" {
		t.Error("a missing key must print as itself")
	}
}

func TestPrompter(t *testing.T) {
	var errOut bytes.Buffer
	p := &Prompter{In: strings.NewReader("\n2\nfree text\n"), Err: &errOut, Interactive: true}
	q := ports.Question{Text: "Which?", Context: "RED · scenario 1", Options: []string{"email", "sms"}}
	if got, err := p.Ask(context.Background(), q); err != nil || got != "sms" {
		t.Fatalf("numbered answer %q %v", got, err)
	}
	if got, err := p.Ask(context.Background(), q); err != nil || got != "free text" {
		t.Fatalf("free answer %q %v", got, err)
	}
	if _, err := p.Ask(context.Background(), q); !errors.Is(err, ports.ErrNonInteractive) {
		t.Fatalf("EOF must be non-interactive, got %v", err)
	}
	if !strings.Contains(errOut.String(), "1) email") || !strings.Contains(errOut.String(), "RED · scenario 1") {
		t.Fatalf("prompt output:\n%s", errOut.String())
	}
	if _, err := (&Prompter{}).Ask(context.Background(), q); !errors.Is(err, ports.ErrNonInteractive) {
		t.Fatal("a non-interactive prompter must refuse")
	}
}

func TestConsoleQuiet(t *testing.T) {
	var out, errOut bytes.Buffer
	c := &Console{Out: &out, Err: &errOut, Lang: "en", Quiet: true}
	c.Title("t")
	c.Info("i")
	c.OK("ok")
	c.Detail("d")
	c.Warn("w")
	c.Bad("b")
	c.Data("data")
	if errOut.String() != "  ⚠ w\n  ✗ b\n" || out.String() != "data\n" {
		t.Fatalf("quiet console wrote %q / %q", errOut.String(), out.String())
	}
}

func TestStrictQuestionAsksAgain(t *testing.T) {
	var errOut bytes.Buffer
	p := &Prompter{In: strings.NewReader("Accept\n7\nyes: it exists\n"), Err: &errOut, Lang: "en", Interactive: true}
	q := ports.Question{Text: "Already implemented?", Options: []string{"Yes: it exists", "No"}, Strict: true}
	got, err := p.Ask(context.Background(), q)
	if err != nil || got != "yes: it exists" {
		t.Fatalf("got %q %v", got, err)
	}
	if strings.Count(errOut.String(), "type a number from 1 to 2") != 2 {
		t.Fatalf("output:\n%s", errOut.String())
	}
}

// TestEveryUsedKeyExists scans the source for message keys passed to T so
// a key missing from both languages cannot slip through.
func TestEveryUsedKeyExists(t *testing.T) {
	keyCall := regexp.MustCompile(`\.T\("([a-z0-9.-]+)"|T\([a-zA-Z.]+, "([a-z0-9.-]+)"`)
	for _, dir := range []string{".", "../../cmd"} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, _ := os.ReadFile(f)
			for _, m := range keyCall.FindAllStringSubmatch(string(src), -1) {
				key := m[1] + m[2]
				if strings.HasSuffix(key, ".") || strings.HasSuffix(key, "-") {
					continue // a prefix completed at run time
				}
				if _, ok := messages["en"][key]; !ok {
					t.Errorf("%s uses %q, which has no message", f, key)
				}
			}
		}
	}
}
