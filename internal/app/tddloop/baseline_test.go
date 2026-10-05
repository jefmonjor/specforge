package tddloop

import (
	"errors"
	"strings"
	"testing"

	"specforge/internal/domain/tdd"
)

var (
	legacyBug = tdd.TestRef{Suite: "example.com/m", Name: "TestLegacyRounding"}
	clockBug  = tdd.TestRef{Suite: "example.com/m", Name: "TestClock"}
)

// suite is a whole-suite run with the given failing tests.
func suite(failing ...tdd.TestRef) tdd.Outcome {
	return tdd.Outcome{Compiled: true, Exact: true, Passed: 4, Failed: len(failing), Failures: failing, Output: "--- FAIL: " + tdd.Names(failing)}
}

func TestKnownFailuresDoNotBlockRefactor(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"), happyScenario("SDD_0001_002", test2, "expiry.go")...)
	h.tests.outcomes = []tdd.Outcome{
		suite(legacyBug, clockBug), // baseline: two tests already fail
		red(1), green(), suite(legacyBug, clockBug),
		red(1), green(), suite(legacyBug), // the clock test passes now
	}
	st, err := h.run()
	if err != nil || !st.Done() {
		t.Fatalf("known failures must not block: err=%v", err)
	}
	if h.events.known != 2 {
		t.Fatalf("the developer is told about both known failures: %d", h.events.known)
	}
	if !st.Baseline.Has(legacyBug) || st.Baseline.Has(clockBug) {
		t.Fatalf("a known failure that passes leaves the baseline: %+v", st.Baseline)
	}
	for _, p := range h.agent.prompts {
		if !strings.Contains(p, "Known failures on this branch") || !strings.Contains(p, "TestLegacyRounding") {
			t.Fatalf("every prompt names the known failures:\n%s", p)
		}
	}
	if saved := h.state(t); !saved.Baseline.Has(legacyBug) {
		t.Fatal("the baseline is persisted for --resume and deliver")
	}
}

func TestAFreshFailureBlocksAndNamesTheTest(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"),
		writes(map[string]string{"reset.go": "package m\n// fixed\n"}))
	broke := tdd.TestRef{Suite: "example.com/m", Name: "TestExistingFeature"}
	h.tests.outcomes = []tdd.Outcome{suite(legacyBug), red(1), green(), suite(legacyBug, broke), suite(legacyBug)}
	h.prompter.answers = nil
	_, err := h.run(func(o *Options) { o.MaxAttempts = 2 })
	if err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("want the script to end at scenario 2, got %v", err)
	}
	refactor := h.agent.prompts[2]
	if !strings.Contains(refactor, "did not fail before the loop") || !strings.Contains(refactor, "TestExistingFeature") {
		t.Fatalf("the REFACTOR prompt must name the new failure:\n%s", refactor)
	}
}

func TestARunnerWithoutNamesKeepsEveryFailureBlocking(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"),
		writes(map[string]string{"reset.go": "package m\n// refactored\n"}))
	unnamed := tdd.Outcome{Compiled: true, Exact: false, Failed: 1, Output: "npm ERR! Test failed"}
	h.tests.outcomes = []tdd.Outcome{unnamed, red(1), green(), unnamed, unnamed}
	_, err := h.run(func(o *Options) { o.MaxAttempts = 1 })
	var gates *GatesError
	if !errors.As(err, &gates) || gates.SuiteFailure == "" {
		t.Fatalf("an unnamed failure blocks as before: %v", err)
	}
	if h.state(t).Baseline != nil {
		t.Fatal("no baseline without names")
	}
}

func TestResumeDoesNotTakeTheBaselineAgain(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = []reply{writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"})}
	h.tests.outcomes = []tdd.Outcome{suite(legacyBug), red(1)}
	_, _ = h.run()
	h.agent.turns = []reply{writes(map[string]string{"reset.go": "package m\n// impl\n"})}
	h.tests.outcomes = []tdd.Outcome{green(), suite(legacyBug)}
	_, err := h.run(func(o *Options) { o.Resume = true })
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") {
		t.Fatalf("want the script to end at scenario 2, got %v", err)
	}
	if h.tests.filters[2] != "SDD_0001_001" {
		t.Fatalf("resume starts with GREEN's test, not a new baseline: %q", h.tests.filters)
	}
}
