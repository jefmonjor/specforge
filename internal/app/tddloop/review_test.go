package tddloop

import (
	"context"
	"strings"
	"testing"

	"specforge/internal/domain/tdd"
)

type fakeVCS struct{ commits [][]string }

func (*fakeVCS) DefaultBase(context.Context, string) (string, error)  { return "main", nil }
func (*fakeVCS) Diff(context.Context, string, string) (string, error) { return "", nil }
func (*fakeVCS) Files(context.Context, string) ([]string, error)      { return nil, nil }
func (v *fakeVCS) Commit(_ context.Context, _, msg string, paths []string) (string, error) {
	v.commits = append(v.commits, append([]string{msg}, paths...))
	return "abc1234def", nil
}

func reviewed(h *harness) *fakeVCS {
	v := &fakeVCS{}
	h.svc.d.VCS = v
	return v
}

func TestReviewChangeGoesBackToGreenThenCommits(t *testing.T) {
	h := newHarness(t, specBody)
	vcs := reviewed(h)
	h.agent.turns = []reply{
		writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"}),
		writes(map[string]string{"reset.go": "package m\n// v1\n"}),
		writes(map[string]string{"reset.go": "package m\n// v2 with a struct\n"}),
	}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), green(), green()}
	h.prompter.answers = []string{"Use a struct instead of a map", "Accept"}

	_, err := h.run(func(o *Options) { o.Review, o.Commit = ReviewScenario, true })
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") {
		t.Fatalf("want the script to end at scenario 2, got %v", err)
	}
	if !strings.Contains(h.agent.prompts[2], "asked for a change: Use a struct instead of a map") {
		t.Fatalf("the GREEN prompt after the review must carry the change:\n%s", h.agent.prompts[2])
	}
	if len(vcs.commits) != 1 || !strings.HasPrefix(vcs.commits[0][0], "feat(SDD_0001_001): Request a link") {
		t.Fatalf("commits = %v", vcs.commits)
	}
	if paths := strings.Join(vcs.commits[0][1:], ","); !strings.Contains(paths, "reset.go") || !strings.Contains(paths, test1) {
		t.Fatalf("committed paths = %s", paths)
	}
	st := h.state(t)
	if st.Scenarios[0].Commit != "abc1234def" || len(st.Scenarios[0].Files) == 0 {
		t.Fatalf("scenario trace %+v", st.Scenarios[0])
	}
}

func TestReviewBackToRedRewritesTheTest(t *testing.T) {
	h := newHarness(t, specBody)
	reviewed(h)
	h.agent.turns = []reply{
		writes(map[string]string{test1: testFor("SDD_0001_001"), "reset.go": "package m\n"}),
		writes(map[string]string{"reset.go": "package m\n// v1\n"}),
		writes(map[string]string{test1: testFor("SDD_0001_001") + "// stricter\n"}),
	}
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), red(1)}
	h.prompter.answers = []string{"Back to RED: the test does not express the scenario"}

	_, err := h.run(func(o *Options) { o.Review = ReviewScenario })
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") {
		t.Fatalf("want the script to end in GREEN, got %v", err)
	}
	if !strings.Contains(h.agent.prompts[2], "# Task: RED") || !strings.Contains(h.agent.prompts[2], "sent it back to RED") {
		t.Fatalf("third prompt:\n%s", h.agent.prompts[2])
	}
	if got := h.events.accepted; len(got) != 4 || got[3] != tdd.PhaseRed {
		t.Fatalf("accepted = %v", got)
	}
}

func TestJumpRedoesOneScenarioFromGreen(t *testing.T) {
	h := newHarness(t, specBody)
	h.agent.turns = append(happyScenario("SDD_0001_001", test1, "reset.go"), happyScenario("SDD_0001_002", test2, "expiry.go")...)
	h.tests.outcomes = []tdd.Outcome{baseline(), red(1), green(), green(), red(1), green(), green()}
	if _, err := h.run(); err != nil {
		t.Fatal(err)
	}

	h.agent.turns = []reply{writes(map[string]string{"reset.go": "package m\n// redone\n"})}
	h.tests.outcomes = []tdd.Outcome{baseline(), green(), green()}
	st, err := h.run(func(o *Options) { o.Scenario, o.From = 1, tdd.PhaseGreen })
	if err != nil {
		t.Fatal(err)
	}
	if !st.Done() || !strings.Contains(h.agent.prompts[len(h.agent.prompts)-1], "# Task: GREEN") {
		t.Fatalf("state %+v", st)
	}
	if _, err := h.run(func(o *Options) { o.Scenario = 9 }); err == nil {
		t.Fatal("an unknown scenario must fail")
	}
}

func TestASatisfiedScenarioCommitsItsTest(t *testing.T) {
	h := newHarness(t, specBody)
	vcs := reviewed(h)
	h.agent.turns = []reply{writes(map[string]string{test1: testFor("SDD_0001_001")})}
	h.tests.outcomes = []tdd.Outcome{baseline(), green()}
	h.prompter.answers = []string{"Yes: mark the scenario as already satisfied"}
	_, err := h.run(func(o *Options) { o.Commit = true })
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") {
		t.Fatalf("want the script to end at scenario 2, got %v", err)
	}
	if len(vcs.commits) != 1 || !strings.HasPrefix(vcs.commits[0][0], "test(SDD_0001_001): Request a link") || vcs.commits[0][1] != test1 {
		t.Fatalf("commits = %v", vcs.commits)
	}
	if st := h.state(t); !st.Scenarios[0].Satisfied || st.Scenarios[0].Commit == "" {
		t.Fatalf("scenario %+v", st.Scenarios[0])
	}
}
