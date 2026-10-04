package tdd

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

func twoScenarios() *State {
	return NewState("specs/0001-x.md", "0001", "h", []ScenarioRef{
		{Index: 1, Title: "a", Marker: "SDD_0001_001"},
		{Index: 2, Title: "b", Marker: "SDD_0001_002"},
	}, t0)
}

func TestStateWalksEveryPhaseOfEveryScenario(t *testing.T) {
	s := twoScenarios()
	want := []struct {
		phase   Phase
		current int
	}{
		{PhaseGreen, 0}, {PhaseRefactor, 0}, {PhaseRed, 1},
		{PhaseGreen, 1}, {PhaseRefactor, 1}, {PhaseCompleted, 2},
	}
	for i, w := range want {
		s.Advance(t0)
		if s.Phase != w.phase || s.Current != w.current {
			t.Fatalf("step %d: got %s/%d, want %s/%d", i, s.Phase, s.Current, w.phase, w.current)
		}
	}
	if !s.Done() {
		t.Fatal("state must be done")
	}
	if _, ok := s.Scenario(); ok {
		t.Fatal("no current scenario once done")
	}
}

func TestRedFailureSurvivesIntoGreen(t *testing.T) {
	// Regression: the first GREEN prompt used to be sent without the RED
	// failure because advancing cleared it.
	s := twoScenarios()
	s.LastFailure = "expected 70, got 0"
	s.Advance(t0)
	if s.Phase != PhaseGreen || s.LastFailure != "expected 70, got 0" {
		t.Fatalf("got %s %q", s.Phase, s.LastFailure)
	}
	s.Advance(t0)
	if s.LastFailure != "" {
		t.Fatal("REFACTOR starts clean")
	}
}

func TestFailCountsAttemptsAndAdvanceResetsThem(t *testing.T) {
	s := twoScenarios()
	s.Fail("x", t0)
	s.Fail("y", t0)
	if s.Attempts != 2 || s.LastFailure != "y" {
		t.Fatalf("got %d %q", s.Attempts, s.LastFailure)
	}
	s.Advance(t0)
	if s.Attempts != 0 {
		t.Fatal("attempts are per phase")
	}
}

func TestMarkSatisfiedSkipsToNextScenario(t *testing.T) {
	s := twoScenarios()
	s.TestHashes = map[string]string{"a_test.go": "h"}
	s.MarkSatisfied(t0)
	if !s.Scenarios[0].Satisfied || !s.Scenarios[0].Done || s.Current != 1 || s.Phase != PhaseRed || s.TestHashes != nil {
		t.Fatalf("unexpected state: %+v", s)
	}
}

func TestAddFilesKeepsOrderWithoutDuplicates(t *testing.T) {
	s := twoScenarios()
	s.AddFiles("a", "b")
	s.AddFiles("b", "c")
	if got := s.FilesWritten; len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("FilesWritten = %v", got)
	}
}

func TestRedVerdict(t *testing.T) {
	cases := []struct {
		name string
		o    Outcome
		want RedVerdict
	}{
		{"compile error is not red", Outcome{Compiled: false, Failed: 1}, RedNotCompiled},
		{"nothing ran", Outcome{Compiled: true}, RedNothingRan},
		{"passing test is premature", Outcome{Compiled: true, Passed: 1}, RedPremature},
		{"failing assertion is red", Outcome{Compiled: true, Failed: 1}, RedValid},
	}
	for _, c := range cases {
		if got := c.o.Red(); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if (Outcome{Compiled: true}).Green() {
		t.Error("an empty run is not green")
	}
}

func TestCarryKeepsUnchangedScenariosDoneAfterAnAmendment(t *testing.T) {
	old := NewState("s.md", "0001", "h1", []ScenarioRef{
		{Index: 1, Title: "a", Fingerprint: "fa"},
		{Index: 2, Title: "b", Fingerprint: "fb"},
		{Index: 3, Title: "c", Fingerprint: "fc"},
	}, t0)
	for i := 0; i < 6; i++ { // finish a and b
		old.Advance(t0)
	}
	if old.Current != 2 {
		t.Fatalf("setup: current = %d", old.Current)
	}

	// The amendment inserts a new scenario first and edits c.
	amended := NewState("s.md", "0001", "h2", []ScenarioRef{
		{Index: 1, Title: "new", Fingerprint: "fn"},
		{Index: 2, Title: "a", Fingerprint: "fa"},
		{Index: 3, Title: "b", Fingerprint: "fb"},
		{Index: 4, Title: "c edited", Fingerprint: "fc2"},
	}, t0)
	pending := amended.Carry(old, t0)

	if len(pending) != 2 || pending[0] != "new" || pending[1] != "c edited" {
		t.Fatalf("pending = %v", pending)
	}
	if amended.Current != 0 || amended.Phase != PhaseRed {
		t.Fatalf("must restart at the first pending scenario: %d %s", amended.Current, amended.Phase)
	}
	for i := 0; i < 3; i++ {
		amended.Advance(t0)
	}
	if amended.Current != 3 {
		t.Fatalf("done scenarios must be skipped, current = %d", amended.Current)
	}
}

func TestNewStateWithoutScenariosIsDone(t *testing.T) {
	if s := NewState("s", "", "", nil, t0); !s.Done() || s.Phase != PhaseCompleted {
		t.Fatalf("state = %+v", s)
	}
}

func TestSendBackAndJump(t *testing.T) {
	now := time.Now()
	s := NewState("s.md", "0001", "h", []ScenarioRef{{Index: 1}, {Index: 2}}, now)
	s.Phase, s.TestHashes, s.LastFailure = PhaseRefactor, map[string]string{"a_test.go": "x"}, "boom"

	s.SendBack(PhaseGreen, "use a struct", now)
	if s.Phase != PhaseGreen || s.ReviewNote != "use a struct" || s.TestHashes == nil || s.LastFailure != "" {
		t.Fatalf("send back to GREEN: %+v", s)
	}
	s.SendBack(PhaseRed, "wrong test", now)
	if s.Phase != PhaseRed || s.TestHashes != nil {
		t.Fatalf("send back to RED: %+v", s)
	}

	s.Scenarios[0].Done, s.Scenarios[1].Done, s.Scenarios[0].Commit = true, true, "abc"
	s.Phase, s.Current = PhaseCompleted, 2
	if err := s.Jump(1, PhaseGreen, now); err != nil {
		t.Fatal(err)
	}
	if s.Done() || s.Current != 0 || s.Phase != PhaseGreen || s.Scenarios[0].Done || s.Scenarios[0].Commit != "" || !s.Scenarios[1].Done {
		t.Fatalf("jump: %+v", s)
	}
	if s.Jump(3, PhaseRed, now) == nil || s.Jump(1, PhaseCompleted, now) == nil {
		t.Fatal("invalid jumps must fail")
	}
	s.Advance(now) // GREEN → REFACTOR
	s.Advance(now) // REFACTOR → done (scenario 2 is still done)
	if !s.Done() {
		t.Fatalf("after redoing scenario 1 the loop is done: %+v", s)
	}
}
