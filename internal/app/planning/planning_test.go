package planning

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/process"
	"specforge/internal/adapters/workspace"
	"specforge/internal/app/clarify"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

const specText = "# Reset\n\n```gherkin\nFeature: Reset\n  Scenario: Link\n    When asked\n    Then sent\n  Scenario: Expired\n    When late\n    Then refused\n```\n"

type agent struct {
	root    string
	writes  []map[string]string
	replies []string
	prompts []string
}

func (a *agent) Name() string { return "fake" }
func (a *agent) Run(_ context.Context, r ports.AgentRequest) (string, error) {
	a.prompts = append(a.prompts, r.Prompt)
	i := len(a.prompts) - 1
	if i >= len(a.replies) {
		return "", errors.New("unexpected call")
	}
	if i < len(a.writes) {
		for rel, c := range a.writes[i] {
			p := filepath.Join(a.root, rel)
			os.MkdirAll(filepath.Dir(p), 0o755)
			os.WriteFile(p, []byte(c), 0o644)
		}
	}
	return a.replies[i], nil
}
func (a *agent) Interactive(context.Context, ports.AgentRequest) error { return nil }

type events struct{ rejected []string }

func (e *events) Working()                {}
func (e *events) Rejected(r string)       { e.rejected = append(e.rejected, r) }
func (e *events) Answered(string, string) {}

const done = "```json\n{\"status\":\"done\",\"files_written\":[\"specs/0001-reset/plan.md\"]}\n```"
const planOK = "## Tests per scenario\n| SDD_0001_001 | a_test.go |\n| SDD_0001_002 | a_test.go |\n"

func draft(t *testing.T, a *agent) (string, *events, error) {
	t.Helper()
	root := t.TempDir()
	a.root = root
	doc, err := spec.Parse(specText, spec.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ev := &events{}
	path, err := Draft(context.Background(), Deps{
		Agent:     a,
		Workspace: workspace.New(process.NewRunner(nil)),
		Files:     fsys.OS{},
		Asker:     &clarify.Asker{Prompter: nopPrompter{}, Files: fsys.OS{}, Lang: "en", Now: time.Now},
		Lister:    func(context.Context, string) ([]string, error) { return []string{"go.mod", "specs/0001-reset.md"}, nil },
		Events:    ev,
	}, Options{Root: root, SpecPath: filepath.Join(root, "specs", "0001-reset.md"), SpecID: "0001", Doc: doc, SpecText: specText, Language: "en", MaxAttempts: 2})
	return path, ev, err
}

type nopPrompter struct{}

func (nopPrompter) Ask(context.Context, ports.Question) (string, error) {
	return "", ports.ErrNonInteractive
}

func TestDraftWritesAPlanWithFrontMatter(t *testing.T) {
	a := &agent{writes: []map[string]string{{"specs/0001-reset/plan.md": planOK}}, replies: []string{done}}
	path, _, err := draft(t, a)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), "---\nspec: \"0001\"\nstatus: \"draft\"") || !strings.Contains(string(data), "SDD_0001_002") {
		t.Fatalf("plan:\n%s", data)
	}
	if strings.Contains(a.prompts[0], "specs/0001-reset.md\n") || !strings.Contains(a.prompts[0], "go.mod") {
		t.Fatalf("the file list must leave specs/ out:\n%s", a.prompts[0])
	}
}

func TestDraftRetriesAnIncompletePlanThenFails(t *testing.T) {
	a := &agent{
		writes:  []map[string]string{{"specs/0001-reset/plan.md": "| SDD_0001_001 |\n"}, {"specs/0001-reset/plan.md": "| SDD_0001_001 |\nTODO\n"}},
		replies: []string{done, done},
	}
	_, ev, err := draft(t, a)
	var inc *IncompleteError
	if !errors.As(err, &inc) || len(ev.rejected) != 1 {
		t.Fatalf("err=%v rejected=%v", err, ev.rejected)
	}
	if !strings.Contains(a.prompts[1], "no planned test for SDD_0001_002") || !strings.Contains(a.prompts[1], "## Current draft") {
		t.Fatalf("retry prompt:\n%s", a.prompts[1])
	}
}

func TestDraftRefusesWorkOutsideThePlan(t *testing.T) {
	a := &agent{writes: []map[string]string{{"specs/0001-reset/plan.md": planOK, "main.go": "package main\n"}}, replies: []string{done}}
	_, _, err := draft(t, a)
	var scope *ScopeError
	if !errors.As(err, &scope) || scope.Files[0] != "main.go" {
		t.Fatalf("err=%v", err)
	}
}

func TestDraftBlockedAndPending(t *testing.T) {
	_, _, err := draft(t, &agent{replies: []string{"```json\n{\"status\":\"blocked\",\"reason\":\"no idea\"}\n```"}})
	var blocked *tdd.AgentBlockedError
	if !errors.As(err, &blocked) || blocked.Reason != "no idea" {
		t.Fatalf("err=%v", err)
	}
	_, _, err = draft(t, &agent{replies: []string{"```json\n{\"status\":\"needs_clarification\",\"question\":\"REST or gRPC?\"}\n```"}})
	var pending *clarify.PendingQuestionError
	if !errors.As(err, &pending) {
		t.Fatalf("err=%v", err)
	}
}
