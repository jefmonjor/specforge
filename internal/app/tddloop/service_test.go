package tddloop

import (
	"errors"
	"strings"
	"testing"

	"specforge/internal/app/clarify"
	"specforge/internal/domain/quality"
	"specforge/internal/domain/tdd"
)

func happyScenario(marker, testFile, implFile string) []reply {
	return []reply{
		writes(map[string]string{testFile: testFor(marker), implFile: "package m\n"}),
		writes(map[string]string{implFile: "package m\n// implemented " + marker + "\n"}),
	}
}

func TestHappyPathRunsEveryScenarioThroughTheThreePhases(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"), happyScenario("SDD_0001_002", test2, "expiry.go")...)
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), red(1), green(), green()}

	st, err := h.run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !st.Done() || !st.Scenarios[0].Done || !st.Scenarios[1].Done {
		t.Fatalf("state = %+v", st)
	}
	wantFilters := []string{"", "SDD_0001_001", "SDD_0001_001", "", "SDD_0001_002", "SDD_0001_002", ""}
	if strings.Join(h.tests.filters, ",") != strings.Join(wantFilters, ",") {
		t.Fatalf("filters = %q, want %q", h.tests.filters, wantFilters)
	}
	// Regression: the first GREEN prompt must carry the RED failure.
	if !strings.Contains(h.agent.prompts[1], "want link") {
		t.Fatal("the first GREEN prompt lacks the RED failure output")
	}
	if !strings.Contains(h.agent.prompts[0], "**Link**: single-use reset URL.") {
		t.Fatal("prompts must carry the ubiquitous language")
	}
	if len(h.events.accepted) != 6 {
		t.Fatalf("accepted = %v", h.events.accepted)
	}
	if !strings.Contains(h.p.read(".specforge/state/0001-reset.json"), `"phase": "COMPLETED"`) {
		t.Fatal("final state not saved")
	}
}

func TestRedThatDoesNotCompileIsRejectedAndRetried(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = []reply{
		writes(map[string]string{test1: testFor("SDD_0001_001")}),
		writes(map[string]string{test1: testFor("SDD_0001_001") + "// fixed\n", "reset.go": "package m\n"}),
	}
	h.tests.outcomes = []tdd.Outcome{baseline(), notCompiled(), red(1)}

	// The script ends after RED, so the run stops when GREEN calls the agent.
	_, err := h.run(func(o *Options) { o.MaxAttempts = 3 })
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") {
		t.Fatalf("expected the run to stop when the script ends, got %v", err)
	}
	if len(h.events.rejected) == 0 || h.events.rejected[0] != RejectNotCompiled {
		t.Fatalf("rejected = %v", h.events.rejected)
	}
	if !strings.Contains(h.agent.prompts[1], "undefined: Reset") {
		t.Fatal("the retry must show the compiler error")
	}
	if len(h.events.accepted) == 0 || h.events.accepted[0] != tdd.PhaseRed {
		t.Fatalf("the second attempt must be accepted: %v", h.events.accepted)
	}
}

func TestRedWithoutAnyTestFileIsRejected(t *testing.T) {
	h := newHarness(t, specBody)
	h.tests.outcomes = []tdd.Outcome{baseline()}
	h.agent.turns = []reply{
		writes(map[string]string{"reset.go": "package m\n"}),
		writes(map[string]string{"reset2.go": "package m\n"}),
	}
	_, err := h.run(func(o *Options) { o.MaxAttempts = 2 })
	if !errors.Is(err, tdd.ErrAttemptsExhausted) {
		t.Fatalf("want ErrAttemptsExhausted, got %v", err)
	}
	if h.events.rejected[0] != RejectNoTest || h.events.rejected[1] != RejectNoTest {
		t.Fatalf("rejected = %v", h.events.rejected)
	}
	if len(h.tests.filters) != 1 { // the baseline only
		t.Fatal("tests must not run when no test was written")
	}
}

