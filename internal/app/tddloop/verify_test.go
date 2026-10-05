package tddloop

import (
	"context"
	"errors"
	"strings"
	"testing"

	"specforge/internal/app/verifier"
	"specforge/internal/domain/tdd"
	"specforge/internal/domain/verification"
)

type fakeVerifier struct {
	reports  []verification.Report
	requests []verifier.Request
}

func (f *fakeVerifier) Verify(_ context.Context, req verifier.Request) (verifier.Result, error) {
	f.requests = append(f.requests, req)
	if len(f.reports) == 0 {
		return verifier.Result{}, errors.New("unexpected verification")
	}
	r := f.reports[0]
	f.reports = f.reports[1:]
	return verifier.Result{Report: r, Required: req.Required}, nil
}

// invSpec names INV-01 in scenario 1 only; INV-02 in no scenario.
var invSpec = strings.Replace(specBody, "Scenario: Request a link", "Scenario: Request a link (INV-01)", 1) +
	"\n## 4. Invariants\n\n- **INV-01**: a link is used once.\n- **INV-02**: a link expires.\n"

var brokenOnce = verification.Report{
	Verdicts: []verification.Verdict{{ID: "INV-01", Status: verification.Unmet}, {ID: "SDD_0001_001", Status: verification.Met}},
	Blockers: []verification.Blocker{{ID: "INV-01", Command: "go run . open twice", Observed: "opened", Expected: "link used"}},
}

func verifying(h *harness, f *fakeVerifier) { h.svc.d.Verifier = f }

func always(o *Options) { o.Verify = VerifyAlways }

func TestTheVerifierChecksTheScenarioAndItsCorrection(t *testing.T) {
	h := newHarness(t, invSpec)
	f := &fakeVerifier{reports: []verification.Report{brokenOnce, {Verdicts: []verification.Verdict{{ID: "INV-01", Status: verification.Met}}}}}
	verifying(h, f)
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"),
		writes(map[string]string{"reset.go": "package m\n// implemented SDD_0001_001\n// used once\n"}))
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), green()}
	_, err := h.run(always)
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") {
		t.Fatalf("want the script to end at scenario 2, got %v", err)
	}
	if got := strings.Join(f.requests[0].Required, ","); got != "INV-01,SDD_0001_001" {
		t.Fatalf("the verifier answers for the scenario and the invariants it names: %s", got)
	}
	if got := strings.Join(f.requests[1].Required, ","); got != "INV-01" {
		t.Fatalf("the recheck asks only for what was broken: %s", got)
	}
	if c := h.agent.prompts[2]; !strings.Contains(c, "# Task: CORRECT") || !strings.Contains(c, "`go run . open twice` printed `opened`") {
		t.Fatalf("the correction carries the command and its output:\n%s", c)
	}
	st := h.state(t)
	if rec := st.Scenarios[0].Verify; rec == nil || !rec.Done || rec.Recheck == nil || len(rec.Recheck.Blockers) != 0 || !st.Scenarios[0].Done {
		t.Fatalf("verify record = %+v", rec)
	}
	if !strings.Contains(h.p.read("specs/0001-reset/verify/SDD_0001_001.json"), `"done": true`) {
		t.Fatal("the verification is kept next to the specification")
	}
}

func TestStillBrokenAfterTheCorrectionGoesToTheDeveloper(t *testing.T) {
	h := newHarness(t, invSpec)
	verifying(h, &fakeVerifier{reports: []verification.Report{brokenOnce, brokenOnce}})
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"), writes(map[string]string{"reset.go": "package m\n// x\n"}))
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), green()}
	h.prompter.answers = []string{"Keep it as a follow-up"}
	_, _ = h.run(always)
	if rec := h.state(t).Scenarios[0].Verify; rec == nil || len(rec.FollowUps) != 1 || rec.FollowUps[0] != "INV-01" {
		t.Fatalf("verify record = %+v", rec)
	}
	if !strings.Contains(h.prompter.questions[0].Text, "There is no second automatic correction") {
		t.Fatalf("question = %v", h.prompter.questions)
	}
}

