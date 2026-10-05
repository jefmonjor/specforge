package tddloop

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jefmonjor/specforge/v6/internal/app/clarify"
	"github.com/jefmonjor/specforge/v6/internal/app/reviewer"
	"github.com/jefmonjor/specforge/v6/internal/domain/review"
	"github.com/jefmonjor/specforge/v6/internal/domain/tdd"
)

// fakeReviewer answers with scripted verdicts.
type fakeReviewer struct {
	verdict    review.Verdict
	validation map[string]review.Check
	reviews    []reviewer.Request
	validated  [][]review.Finding
}

func (f *fakeReviewer) Review(_ context.Context, req reviewer.Request) (reviewer.Result, error) {
	f.reviews = append(f.reviews, req)
	return reviewer.Result{Lenses: req.Lenses, Reported: len(f.verdict.Blocking) + len(f.verdict.Escalated), Verdict: f.verdict}, nil
}

func (f *fakeReviewer) Validate(_ context.Context, _ reviewer.Request, fixed []review.Finding) (map[string]review.Check, error) {
	f.validated = append(f.validated, fixed)
	return f.validation, nil
}

var negativeNet = review.Finding{ID: "REL-001", Lens: "reliability", Severity: review.Critical, Claim: "A negative bonus makes the net negative.",
	Location: review.Location{Path: "reset.go", Line: 2}, Evidence: review.Deterministic, Causal: review.Introduced}

func withReviewer(h *harness, f *fakeReviewer) {
	h.svc.d.Reviewer = f
}

func lensed(o *Options) { o.LensesAuto = true }

func TestAScenarioWithoutBlockingFindingsIsCommittedAfterItsReview(t *testing.T) {
	h := newHarness(t, specBody)
	f := &fakeReviewer{verdict: review.Verdict{Info: []review.Finding{{ID: "RDB-001"}}}}
	withReviewer(h, f)
	h.agent.turns = happyScenario("SDD_0001_001", test1, "reset.go")
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green()}
	_, err := h.run(lensed)
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") {
		t.Fatalf("want the script to end at scenario 2, got %v", err)
	}
	if len(f.reviews) != 1 || len(f.reviews[0].Lenses) != 1 || f.reviews[0].Lenses[0] != review.LensReliability {
		t.Fatalf("a medium change gets the reliability lens: %+v", f.reviews)
	}
	if !strings.Contains(f.reviews[0].Diff, "+++ b/reset.go") || f.reviews[0].Marker != "SDD_0001_001" {
		t.Fatalf("the lens sees the scenario's diff:\n%s", f.reviews[0].Diff)
	}
	if !strings.Contains(h.p.read("specs/0001-reset/review/SDD_0001_001.json"), `"done": true`) {
		t.Fatal("the review is kept next to the specification")
	}
	if len(h.events.reviews) != 1 {
		t.Fatal("the developer sees the review")
	}
}

func TestABlockingFindingGetsOneCorrectionThenValidation(t *testing.T) {
	h := newHarness(t, specBody)
	f := &fakeReviewer{
		verdict:    review.Verdict{Blocking: []review.Finding{negativeNet}},
		validation: map[string]review.Check{"REL-001": {Status: "resolved", Reason: "clamped"}},
	}
	withReviewer(h, f)
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"),
		writes(map[string]string{"reset.go": "package m\n// implemented SDD_0001_001\n// clamped\n"}))
	// REFACTOR runs again after the correction: suite then gates.
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), green()}
	_, err := h.run(lensed)
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") {
		t.Fatalf("want the script to end at scenario 2, got %v", err)
	}
	correction := h.agent.prompts[2]
	if !strings.Contains(correction, "# Task: CORRECT") || !strings.Contains(correction, "REL-001") || !strings.Contains(correction, "at most 3 changed lines") {
		t.Fatalf("the correction prompt carries the findings and the budget:\n%s", correction)
	}
	if len(f.reviews) != 1 || len(f.validated) != 1 || f.validated[0][0].ID != "REL-001" {
		t.Fatalf("lenses once, validation of the corrected finding only: reviews=%d validated=%v", len(f.reviews), f.validated)
	}
	st := h.state(t)
	rec := st.Scenarios[0].Review
	if !st.Scenarios[0].Done || rec == nil || rec.Validation["REL-001"].Status != "resolved" || rec.Lines != 1 || rec.Budget != 3 {
		t.Fatalf("review record = %+v", rec)
	}
	if len(h.events.accepted) < 4 || h.events.accepted[3] != tdd.PhaseRefactor {
		t.Fatalf("REFACTOR judged the correction: %v", h.events.accepted)
	}
}

