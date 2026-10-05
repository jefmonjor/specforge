package tddloop

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/jefmonjor/specforge/v6/internal/domain/quality"
	"github.com/jefmonjor/specforge/v6/internal/domain/risk"
	"github.com/jefmonjor/specforge/v6/internal/domain/stack"
	"github.com/jefmonjor/specforge/v6/internal/domain/tdd"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// mutationGate is a gate named like the real mutation gate.
type mutationGate struct{ runs int }

func (*mutationGate) Name() string               { return "mutation" }
func (*mutationGate) Applies(stack.Profile) bool { return true }
func (g *mutationGate) Check(context.Context, string, stack.Profile) (quality.Result, error) {
	g.runs++
	return quality.Result{Gate: "mutation", Status: quality.Passed}, nil
}

func rules(t *testing.T, high, passive []string) risk.Rules {
	t.Helper()
	r, err := risk.NewRules(0, high, passive, risk.Passive)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRiskIsAssessedFromTheFilesThatChanged(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = []reply{
		writes(map[string]string{test1: testFor("SDD_0001_001"), "auth/reset.go": "package m\n"}),
		writes(map[string]string{"auth/reset.go": "package m\n// impl\n"}),
	}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green()}
	_, err := h.run()
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") {
		t.Fatalf("want the script to end at scenario 2, got %v", err)
	}
	st := h.state(t)
	a := st.Scenarios[0].Risk
	if a == nil || a.Tier != risk.High || !strings.Contains(strings.Join(a.Reasons, ";"), "`auth/reset.go` is a sensitive path") {
		t.Fatalf("risk = %+v", a)
	}
	if len(h.events.risks) == 0 || h.events.risks[0] != risk.High {
		t.Fatalf("the developer sees the tier: %v", h.events.risks)
	}
}

func TestTheAgentCanRaiseTheRiskButNotLowerIt(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = []reply{
		writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"}),
		func(p *project, _ string) string {
			p.write("reset.go", "package m\n// impl\n")
			return "```json\n{\"status\":\"done\",\"files_written\":[\"reset.go\"],\"risk\":\"high\",\"risk_reason\":\"the link embeds a signed token\"}\n```"
		},
	}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green()}
	_, _ = h.run()
	st := h.state(t)
	a := st.Scenarios[0].Risk
	if a == nil || a.Tier != risk.High || !slices.Contains(a.Reasons, "raised by the agent: the link embeds a signed token") {
		t.Fatalf("risk = %+v", a)
	}
	if d := h.p.read("specs/0001-reset/decisions.md"); !strings.Contains(d, "· RISK ·") || !strings.Contains(d, "high · the link embeds a signed token") {
		t.Fatalf("the escalation is a recorded decision:\n%s", d)
	}
}

func TestAnInvalidRiskIsAContractViolation(t *testing.T) {
	h := newHarness(t, specBody)
	noReason := func(*project, string) string {
		return "```json\n{\"status\":\"done\",\"files_written\":[],\"risk\":\"high\"}\n```"
	}
	h.agent.turns = []reply{noReason, noReason}
	h.tests.outcomes = []tdd.Outcome{baseline()}
	if _, err := h.run(); err == nil || !strings.Contains(err.Error(), "JSON status object") {
		t.Fatalf("a risk without a reason is not a valid contract: %v", err)
	}
}

func TestReviewByRiskSkipsPassiveScenarios(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"), happyScenario("SDD_0001_002", test2, "expiry.go")...)
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), red(1), green(), green()}
	h.prompter.answers = []string{"Accept"}
	_, err := h.run(func(o *Options) {
		o.Review = ReviewRisk
		// Scenario 1's files are passive; scenario 2's expiry.go is not.
		o.Risk = rules(t, []string{`^$`}, []string{`reset`})
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if h.events.reviewSkipped != 1 || len(h.prompter.questions) != 1 || !strings.Contains(h.prompter.questions[0].Text, "scenario 2") {
		t.Fatalf("only the medium scenario is reviewed: skipped=%d questions=%v", h.events.reviewSkipped, h.prompter.questions)
	}
}

func TestMutationRunsFromTheConfiguredTier(t *testing.T) {
	h := newHarness(t, specBody)
	mutation := &mutationGate{}
	h.svc.d.Gates = []ports.Gate{h.gate, mutation}
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"), happyScenario("SDD_0001_002", test2, "auth.go")...)
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), red(1), green(), green()}
	_, err := h.run(func(o *Options) { o.MutationFrom = risk.High })
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if mutation.runs != 1 || !slices.Equal(h.events.notRun, []string{"mutation"}) {
		t.Fatalf("mutation runs only for the high scenario: runs=%d notRun=%v", mutation.runs, h.events.notRun)
	}
}

func TestEveryPhaseGetsItsModel(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = []reply{
		writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"}),
		writes(map[string]string{"reset.go": "package m\n// impl\n"}),
		writes(map[string]string{"reset.go": "package m\n// clean\n"}),
	}
	h.gate.results = []quality.Result{{Gate: "fake-gate", Status: quality.Failed, Summary: "lint"}}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), green()}
	models := map[string]string{"red": "big", "green": "small", "refactor": "medium"}
	_, _ = h.run(func(o *Options) { o.Models = func(phase string) string { return models[phase] } })
	if want := []string{"big", "small", "medium"}; !slices.Equal(h.agent.models[:3], want) {
		t.Fatalf("models = %v, want %v", h.agent.models, want)
	}
}
