package interview

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
	"specforge/internal/ports"
)

const draft = "---\nid: \"0001\"\ntitle: \"Reset\"\nstatus: draft\n---\n\n# 0001 · Reset\n\n## 1. Intent\n\nTODO intent\n\n" +
	"## 6. Scenarios\n\n```gherkin\nFeature: Reset\n  Scenario: TODO\n    When TODO\n    Then TODO\n```\n\n## 12. Open questions\n"

const complete = "---\nid: \"0001\"\ntitle: \"Reset\"\nstatus: draft\n---\n\n# 0001 · Reset\n\n## 1. Intent\n\nUsers recover access alone.\n\n" +
	"## 6. Scenarios\n\n```gherkin\nFeature: Reset\n  Scenario: Link\n    When she asks\n    Then a link is emailed\n```\n\n## 12. Open questions\n\n- [NEEDS CLARIFICATION]: How long is a link valid?\n"

// step is one scripted agent turn: optional spec content, then the reply.
type step struct {
	spec  string
	other map[string]string
	reply string
}

type agent struct {
	root    string
	steps   []step
	prompts []string
}

func (a *agent) Name() string { return "fake" }
func (a *agent) Run(_ context.Context, r ports.AgentRequest) (string, error) {
	a.prompts = append(a.prompts, r.Prompt)
	if len(a.steps) == 0 {
		return "", errors.New("unexpected agent call")
	}
	s := a.steps[0]
	a.steps = a.steps[1:]
	if s.spec != "" {
		os.WriteFile(filepath.Join(a.root, "specs", "0001-reset.md"), []byte(s.spec), 0o644)
	}
	for rel, c := range s.other {
		os.WriteFile(filepath.Join(a.root, rel), []byte(c), 0o644)
	}
	return s.reply, nil
}
func (a *agent) Interactive(context.Context, ports.AgentRequest) error { return nil }

type prompter struct{ answers []string }

func (p *prompter) Ask(context.Context, ports.Question) (string, error) {
	if len(p.answers) == 0 {
		return "", ports.ErrNonInteractive
	}
	a := p.answers[0]
	p.answers = p.answers[1:]
	return a, nil
}

type events struct{ asked, answered, rejected int }

func (e *events) Working()          {}
func (e *events) Asked(string, int) { e.asked++ }
func (e *events) Answered()         { e.answered++ }
func (e *events) Rejected(string)   { e.rejected++ }

func ask(q string) string {
	return "```json\n{\"status\":\"needs_clarification\",\"question\":\"" + q + "\",\"context\":\"it decides the scenarios\",\"section\":\"1. Intent\",\"unknowns\":[\"" + q + "\",\"expiry\"]}\n```"
}

const done = "```json\n{\"status\":\"done\",\"files_written\":[\"specs/0001-reset.md\"],\"unknowns\":[]}\n```"

func run(t *testing.T, a *agent, p *prompter) (Result, *events, string, error) {
	t.Helper()
	if a.root == "" {
		a.root = t.TempDir()
		os.MkdirAll(filepath.Join(a.root, "specs"), 0o755)
		os.WriteFile(filepath.Join(a.root, "specs", "0001-reset.md"), []byte(draft), 0o644)
	}
	ev := &events{}
	res, err := Run(context.Background(), Deps{
		Agent:     a,
		Workspace: workspace.New(process.NewRunner(nil)),
		Files:     fsys.OS{},
		Asker:     &clarify.Asker{Prompter: p, Files: fsys.OS{}, Lang: "en", Now: time.Now},
		Events:    ev,
		Now:       time.Now,
	}, Options{Root: a.root, SpecPath: filepath.Join(a.root, "specs", "0001-reset.md"), Language: "en"})
	return res, ev, a.root, err
}

func read(root, rel string) string {
	b, _ := os.ReadFile(filepath.Join(root, rel))
	return string(b)
}

func TestInterviewAsksOneQuestionAtATimeUntilComplete(t *testing.T) {
	a := &agent{steps: []step{
		{reply: ask("Why do users need to reset?")},
		{spec: complete, reply: done},
	}}
	res, ev, root, err := run(t, a, &prompter{answers: []string{"To recover access without support"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Questions != 1 || ev.asked != 1 || ev.answered != 1 || len(res.Open) != 1 {
		t.Fatalf("result %+v events %+v", res, ev)
	}
	if !strings.Contains(a.prompts[1], "Answer: **To recover access without support**") || !strings.Contains(a.prompts[1], "TODO intent") {
		t.Fatalf("second prompt:\n%s", a.prompts[1])
	}
	tr := read(root, "specs/0001-reset/interview.jsonl")
	if strings.Count(tr, "\n") != 2 || !strings.Contains(tr, `"kind":"question"`) || !strings.Contains(tr, `"section":"1. Intent"`) {
		t.Fatalf("interview.jsonl:\n%s", tr)
	}
	if !strings.Contains(read(root, "specs/0001-reset/decisions.md"), "Why do users need to reset?") {
		t.Fatal("answers must also be in the decisions log")
	}
}

func TestDoneWithPlaceholdersIsSentBack(t *testing.T) {
	a := &agent{steps: []step{{reply: done}, {spec: complete, reply: done}}}
	_, ev, _, err := run(t, a, &prompter{})
	if err != nil || ev.rejected != 1 {
		t.Fatalf("err=%v rejected=%d", err, ev.rejected)
	}
	if !strings.Contains(a.prompts[1], "still has:") || !strings.Contains(a.prompts[1], "[placeholder]") {
		t.Fatalf("second prompt:\n%s", a.prompts[1])
	}
	a = &agent{steps: []step{{reply: done}, {reply: done}, {reply: done}}}
	_, _, _, err = run(t, a, &prompter{})
	var inc *IncompleteError
	if !errors.As(err, &inc) {
		t.Fatalf("want IncompleteError, got %v", err)
	}
}

func TestAnUnansweredQuestionResumesFromTheQuestionsFile(t *testing.T) {
	a := &agent{steps: []step{{reply: ask("Who can reset?")}}}
	_, _, root, err := run(t, a, &prompter{})
	var pending *clarify.PendingQuestionError
	if !errors.As(err, &pending) {
		t.Fatalf("want pending, got %v", err)
	}
	q := read(root, "specs/0001-reset/questions.md")
	os.WriteFile(filepath.Join(root, "specs/0001-reset/questions.md"), []byte(strings.Replace(q, "_awaiting an answer_", "Any registered user", 1)), 0o644)

	b := &agent{root: root, steps: []step{{spec: complete, reply: done}}}
	if _, _, _, err := run(t, b, &prompter{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.prompts[0], "Question: Who can reset?\nAnswer: **Any registered user**") {
		t.Fatalf("resumed prompt:\n%s", b.prompts[0])
	}
}

func TestTheInterviewMayOnlyWriteTheSpecification(t *testing.T) {
	a := &agent{steps: []step{{spec: complete, other: map[string]string{"main.go": "package main\n"}, reply: done}}}
	_, _, _, err := run(t, a, &prompter{})
	var scope *ScopeError
	if !errors.As(err, &scope) || scope.Files[0] != "main.go" {
		t.Fatalf("err=%v", err)
	}
}