func TestRegressionTestsAreOfferedAndMustPass(t *testing.T) {
	h := newHarness(t, invSpec)
	withTest := verification.Report{
		Verdicts:        []verification.Verdict{{ID: "INV-01", Status: verification.Met}, {ID: "SDD_0001_001", Status: verification.Met}},
		RegressionTests: []verification.RegressionTest{{Path: "reset_once_test.go", Covers: []string{"INV-01"}, Content: "package m\n"}, {Path: "../escape_test.go", Covers: []string{"INV-01"}, Content: "x"}},
	}
	verifying(h, &fakeVerifier{reports: []verification.Report{withTest}})
	h.agent.turns = happyScenario("SDD_0001_001", test1, "reset.go")
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), green()}
	h.prompter.answers = []string{"Add them"}
	_, _ = h.run(always)
	st := h.state(t)
	if rec := st.Scenarios[0].Verify; rec == nil || len(rec.Tests) != 1 || rec.Tests[0] != "reset_once_test.go" {
		t.Fatalf("only a new test file inside the project is added: %+v", rec)
	}
	if !strings.Contains(strings.Join(st.Scenarios[0].Files, ","), "reset_once_test.go") {
		t.Fatalf("an added test is part of the scenario: %v", st.Scenarios[0].Files)
	}
}

func TestTheVerifierRunsOnlyForHighRiskByDefault(t *testing.T) {
	h := newHarness(t, invSpec)
	f := &fakeVerifier{}
	verifying(h, f)
	h.agent.turns = happyScenario("SDD_0001_001", test1, "reset.go")
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green()}
	_, _ = h.run()
	if len(f.requests) != 0 || !h.state(t).Scenarios[0].Done {
		t.Fatalf("a medium scenario is not verified by default: %d", len(f.requests))
	}
}

func TestFeatureVerificationRunsOnceAtTheEnd(t *testing.T) {
	h := newHarness(t, invSpec)
	f := &fakeVerifier{reports: []verification.Report{brokenOnce}}
	verifying(h, f)
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"), happyScenario("SDD_0001_002", test2, "expiry.go")...)
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), red(1), green(), green()}
	_, err := h.run(func(o *Options) { o.Verify = VerifyFeature })
	var blocked *verifier.BlockedError
	if !errors.As(err, &blocked) || len(f.requests) != 1 || strings.Join(f.requests[0].Required, ",") != "INV-01,INV-02,SDD_0001_001,SDD_0001_002" {
		t.Fatalf("err=%v requests=%+v", err, f.requests)
	}
	if !strings.HasSuffix(f.requests[0].Report, "verify/feature.json") {
		t.Fatalf("report = %s", f.requests[0].Report)
	}
}

func TestAFailingRegressionTestIsNotAdded(t *testing.T) {
	h := newHarness(t, invSpec)
	withTest := verification.Report{
		Verdicts:        []verification.Verdict{{ID: "INV-01", Status: verification.Met}, {ID: "SDD_0001_001", Status: verification.Met}},
		RegressionTests: []verification.RegressionTest{{Path: "reset_once_test.go", Covers: []string{"INV-01"}, Content: "package m\n"}},
	}
	verifying(h, &fakeVerifier{reports: []verification.Report{withTest}})
	h.agent.turns = happyScenario("SDD_0001_001", test1, "reset.go")
	failing := tdd.Outcome{Compiled: true, Exact: true, Passed: 1, Failed: 1, Failures: []tdd.TestRef{{Name: "TestOnce"}}, Output: "FAIL TestOnce"}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), failing}
	h.prompter.answers = []string{"Add them"}
	_, _ = h.run(always)
	rec := h.state(t).Scenarios[0].Verify
	if rec == nil || len(rec.Tests) != 0 || h.p.read("reset_once_test.go") != "" || !strings.Contains(strings.Join(rec.Report.Advisories, ";"), "failed and were not added") {
		t.Fatalf("a failing regression test is removed and reported: %+v", rec)
	}
}