func TestRedWithoutMarkerIsRejected(t *testing.T) {
	h := newHarness(t, specBody)
	h.tests.outcomes = []tdd.Outcome{baseline()}
	h.agent.turns = []reply{writes(map[string]string{test1: testFor("Something")})}
	_, err := h.run(func(o *Options) { o.MaxAttempts = 1 })
	if !errors.Is(err, tdd.ErrAttemptsExhausted) || h.events.rejected[0] != RejectNoMarker {
		t.Fatalf("err=%v rejected=%v", err, h.events.rejected)
	}
}

func TestFalseClaimsAreRejected(t *testing.T) {
	h := newHarness(t, specBody)
	h.tests.outcomes = []tdd.Outcome{baseline()}
	h.agent.turns = []reply{func(p *project, _ string) string {
		p.write(test1, testFor("SDD_0001_001"))
		return done(test1, "never_written.go")
	}}
	_, err := h.run(func(o *Options) { o.MaxAttempts = 1 })
	if !errors.Is(err, tdd.ErrAttemptsExhausted) || h.events.rejected[0] != RejectFalseClaim {
		t.Fatalf("err=%v rejected=%v", err, h.events.rejected)
	}
}

func TestPrematureGreenAsksTheDeveloper(t *testing.T) {
	t.Run("already implemented", func(t *testing.T) {
		h := newHarness(t, specBody)
		h.agent.turns = append([]reply{writes(map[string]string{test1: testFor("SDD_0001_001")})},
			happyScenario("SDD_0001_002", test2, "expiry.go")...)
		h.tests.outcomes = []tdd.Outcome{baseline(), green(), red(1), green(), green()}
		h.prompter.answers = []string{"Yes: mark the scenario as already satisfied"}

		st, err := h.run()
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if !st.Scenarios[0].Satisfied || h.events.satisfied != 1 || len(h.prompter.questions) != 1 {
			t.Fatalf("scenario 1 should be satisfied: %+v", st.Scenarios[0])
		}
		if !strings.Contains(h.p.read("specs/0001-reset/decisions.md"), "passed before any implementation") {
			t.Fatal("the developer's choice must be recorded as a decision")
		}
	})
	t.Run("stop", func(t *testing.T) {
		h := newHarness(t, specBody)
		h.agent.turns = []reply{writes(map[string]string{test1: testFor("SDD_0001_001")})}
		h.tests.outcomes = []tdd.Outcome{baseline(), green()}
		h.prompter.answers = []string{"Stop the loop"}
		if _, err := h.run(); !errors.Is(err, tdd.ErrPrematureGreen) {
			t.Fatalf("want ErrPrematureGreen, got %v", err)
		}
	})
	t.Run("no terminal", func(t *testing.T) {
		h := newHarness(t, specBody)
		h.agent.turns = []reply{writes(map[string]string{test1: testFor("SDD_0001_001")})}
		h.tests.outcomes = []tdd.Outcome{baseline(), green()}
		h.prompter.nonTTY = true
		var pending *clarify.PendingQuestionError
		if _, err := h.run(); !errors.Is(err, tdd.ErrPrematureGreen) || !errors.As(err, &pending) {
			t.Fatalf("want ErrPrematureGreen with a pending question, got %v", err)
		}
	})
}

func TestGreenThatEditsATestStopsTheLoop(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = []reply{
		writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"}),
		// The classic cheat: delete the assertion instead of implementing.
		writes(map[string]string{test1: "package m\nimport \"testing\"\nfunc TestSDD_0001_001_X(t *testing.T) {}\n"}),
	}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1)}

	st, err := h.run()
	var tamper *tdd.TamperingError
	if !errors.As(err, &tamper) || tamper.Phase != tdd.PhaseGreen || tamper.Changed[0] != test1 {
		t.Fatalf("want TamperingError on %s, got %v", test1, err)
	}
	if len(h.tests.filters) != 2 { // baseline and RED
		t.Fatal("tests must not run after tampering")
	}
	if st.Phase != tdd.PhaseGreen {
		t.Fatalf("state must stay at GREEN for --resume, got %s", st.Phase)
	}
}