func TestARegressionGoesToTheDeveloper(t *testing.T) {
	h := newHarness(t, specBody)
	f := &fakeReviewer{
		verdict:    review.Verdict{Blocking: []review.Finding{negativeNet}},
		validation: map[string]review.Check{"REL-001": {Status: "regression", Reason: "still negative for refunds."}},
	}
	withReviewer(h, f)
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"), writes(map[string]string{"reset.go": "package m\n// try\n"}))
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), green()}
	h.prompter.answers = []string{"Stop the loop"}
	_, err := h.run(lensed)
	if !errors.Is(err, ErrReviewStopped) || !strings.Contains(h.prompter.questions[0].Text, "still negative for refunds.") {
		t.Fatalf("err=%v questions=%v", err, h.prompter.questions)
	}
}

func TestEscalatedFindingsAreTheDevelopersCall(t *testing.T) {
	h := newHarness(t, specBody)
	unclear := negativeNet
	unclear.ID, unclear.Causal = "REL-002", review.Unknown
	f := &fakeReviewer{verdict: review.Verdict{Escalated: []review.Finding{unclear}}}
	withReviewer(h, f)
	h.agent.turns = happyScenario("SDD_0001_001", test1, "reset.go")
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green()}
	h.prompter.answers = []string{"Keep it as a follow-up"}
	_, _ = h.run(lensed)
	st := h.state(t)
	if rec := st.Scenarios[0].Review; rec == nil || len(rec.FollowUps) != 1 || rec.FollowUps[0].ID != "REL-002" || !st.Scenarios[0].Done {
		t.Fatalf("review = %+v", rec)
	}
}

func TestAPendingReviewQuestionNeverRunsTheLensesTwice(t *testing.T) {
	h := newHarness(t, specBody)
	unclear := negativeNet
	unclear.Causal = review.Unknown
	f := &fakeReviewer{verdict: review.Verdict{Escalated: []review.Finding{unclear}}}
	withReviewer(h, f)
	h.prompter.nonTTY = true
	h.agent.turns = happyScenario("SDD_0001_001", test1, "reset.go")
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green()}
	_, err := h.run(lensed)
	var pending *clarify.PendingQuestionError
	if !errors.As(err, &pending) {
		t.Fatalf("want a pending question, got %v", err)
	}
	answerInFile(t, h.p, "1")
	_, err = h.run(func(o *Options) { o.Resume, o.LensesAuto = true, true })
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") || len(f.reviews) != 1 {
		t.Fatalf("resume continues the review without running the lenses again: err=%v reviews=%d", err, len(f.reviews))
	}
}

func TestAPassiveChangeGetsNoLens(t *testing.T) {
	h := newHarness(t, specBody)
	f := &fakeReviewer{}
	withReviewer(h, f)
	h.agent.turns = happyScenario("SDD_0001_001", test1, "reset.go")
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green()}
	_, _ = h.run(lensed, func(o *Options) { o.Risk = rules(t, []string{`^$`}, []string{`.*`}) })
	if len(f.reviews) != 0 || !h.state(t).Scenarios[0].Done {
		t.Fatalf("a passive scenario costs no review call: %d", len(f.reviews))
	}
}

func TestAnOverBudgetCorrectionNeedsTheDeveloper(t *testing.T) {
	h := newHarness(t, specBody)
	f := &fakeReviewer{verdict: review.Verdict{Blocking: []review.Finding{negativeNet}}}
	withReviewer(h, f)
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"),
		writes(map[string]string{"reset.go": "package m\n// a\n// b\n// c\n// d\n// e\n"}))
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green()}
	h.prompter.answers = []string{"Stop the loop"}
	_, err := h.run(lensed)
	if !errors.Is(err, ErrReviewStopped) || !strings.Contains(h.prompter.questions[0].Text, "over its budget of 3") {
		t.Fatalf("err=%v questions=%v", err, h.prompter.questions)
	}
}
