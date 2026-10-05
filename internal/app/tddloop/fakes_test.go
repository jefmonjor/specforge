package tddloop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/process"
	"specforge/internal/adapters/workspace"
	"specforge/internal/app/clarify"
	"specforge/internal/domain/quality"
	"specforge/internal/domain/risk"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/stack"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

var goProfile = stack.Profile{Kind: stack.Go, Runner: stack.RunnerGo}

const specBody = `# Password reset

## 2. Ubiquitous language
- **Link**: single-use reset URL.

## 5. Acceptance criteria

` + "```gherkin" + `
Feature: Password reset
  Scenario: Request a link
    Given a registered user
    When she asks for a reset
    Then she gets a link

  Scenario: Expired link
    Given a link issued 31 minutes ago
    When she opens it
    Then she sees "link expired"
` + "```\n"

// project is a temporary repository with a sealed specification.
type project struct {
	t        *testing.T
	root     string
	specPath string
}

func newProject(t *testing.T, body string) *project {
	t.Helper()
	root := t.TempDir()
	p := &project{t: t, root: root, specPath: filepath.Join(root, "specs", "0001-reset.md")}
	sealed, _ := spec.Seal(body)
	p.write("specs/0001-reset.md", sealed)
	p.write("go.mod", "module example.com/m\n")
	return p
}

func (p *project) write(rel, content string) {
	p.t.Helper()
	if err := fsys.WriteAtomic(filepath.Join(p.root, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
		p.t.Fatal(err)
	}
}

// gitInit makes the project a repository with everything committed, so
// snapshots hold only what changes afterwards.
func (p *project) gitInit() {
	p.t.Helper()
	p.t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	p.t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(p.t.TempDir(), "gitconfig"))
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", p.root}, args...)...).CombinedOutput(); err != nil {
			p.t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}

func (p *project) read(rel string) string {
	data, _ := os.ReadFile(filepath.Join(p.root, filepath.FromSlash(rel)))
	return string(data)
}

// reply is one scripted agent answer: it edits the project and replies.
type reply func(p *project, prompt string) string

func done(files ...string) string {
	return fmt.Sprintf("Done.\n```json\n{\"status\":\"done\",\"files_written\":[%s]}\n```", quoted(files))
}

func quoted(files []string) string {
	var q []string
	for _, f := range files {
		q = append(q, `"`+f+`"`)
	}
	return strings.Join(q, ",")
}

// writes returns a reply that writes files and reports exactly them.
func writes(files map[string]string) reply {
	return func(p *project, _ string) string {
		var names []string
		for name, content := range files {
			p.write(name, content)
			names = append(names, name)
		}
		return done(names...)
	}
}

func ask(question string) reply {
	return func(*project, string) string {
		return "```json\n{\"status\":\"needs_clarification\",\"question\":\"" + question + "\",\"options\":[\"a\",\"b\"]}\n```"
	}
}

type fakeAgent struct {
	p       *project
	turns   []reply
	prompts []string
	models  []string
}

func (a *fakeAgent) Name() string { return "fake" }

func (a *fakeAgent) Run(_ context.Context, req ports.AgentRequest) (string, error) {
	a.prompts = append(a.prompts, req.Prompt)
	a.models = append(a.models, req.Model)
	if len(a.turns) == 0 {
		return "", fmt.Errorf("unexpected agent call #%d", len(a.prompts))
	}
	t := a.turns[0]
	a.turns = a.turns[1:]
	return t(a.p, req.Prompt), nil
}

func (a *fakeAgent) Interactive(context.Context, ports.AgentRequest) error { return nil }

type fakeTests struct {
	outcomes []tdd.Outcome
	filters  []string
}

func (f *fakeTests) Run(_ context.Context, req ports.TestRequest) (tdd.Outcome, error) {
	f.filters = append(f.filters, req.Filter)
	if len(f.outcomes) == 0 {
		return tdd.Outcome{}, fmt.Errorf("unexpected test run #%d (filter %q)", len(f.filters), req.Filter)
	}
	o := f.outcomes[0]
	f.outcomes = f.outcomes[1:]
	return o, nil
}

type fakeGate struct {
	results []quality.Result
}

func (g *fakeGate) Name() string               { return "fake-gate" }
func (g *fakeGate) Applies(stack.Profile) bool { return true }
func (g *fakeGate) Check(context.Context, string, stack.Profile) (quality.Result, error) {
	if len(g.results) == 0 {
		return quality.Result{Gate: "fake-gate", Status: quality.Passed}, nil
	}
	r := g.results[0]
	g.results = g.results[1:]
	return r, nil
}

type fakePrompter struct {
	answers   []string
	questions []ports.Question
	nonTTY    bool
}

