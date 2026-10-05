package tddloop

import (
	"errors"
	"strings"
	"testing"

	"specforge/internal/adapters/process"
	"specforge/internal/adapters/scratch"
	"specforge/internal/app/clarify"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
)

const parallelPlan = "# Plan\n\n## Components\n" +
	"- `reset.go`: the reset link (new) · SDD_0001_001\n" +
	"- `expiry.go`: link expiry (new) · SDD_0001_002\n\n" +
	"## Tests per scenario\n| Marker | Scenario | File | Test |\n| :--- | :--- | :--- | :--- |\n" +
	"| SDD_0001_001 | Request a link | `reset_test.go` | TestSDD_0001_001_X |\n" +
	"| SDD_0001_002 | Expired link | `expiry_test.go` | TestSDD_0001_002_X |\n"

// parallelHarness is a loop with two disjoint scenarios and sandboxes.
func parallelHarness(t *testing.T) *harness {
	t.Helper()
	if !process.Available("git") {
		t.Skip("git not installed")
	}
	h := newHarness(t, specBody)
	sealed, _ := spec.Seal(parallelPlan)
	h.p.write("specs/0001-reset/plan.md", sealed)
	h.svc.d.Scratch = scratch.New(process.NewRunner(nil), 0)
	h.agent.byScenario = map[int][]reply{
		1: happyScenario("SDD_0001_001", test1, "reset.go"),
		2: happyScenario("SDD_0001_002", test2, "expiry.go"),
	}
	h.tests.mainRoot = h.p.root
	h.tests.keyed = map[string][]tdd.Outcome{
		"":             {baseline()},
		"SDD_0001_001": {red(1), green()},
		"SDD_0001_002": {red(1), green()},
	}
	h.tests.fallback = map[string]tdd.Outcome{"": green()}
	return h
}

func inParallel(o *Options) { o.Parallel = 2 }

func TestDisjointScenariosRunSideBySide(t *testing.T) {
	h := parallelHarness(t)
	st, err := h.run(inParallel)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !st.Done() || len(h.events.batches) != 1 || strings.Join(h.events.integrated, ",") != "SDD_0001_001,SDD_0001_002" {
		t.Fatalf("batches=%v integrated=%v skipped=%v", h.events.batches, h.events.integrated, h.events.skipped)
	}
	for _, f := range []string{"reset.go", test1, "expiry.go", test2} {
		if h.p.read(f) == "" {
			t.Errorf("%s was not brought into the project", f)
		}
	}
	if !strings.Contains(h.p.read("reset.go"), "implemented SDD_0001_001") {
		t.Fatal("the scenario's final content is brought back")
	}
	if got := st.Scenarios[1].Files; strings.Join(got, ",") != "expiry.go,expiry_test.go" {
		t.Fatalf("each scenario keeps its own files: %v", got)
	}
}

func TestOverlappingScenariosRunInTurn(t *testing.T) {
	h := parallelHarness(t)
	shared := strings.Replace(parallelPlan, "(new) · SDD_0001_002", "(new)", 1) // expiry.go shared by both
	sealed, _ := spec.Seal(shared)
	h.p.write("specs/0001-reset/plan.md", sealed)
	h.agent.byScenario = nil
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"), happyScenario("SDD_0001_002", test2, "expiry.go")...)
	h.tests.keyed = nil
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), red(1), green(), green()}
	if _, err := h.run(inParallel); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(h.events.batches) != 0 {
		t.Fatalf("overlapping surfaces never run side by side: %v", h.events.batches)
	}
}

func TestASeamFailureRunsTheScenariosAgainInTurn(t *testing.T) {
	h := parallelHarness(t)
	broken := tdd.Outcome{Compiled: true, Exact: true, Passed: 3, Failed: 1, Failures: []tdd.TestRef{{Name: "TestTogether"}}, Output: "FAIL TestTogether"}
	h.tests.mainSuite = []tdd.Outcome{baseline(), broken}
	// After the seam check fails, both scenarios run again, one at a time.
	h.agent.byScenario[1] = append(h.agent.byScenario[1], happyScenario("SDD_0001_001", test1, "reset.go")...)
	h.agent.byScenario[2] = append(h.agent.byScenario[2], happyScenario("SDD_0001_002", test2, "expiry.go")...)
	h.tests.keyed["SDD_0001_001"] = append(h.tests.keyed["SDD_0001_001"], red(1), green())
	h.tests.keyed["SDD_0001_002"] = append(h.tests.keyed["SDD_0001_002"], red(1), green())
	st, err := h.run(inParallel)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.events.seams != 1 || len(h.events.integrated) != 0 || !st.Done() {
		t.Fatalf("seams=%d integrated=%v done=%v", h.events.seams, h.events.integrated, st.Done())
	}
}

func TestAScenarioThatStopsRunsAgainOnItsOwn(t *testing.T) {
	h := parallelHarness(t)
	// In its sandbox scenario 2 asks and nobody answers; on its own, right
	// after, it asks again, now with its state in the project.
	h.agent.byScenario[2] = []reply{ask("Which clock?"), ask("Which clock?")}
	h.prompter.nonTTY = true
	_, err := h.run(inParallel)
	var pending *clarify.PendingQuestionError
	if !errors.As(err, &pending) || !strings.HasPrefix(pending.File, h.p.root) {
		t.Fatalf("the question is the project's: %v", err)
	}
	st := h.state(t)
	if !st.Scenarios[0].Done || st.Scenarios[1].Done || st.Pending == nil || st.Current != 1 {
		t.Fatalf("scenario 1 is integrated, scenario 2 waits with its state: %+v", st)
	}
	if n := strings.Count(h.p.read("specs/0001-reset/questions.md"), "Which clock?"); n != 1 {
		t.Fatalf("the question is written once: %d", n)
	}
}

func TestAStoppedScenarioRecordsItsEscalationOnce(t *testing.T) {
	h := parallelHarness(t)
	raised := func(p *project, _ string) string {
		p.write(test2, testFor("SDD_0001_002"))
		return "```json\n{\"status\":\"done\",\"files_written\":[\"" + test2 + "\"],\"risk\":\"high\",\"risk_reason\":\"expiry decides access\"}\n```"
	}
	// The attempt in the sandbox is discarded: its escalation is not a
	// decision about the code that ships, the attempt in the project is.
	h.agent.byScenario[2] = []reply{raised, ask("Which clock?"), raised, ask("Which clock?")}
	h.tests.keyed["SDD_0001_002"] = []tdd.Outcome{red(1), red(1)}
	h.prompter.nonTTY = true
	_, _ = h.run(inParallel)
	if n := strings.Count(h.p.read("specs/0001-reset/decisions.md"), "expiry decides access"); n != 1 {
		t.Fatalf("the escalation is recorded %d times:\n%s", n, h.p.read("specs/0001-reset/decisions.md"))
	}
}

func TestWithoutEntriesKeepsTheOtherDecisions(t *testing.T) {
	log := "### t · RISK · scenario 2\n\n- **Answer:** high\n\n" +
		"### t · GREEN · scenario 2\n\n- **Answer:** UTC\n\n" +
		"### t · RISK\n\n- **Answer:** high\n"
	if got := withoutEntries(log, OriginRisk); got != "### t · GREEN · scenario 2\n\n- **Answer:** UTC\n\n" {
		t.Fatalf("got %q", got)
	}
}