func TestGreenRunsOutOfAttempts(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = []reply{
		writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"}),
		writes(map[string]string{"reset.go": "package m\n// try 1\n"}),
		writes(map[string]string{"reset.go": "package m\n// try 2\n"}),
	}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), red(1), notCompiled()}

	_, err := h.run(func(o *Options) { o.MaxAttempts = 2 })
	if !errors.Is(err, tdd.ErrAttemptsExhausted) {
		t.Fatalf("want ErrAttemptsExhausted, got %v", err)
	}
	if got := h.events.rejected; len(got) != 2 || got[0] != RejectStillFailing || got[1] != RejectNotCompiled {
		t.Fatalf("rejected = %v", got)
	}
}

func TestRefactorFixesAFailingGate(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"),
		writes(map[string]string{"reset.go": "package m\n// deduplicated\n"}))
	h.agent.turns = append(h.agent.turns, happyScenario("SDD_0001_002", test2, "expiry.go")...)
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), green(), red(1), green(), green()}
	h.gate.results = []quality.Result{{Gate: "fake-gate", Status: quality.Failed, Summary: "4% duplicated"}}

	if _, err := h.run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(h.agent.prompts[2], "4% duplicated") {
		t.Fatal("the REFACTOR prompt must carry the gate finding")
	}
}

func TestStrictModeStopsOnSkippedGatesWithoutCallingTheAgent(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = happyScenario("SDD_0001_001", test1, "reset.go")
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green()}
	h.gate.results = []quality.Result{{Gate: "fake-gate", Status: quality.Skipped, Summary: "jscpd not installed"}}

	_, err := h.run(func(o *Options) { o.Strict = true })
	var ge *GatesError
	if !errors.As(err, &ge) || len(ge.Report.Blocking(true)) != 1 {
		t.Fatalf("want GatesError, got %v", err)
	}
	if len(h.agent.prompts) != 2 {
		t.Fatal("the agent cannot install tools; it must not be asked to")
	}
}

func TestTheAgentCanAskInEveryPhase(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = []reply{
		ask("Which package holds reset?"),
		writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"}),
		ask("30 minutes from request or delivery?"),
		writes(map[string]string{"reset.go": "package m\n// impl\n"}),
		ask("Keep the helper exported?"),
		writes(map[string]string{"reset.go": "package m\n// clean\n"}),
	}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), {Compiled: true, Failed: 1, Output: "boom"}}
	h.gate.results = []quality.Result{{Gate: "fake-gate", Status: quality.Failed, Summary: "dup"}}
	h.prompter.answers = []string{"package m", "from request", "no"}

	_, _ = h.run(func(o *Options) { o.MaxAttempts = 1 })
	if h.events.answered != 3 || len(h.prompter.questions) != 3 {
		t.Fatalf("answered=%d questions=%d", h.events.answered, len(h.prompter.questions))
	}
	decisions := h.p.read("specs/0001-reset/decisions.md")
	for _, phase := range []string{"· RED ·", "· GREEN ·", "· REFACTOR ·"} {
		if !strings.Contains(decisions, phase) {
			t.Errorf("decisions lack a %s entry:\n%s", phase, decisions)
		}
	}
	if !strings.Contains(h.agent.prompts[3], "from request") {
		t.Fatal("the prompt after an answer must include it")
	}
}

func TestAQuestionWithoutTerminalStopsWithAPendingQuestion(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = []reply{ask("Which package holds reset?")}
	h.prompter.nonTTY = true
	h.tests.outcomes = []tdd.Outcome{baseline()}
	var pending *clarify.PendingQuestionError
	if _, err := h.run(); !errors.As(err, &pending) {
		t.Fatalf("want PendingQuestionError, got %v", err)
	}
	if !strings.Contains(h.p.read("specs/0001-reset/questions.md"), "Which package holds reset?") {
		t.Fatal("the question must be saved for the developer")
	}
}

