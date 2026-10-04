package tddloop

import (
	"errors"
	"strings"
	"testing"

	"specforge/internal/app/clarify"
	"specforge/internal/domain/tdd"
)

// answerInFile replaces the first pending placeholder of questions.md.
func answerInFile(t *testing.T, p *project, answer string) {
	t.Helper()
	q := p.read("specs/0001-reset/questions.md")
	if !strings.Contains(q, "_awaiting an answer_") {
		t.Fatalf("no pending question in:\n%s", q)
	}
	p.write("specs/0001-reset/questions.md", strings.Replace(q, "_awaiting an answer_", answer, 1))
}

func TestAQuestionAfterWritingResumesTheSameTurn(t *testing.T) {
	h := newHarness(t, specBody)
	h.prompter.nonTTY = true
	// The agent writes the test, then asks: nobody is there.
	h.agent.turns = []reply{func(p *project, prompt string) string {
		p.write(test1, testFor("SDD_0001_001"))
		return ask("Which package holds reset?")(p, prompt)
	}}
	_, err := h.run()
	var pending *clarify.PendingQuestionError
	if !errors.As(err, &pending) {
		t.Fatalf("want a pending question, got %v", err)
	}

	// Resume without an answer: no agent call, same error, no duplicate.
	_, err = h.run(func(o *Options) { o.Resume = true })
	if !errors.As(err, &pending) || len(h.agent.prompts) != 1 {
		t.Fatalf("resume without an answer: err=%v calls=%d", err, len(h.agent.prompts))
	}
	if n := strings.Count(h.p.read("specs/0001-reset/questions.md"), "Which package holds reset?"); n != 1 {
		t.Fatalf("question written %d times", n)
	}

	// The developer answers in the file. The resumed turn starts with the
	// answer, and the test written before the question still counts.
	answerInFile(t, h.p, "2")
	h.agent.turns = []reply{
		func(*project, string) string { return done(test1) }, // nothing rewritten
		writes(map[string]string{"reset.go": "package m\n"}),
	}
	h.tests.outcomes = []tdd.Outcome{red(1), green(), green()}
	_, err = h.run(func(o *Options) { o.Resume = true })
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") {
		t.Fatalf("want the script to end at scenario 2, got %v", err)
	}
	if len(h.events.accepted) != 3 || h.events.accepted[0] != tdd.PhaseRed {
		t.Fatalf("accepted = %v, rejected = %v", h.events.accepted, h.events.rejected)
	}
	resumed := h.agent.prompts[1]
	if !strings.Contains(resumed, "The developer answered: **b**") || !strings.Contains(resumed, "**Answer:** b") {
		t.Fatalf("the resumed prompt must carry the answer:\n%s", resumed)
	}
	if !strings.Contains(h.p.read("specs/0001-reset/decisions.md"), "Which package holds reset?") {
		t.Fatal("the answer must be recorded as a decision")
	}
}

func TestAPrematureGreenWithoutTerminalIsDecidedOnResume(t *testing.T) {
	h := newHarness(t, specBody)
	h.prompter.nonTTY = true
	h.agent.turns = []reply{writes(map[string]string{test1: testFor("SDD_0001_001")})}
	h.tests.outcomes = []tdd.Outcome{green()}
	_, err := h.run()
	var pending *clarify.PendingQuestionError
	if !errors.As(err, &pending) || !errors.Is(err, tdd.ErrPrematureGreen) {
		t.Fatalf("want a pending premature-green question, got %v", err)
	}

	answerInFile(t, h.p, "1") // already satisfied
	h.agent.turns = happyScenario("SDD_0001_002", test2, "expiry.go")
	h.tests.outcomes = []tdd.Outcome{green(), red(1), green(), green()}
	st, err := h.run(func(o *Options) { o.Resume = true })
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !st.Done() || !st.Scenarios[0].Satisfied || h.events.satisfied != 1 {
		t.Fatalf("state %+v", st)
	}
	// The premature RED was verified again, not re-asked to the agent.
	if len(h.agent.prompts) != 3 {
		t.Fatalf("agent calls = %d, want 3", len(h.agent.prompts))
	}
}
