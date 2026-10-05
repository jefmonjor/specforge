package tddloop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
	mu      sync.Mutex
	p       *project
	turns   []reply
	prompts []string
	models  []string
	// byScenario scripts a parallel loop: the turns of each scenario, by
	// its number, written into the directory the agent runs in.
	byScenario map[int][]reply
}

var scenarioOf = regexp.MustCompile(`Scenario (\d+) of`)

func (a *fakeAgent) Name() string { return "fake" }

func (a *fakeAgent) Run(_ context.Context, req ports.AgentRequest) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.prompts = append(a.prompts, req.Prompt)
	a.models = append(a.models, req.Model)
	if a.byScenario != nil {
		m := scenarioOf.FindStringSubmatch(req.Prompt)
		n := 0
		if m != nil {
			n, _ = strconv.Atoi(m[1])
		}
		turns := a.byScenario[n]
		if len(turns) == 0 {
			return "", fmt.Errorf("unexpected agent call for scenario %d", n)
		}
		a.byScenario[n] = turns[1:]
		return turns[0](&project{t: a.p.t, root: req.Dir}, req.Prompt), nil
	}
	if len(a.turns) == 0 {
		return "", fmt.Errorf("unexpected agent call #%d", len(a.prompts))
	}
	t := a.turns[0]
	a.turns = a.turns[1:]
	return t(a.p, req.Prompt), nil
}

func (a *fakeAgent) Interactive(context.Context, ports.AgentRequest) error { return nil }

type fakeTests struct {
	mu       sync.Mutex
	outcomes []tdd.Outcome
	filters  []string
	// keyed scripts a parallel loop by filter; fallback answers a filter
	// whose script ran out. mainSuite answers the whole suite in the
	// project itself (the seam check), not in a sandbox.
	keyed     map[string][]tdd.Outcome
	fallback  map[string]tdd.Outcome
	mainRoot  string
	mainSuite []tdd.Outcome
}

func (f *fakeTests) Run(_ context.Context, req ports.TestRequest) (tdd.Outcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.filters = append(f.filters, req.Filter)
	if req.Root == f.mainRoot && req.Filter == "" && len(f.mainSuite) > 0 {
		o := f.mainSuite[0]
		f.mainSuite = f.mainSuite[1:]
		return o, nil
	}
	if f.keyed != nil {
		if q := f.keyed[req.Filter]; len(q) > 0 {
			f.keyed[req.Filter] = q[1:]
			return q[0], nil
		}
		if o, ok := f.fallback[req.Filter]; ok {
			return o, nil
		}
		return tdd.Outcome{}, fmt.Errorf("unexpected test run (filter %q)", req.Filter)
	}
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
	mu               sync.Mutex
	rejected         []Rejection
	accepted         []tdd.Phase
	satisfied        int
	answered         int
	amended          []string
	orphaned         []string
	checkpointFailed []string
	commits          []string
	known            int
	risks            []risk.Tier
	notRun           []string

	reviewSkipped int
	reviews       []tdd.ReviewRecord
	verified      []tdd.VerifyRecord
	batches       [][]string
	skipped       []string
	integrated    []string
	seams         int
}

func (r *recorder) Started(*tdd.State, *spec.Document) {}
func (r *recorder) Amended(p []string)                 { r.amended = p }
func (r *recorder) Orphaned(tests []string)            { r.orphaned = tests }
func (r *recorder) CheckpointFailed(reason string) {
	r.checkpointFailed = append(r.checkpointFailed, reason)
}
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
func (r *recorder) Verified(_ tdd.ScenarioRef, rec tdd.VerifyRecord) {
	r.verified = append(r.verified, rec)
}
func (r *recorder) Parallel(m []string) { r.mu.Lock(); r.batches = append(r.batches, m); r.mu.Unlock() }
func (r *recorder) ParallelSkipped(sc tdd.ScenarioRef, why string) {
	r.mu.Lock()
	r.skipped = append(r.skipped, sc.Marker+": "+why)
	r.mu.Unlock()
}
func (r *recorder) SeamFailed(string) { r.mu.Lock(); r.seams++; r.mu.Unlock() }
func (r *recorder) Integrated(sc tdd.ScenarioRef) {
	r.mu.Lock()
	r.integrated = append(r.integrated, sc.Marker)
	r.mu.Unlock()
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