func TestMissingContractIsRetriedOnceThenFails(t *testing.T) {
	h := newHarness(t, specBody)
	h.tests.outcomes = []tdd.Outcome{baseline()}
	prose := func(*project, string) string { return "I implemented everything." }
	h.agent.turns = []reply{prose, prose}
	if _, err := h.run(); err == nil || !strings.Contains(err.Error(), "JSON status object") {
		t.Fatalf("want ErrNoContract, got %v", err)
	}
	if len(h.agent.prompts) != 2 || !strings.Contains(h.agent.prompts[1], "did not end with the JSON status object") {
		t.Fatal("exactly one retry with feedback expected")
	}
}

func TestBlockedAgentStopsWithItsReason(t *testing.T) {
	h := newHarness(t, specBody)
	h.tests.outcomes = []tdd.Outcome{baseline()}
	h.agent.turns = []reply{func(*project, string) string {
		return `{"status":"blocked","reason":"go is not installed","suggested_action":"install Go 1.24"}`
	}}
	var blocked *tdd.AgentBlockedError
	if _, err := h.run(); !errors.As(err, &blocked) || blocked.SuggestedAction != "install Go 1.24" {
		t.Fatalf("want AgentBlockedError, got %v", err)
	}
}

func TestUnsealedOrOpenSpecsNeverStart(t *testing.T) {
	h := newHarness(t, specBody)
	h.p.write("specs/0001-reset.md", specBody) // seal removed
	if _, err := h.run(); err == nil || !strings.Contains(err.Error(), "not sealed") {
		t.Fatalf("unsealed: %v", err)
	}

	h = newHarness(t, specBody+"\n## Open questions\n- [NEEDS CLARIFICATION]: who approves?\n")
	var open *tdd.OpenQuestionsError
	if _, err := h.run(); !errors.As(err, &open) || open.Questions[0] != "who approves?" {
		t.Fatalf("open questions: %v", err)
	}
	if len(h.agent.prompts) != 0 {
		t.Fatal("the agent must not be called")
	}
}

func TestResumeContinuesAndRestartIsExplicit(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = []reply{writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"})}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1)}
	if _, err := h.run(); err == nil {
		t.Fatal("the script ends at GREEN, the run must stop")
	}

	if _, err := h.run(); !errors.Is(err, ErrLoopInProgress) {
		t.Fatalf("a second run without --resume must refuse, got %v", err)
	}

	h.agent.turns = []reply{writes(map[string]string{"reset.go": "package m\n// impl\n"})}
	h.tests.outcomes = []tdd.Outcome{green(), green()}
	h.agent.turns = append(h.agent.turns, happyScenario("SDD_0001_002", test2, "expiry.go")...)
	h.tests.outcomes = append(h.tests.outcomes, red(1), green(), green())
	st, err := h.run(func(o *Options) { o.Resume = true })
	if err != nil || !st.Done() {
		t.Fatalf("resume: %v %+v", err, st)
	}
	if !strings.Contains(h.agent.prompts[1], "want link") {
		t.Fatal("the resumed GREEN prompt must carry the saved RED failure")
	}
}

func TestResumeAfterAnAmendmentRedoesOnlyChangedScenarios(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = happyScenario("SDD_0001_001", test1, "reset.go")
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green()}
	_, _ = h.run() // scenario 1 done, stops at scenario 2

	amended := strings.Replace(specBody, `Then she sees "link expired"`, `Then she sees "the link has expired"`, 1)
	sealed, _ := specSeal(amended)
	h.p.write("specs/0001-reset.md", sealed)

	h.agent.turns = happyScenario("SDD_0001_002", test2, "expiry.go")
	h.tests.outcomes = []tdd.Outcome{red(1), green(), green()}
	st, err := h.run(func(o *Options) { o.Resume = true })
	if err != nil || !st.Done() {
		t.Fatalf("resume after amendment: %v", err)
	}
	if len(h.events.amended) != 1 || h.events.amended[0] != "Expired link" {
		t.Fatalf("only the edited scenario is pending: %v", h.events.amended)
	}
}

func TestResumeWithoutStateFails(t *testing.T) {
	h := newHarness(t, specBody)
	if _, err := h.run(func(o *Options) { o.Resume = true }); !errors.Is(err, ErrNothingToResume) {
		t.Fatalf("want ErrNothingToResume, got %v", err)
	}
}