func (f *fakePrompter) Ask(_ context.Context, q ports.Question) (string, error) {
	f.questions = append(f.questions, q)
	if f.nonTTY {
		return "", ports.ErrNonInteractive
	}
	if len(f.answers) == 0 {
		return "", fmt.Errorf("unexpected question: %s", q.Text)
	}
	a := f.answers[0]
	f.answers = f.answers[1:]
	return a, nil
}

type recorder struct {
	mu        sync.Mutex
	rejected  []Rejection
	accepted  []tdd.Phase
	satisfied int
	answered  int
	amended   []string
	commits   []string
	known     int
	risks     []risk.Tier
	notRun    []string

	reviewSkipped int
	reviews       []tdd.ReviewRecord
}

func (r *recorder) Started(*tdd.State, *spec.Document) {}
func (r *recorder) Amended(p []string)                 { r.amended = p }
func (r *recorder) Baseline(b *tdd.Baseline, _ bool) {
	if b != nil {
		r.known = len(b.Failures)
	}
}
func (r *recorder) Phase(*tdd.State, tdd.ScenarioRef) {}
func (r *recorder) AgentWorking(tdd.Phase)            {}
func (r *recorder) RunningTests(string)               {}
func (r *recorder) Rejected(why Rejection, _ string) {
	r.mu.Lock()
	r.rejected = append(r.rejected, why)
	r.mu.Unlock()
}
func (r *recorder) Answered(string, string)                  { r.answered++ }
func (r *recorder) Gates(quality.Report)                     {}
func (r *recorder) Accepted(ph tdd.Phase, _ tdd.ScenarioRef) { r.accepted = append(r.accepted, ph) }
func (r *recorder) Satisfied(tdd.ScenarioRef)                { r.satisfied++ }
func (r *recorder) Committed(_ tdd.ScenarioRef, sha string)  { r.commits = append(r.commits, sha) }
func (r *recorder) Finished(*tdd.State)                      {}
func (r *recorder) GateNotRun(g string, _ risk.Tier)         { r.notRun = append(r.notRun, g) }
func (r *recorder) Risk(_ tdd.ScenarioRef, a risk.Assessment) {
	r.risks = append(r.risks, a.Tier)
}
func (r *recorder) ReviewSkipped(tdd.ScenarioRef, risk.Assessment) { r.reviewSkipped++ }
func (r *recorder) Reviewed(_ tdd.ScenarioRef, rec tdd.ReviewRecord) {
	r.reviews = append(r.reviews, rec)
}

type harness struct {
	p        *project
	agent    *fakeAgent
	tests    *fakeTests
	gate     *fakeGate
	prompter *fakePrompter
	events   *recorder
	svc      *Service
}

func newHarness(t *testing.T, body string) *harness {
	p := newProject(t, body)
	h := &harness{p: p, agent: &fakeAgent{p: p}, tests: &fakeTests{}, gate: &fakeGate{}, prompter: &fakePrompter{}, events: &recorder{}}
	clock := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	h.svc = New(Deps{
		Agent:     h.agent,
		Tests:     h.tests,
		Gates:     []ports.Gate{h.gate},
		Workspace: workspace.New(process.NewRunner(nil)),
		Files:     fsys.OS{},
		Asker:     &clarify.Asker{Prompter: h.prompter, Files: fsys.OS{}, Lang: "en", Now: func() time.Time { return clock }},
		Events:    h.events,
		Now:       func() time.Time { return clock },
	})
	return h
}

func (h *harness) run(opts ...func(*Options)) (*tdd.State, error) {
	o := Options{Root: h.p.root, SpecPath: h.p.specPath, Profile: goProfile, Language: "en", MaxAttempts: 3}
	for _, f := range opts {
		f(&o)
	}
	return h.svc.Run(context.Background(), o)
}

func red(failed int) tdd.Outcome {
	return tdd.Outcome{Compiled: true, Exact: true, Failed: failed, Output: "want link"}
}
func green() tdd.Outcome { return tdd.Outcome{Compiled: true, Exact: true, Passed: 1} }

// baseline is the whole-suite run a new loop takes before its first RED:
// nothing fails yet.
func baseline() tdd.Outcome { return tdd.Outcome{Compiled: true, Exact: true, Passed: 3} }
func notCompiled() tdd.Outcome {
	return tdd.Outcome{Compiled: false, Exact: true, Output: "undefined: Reset"}
}

const (
	test1 = "reset_test.go"
	test2 = "expiry_test.go"
)

func testFor(marker string) string {
	return "package m\nimport \"testing\"\nfunc Test" + marker + "_X(t *testing.T) { t.Fatal(\"todo\") }\n"
}

func specSeal(body string) (string, string) { return spec.Seal(body) }

func (h *harness) state(t *testing.T) *tdd.State {
	t.Helper()
	var st tdd.State
	if err := json.Unmarshal([]byte(h.p.read(".specforge/state/0001-reset.json")), &st); err != nil {
		t.Fatal(err)
	}
	return &st
}
