package tddloop

import (
	"errors"
	"strings"
	"testing"

	"github.com/jefmonjor/specforge/v6/internal/app/clarify"
	"github.com/jefmonjor/specforge/v6/internal/domain/spec"
	"github.com/jefmonjor/specforge/v6/internal/domain/tdd"
)

const planBody = "# Plan\n\n## Components\n- `reset.go`: the reset link (new)\n- `expiry.go`: link expiry (new)\n\n" +
	"## Tests per scenario\n| Marker | Scenario | File | Test |\n| :--- | :--- | :--- | :--- |\n" +
	"| SDD_0001_001 | Request a link | `reset_test.go` | TestSDD_0001_001_X |\n" +
	"| SDD_0001_002 | Expired link | `expiry_test.go` | TestSDD_0001_002_X |\n"

// planned approves planBody for the specification.
func planned(h *harness) {
	sealed, _ := spec.Seal(planBody)
	h.p.write("specs/0001-reset/plan.md", sealed)
}

// outside writes the scenario's implementation plus a file the plan does
// not name.
func outside(file string) reply {
	return writes(map[string]string{"reset.go": "package m\n// impl\n", file: "x\n"})
}

func TestTheAgentIsToldWhichFilesItMayEdit(t *testing.T) {
	h := newHarness(t, specBody)
	planned(h)
	h.agent.turns = []reply{writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"})}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1)}
	_, _ = h.run()
	if p := h.agent.prompts[0]; !strings.Contains(p, "## Allowed edit surfaces") || !strings.Contains(p, "- `expiry_test.go`") {
		t.Fatalf("the RED prompt lists the plan's files:\n%s", p)
	}
}

func TestAFileOutsideThePlanGoesToTheDeveloper(t *testing.T) {
	t.Run("accepted", func(t *testing.T) {
		h := newHarness(t, specBody)
		planned(h)
		h.agent.turns = append([]reply{writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"}), outside("README.md")},
			happyScenario("SDD_0001_002", test2, "expiry.go")...)
		h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), red(1), green(), green()}
		h.prompter.answers = []string{"Accept them"}
		st, err := h.run()
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if len(st.Surfaces) != 1 || st.Surfaces[0] != "README.md" || !strings.Contains(h.prompter.questions[0].Text, "README.md") {
			t.Fatalf("surfaces=%v questions=%v", st.Surfaces, h.prompter.questions)
		}
		if !strings.Contains(strings.Join(st.Scenarios[0].Files, ","), "README.md") {
			t.Fatal("an accepted file belongs to the scenario")
		}
		if !strings.Contains(h.p.read("specs/0001-reset/decisions.md"), "README.md") {
			t.Fatal("the acceptance is a recorded decision")
		}
	})
	t.Run("refused and put back", func(t *testing.T) {
		h := newHarness(t, specBody)
		planned(h)
		h.p.gitInit()
		h.agent.turns = []reply{
			writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"}),
			outside("go.mod"),
			func(p *project, _ string) string { // puts go.mod back
				p.write("go.mod", "module example.com/m\n")
				return done("go.mod")
			},
		}
		h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green()}
		h.prompter.answers = []string{"Refuse them"}
		_, err := h.run()
		if err == nil || !strings.Contains(err.Error(), "unexpected agent call") {
			t.Fatalf("want the script to end at scenario 2, got %v", err)
		}
		if !strings.Contains(h.agent.prompts[2], "not in the approved plan: go.mod") {
			t.Fatalf("the next GREEN says what to put back:\n%s", h.agent.prompts[2])
		}
		st := h.state(t)
		if len(st.Refused) != 0 || strings.Contains(strings.Join(st.Scenarios[0].Files, ","), "go.mod") {
			t.Fatalf("a put-back file is neither refused nor committed: %+v", st.Scenarios[0])
		}
	})
}

func TestStrictSurfacesRejectWithoutAsking(t *testing.T) {
	h := newHarness(t, specBody)
	planned(h)
	h.agent.turns = []reply{writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n", "notes.txt": "x"})}
	h.tests.outcomes = []tdd.Outcome{baseline()}
	_, err := h.run(func(o *Options) { o.Surfaces, o.MaxAttempts = SurfacesStrict, 1 })
	if !errors.Is(err, tdd.ErrAttemptsExhausted) || h.events.rejected[0] != RejectOutsidePlan || len(h.prompter.questions) != 0 {
		t.Fatalf("err=%v rejected=%v questions=%v", err, h.events.rejected, h.prompter.questions)
	}
}

func TestARefusedFileThatStaysStopsTheScenario(t *testing.T) {
	h := newHarness(t, specBody)
	planned(h)
	h.agent.turns = []reply{
		writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"}),
		outside("notes.txt"),
		writes(map[string]string{"reset.go": "package m\n// impl again\n"}), // ignores the refusal
		writes(map[string]string{"reset.go": "package m\n// once more\n"}),
	}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1)}
	h.prompter.answers = []string{"Refuse them"}
	_, err := h.run(func(o *Options) { o.MaxAttempts = 3 })
	if !errors.Is(err, tdd.ErrAttemptsExhausted) {
		t.Fatalf("an unreverted refused file keeps rejecting: %v", err)
	}
	if n := strings.Count(strings.Join(rejections(h), ","), string(RejectOutsidePlan)); n < 2 {
		t.Fatalf("rejected = %v", h.events.rejected)
	}
}

func TestAQuestionAboutSurfacesWithoutTerminalIsAnsweredOnResume(t *testing.T) {
	h := newHarness(t, specBody)
	planned(h)
	h.prompter.nonTTY = true
	h.agent.turns = []reply{writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"}), outside("README.md")}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1)}
	_, err := h.run()
	var pending *clarify.PendingQuestionError
	if !errors.As(err, &pending) {
		t.Fatalf("want a pending question, got %v", err)
	}
	answerInFile(t, h.p, "1")
	h.agent.turns = nil
	h.tests.outcomes = []tdd.Outcome{green(), green()}
	_, err = h.run(func(o *Options) { o.Resume = true })
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") || len(h.agent.prompts) != 3 || !strings.Contains(h.agent.prompts[2], "# Task: RED") {
		t.Fatalf("the answered turn is verified again without calling the agent: err=%v calls=%d", err, len(h.agent.prompts))
	}
	if st := h.state(t); len(st.Surfaces) != 1 || !st.Scenarios[0].Done {
		t.Fatalf("state = %+v", st)
	}
}

func TestWithoutAPlanNothingIsChecked(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = []reply{writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n", "anything.txt": "x"})}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1)}
	_, _ = h.run()
	if len(h.prompter.questions) != 0 || strings.Contains(h.agent.prompts[0], "Allowed edit surfaces") {
		t.Fatal("no plan, no surfaces")
	}
}

func rejections(h *harness) []string {
	var out []string
	for _, r := range h.events.rejected {
		out = append(out, string(r))
	}
	return out
}
