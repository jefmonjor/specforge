package tdd

import (
	"github.com/jefmonjor/specforge/v6/internal/domain/risk"
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

func sameFingerprint(old, cur ScenarioRef) bool { return old.Fingerprint == cur.Fingerprint }

func TestCarryMatchesScenariosByMarkerAfterAnAmendment(t *testing.T) {
	old := NewState("s.md", "0001", "h1", []ScenarioRef{
		{Index: 1, Title: "a", Marker: "M1", Fingerprint: "fa"},
		{Index: 2, Title: "b", Marker: "M2", Fingerprint: "fb"},
		{Index: 3, Title: "c", Marker: "M3", Fingerprint: "fc"},
	}, t0)
	for i := 0; i < 6; i++ { // finish a and b
		old.Advance(t0)
	}
	old.Scenarios[1].Review = &ReviewRecord{Reported: 2}
	old.Scenarios[1].Commit = "c2"

	// The amendment inserts a new scenario first, moves b before a and
	// edits c: markers, not positions, say which is which.
	amended := NewState("s.md", "0001", "h2", []ScenarioRef{
		{Index: 1, Title: "new", Marker: "M4", Fingerprint: "fn"},
		{Index: 2, Title: "b", Marker: "M2", Fingerprint: "fb"},
		{Index: 3, Title: "a", Marker: "M1", Fingerprint: "fa"},
		{Index: 4, Title: "c edited", Marker: "M3", Fingerprint: "fc2"},
	}, t0)
	pending := amended.Carry(old, t0, sameFingerprint)

	if len(pending) != 2 || pending[0] != "new" || pending[1] != "c edited" {
		t.Fatalf("pending = %v", pending)
	}
	b := amended.Scenarios[1]
	if !b.Done || b.Index != 2 || b.Commit != "c2" || b.Review == nil || b.Review.Reported != 2 {
		t.Fatalf("b keeps everything at its new position: %+v", b)
	}
	if amended.Scenarios[3].Amended {
		t.Fatal("c was never implemented: nothing to amend")
	}
	if amended.Current != 0 || amended.Phase != PhaseRed {
		t.Fatalf("must restart at the first pending scenario: %d %s", amended.Current, amended.Phase)
	}
}

func TestADoneScenarioWhoseContentChangedIsAmended(t *testing.T) {
	old := NewState("s.md", "0001", "h1", []ScenarioRef{{Index: 1, Title: "a", Marker: "M1", Fingerprint: "fa"}}, t0)
	for i := 0; i < 3; i++ {
		old.Advance(t0)
	}
	amended := NewState("s.md", "0001", "h2", []ScenarioRef{{Index: 1, Title: "a", Marker: "M1", Fingerprint: "fa2"}}, t0)
	if pending := amended.Carry(old, t0, sameFingerprint); len(pending) != 1 || !amended.Scenarios[0].Amended || amended.Scenarios[0].Done {
		t.Fatalf("pending=%v state=%+v", pending, amended.Scenarios[0])
	}
}

func TestAnUnfinishedUnchangedScenarioIsNotAmended(t *testing.T) {
	old := NewState("s.md", "0001", "h1", []ScenarioRef{{Index: 1, Title: "a", Marker: "M1", Fingerprint: "fa", Files: []string{"a_test.go"}}}, t0)
	amended := NewState("s.md", "0001", "h2", []ScenarioRef{{Index: 1, Title: "a", Marker: "M1", Fingerprint: "fa"}}, t0)
	if pending := amended.Carry(old, t0, sameFingerprint); len(pending) != 1 || amended.Scenarios[0].Amended {
		t.Fatalf("pending=%v %+v", pending, amended.Scenarios[0])
	}
}

func TestCheckpointsFollowTheMarker(t *testing.T) {
	s := NewState("s.md", "0001", "h", []ScenarioRef{{Index: 1, Marker: "M1"}, {Index: 2, Marker: "M2"}}, t0)
	s.Record("refactor", "accepted", "lint ✓", t0)
	c := s.Checkpoints[len(s.Checkpoints)-1]
	if c.Marker != "M1" || !c.About(ScenarioRef{Index: 7, Marker: "M1"}) || c.About(ScenarioRef{Index: 1, Marker: "M2"}) {
		t.Fatalf("%+v", c)
	}
	if !(Checkpoint{Scenario: 2}).About(ScenarioRef{Index: 2, Marker: "M2"}) {
		t.Fatal("an older checkpoint without marker matches by position")
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

func TestUpgradeReadsTheLastVersionOnly(t *testing.T) {
	for v, ok := range map[int]bool{1: false, 2: true, StateVersion: true, StateVersion + 1: false} {
		s := &State{Version: v}
		if got := s.Upgrade(); got != ok {
			t.Fatalf("Upgrade(v%d) = %v, want %v", v, got, ok)
		}
		if ok && s.Version != StateVersion {
			t.Fatalf("Upgrade(v%d) left version %d", v, s.Version)
		}
	}
}

func TestCarryKeepsTheBaseline(t *testing.T) {
	now := time.Now()
	old := NewState("s.md", "0001", "h1", []ScenarioRef{{Index: 1, Fingerprint: "a"}}, now)
	old.Baseline = &Baseline{Failures: []TestRef{{Name: "TestBroken"}}}
	fresh := NewState("s.md", "0001", "h2", []ScenarioRef{{Index: 1, Fingerprint: "b"}}, now)
	fresh.Carry(old, now, sameFingerprint)
	if !fresh.Baseline.Has(TestRef{Name: "TestBroken"}) {
		t.Fatal("an amended specification keeps the baseline of the branch")
	}
}

func TestRaiseKeepsTheStrictestRequest(t *testing.T) {
	s := twoScenarios()
	if s.Raise(risk.Passive, "no") {
		t.Fatal("asking for passive raises nothing")
	}
	if !s.Raise(risk.Medium, "touches the API") || !s.Raise(risk.High, "touches tokens") {
		t.Fatal("stricter requests are kept")
	}
	if s.Raise(risk.Medium, "less") {
		t.Fatal("a lower request never replaces a higher one")
	}
	if sc, _ := s.Scenario(); sc.RaisedTo != risk.High || sc.RaisedWhy != "touches tokens" {
		t.Fatalf("scenario = %+v", sc)
	}
	if err := s.Jump(1, PhaseRed, t0); err != nil || s.Scenarios[0].RaisedTo != "" {
		t.Fatalf("redoing a scenario forgets its risk: %+v %v", s.Scenarios[0], err)
	}
}

func TestAcceptedAndRefusedSurfaces(t *testing.T) {
	s := twoScenarios()
	s.Refuse("go.mod", "")
	s.Refuse("README.md", "h1")
	s.Refuse("README.md", "h2") // the first fingerprint is the one to go back to
	if got := s.Unreverted(map[string]string{"go.mod": "x", "README.md": "h2"}); len(got) != 2 {
		t.Fatalf("both still changed: %v", got)
	}
	if got := s.Unreverted(map[string]string{"README.md": "h1"}); len(got) != 0 || len(s.Refused) != 0 {
		t.Fatalf("both put back (go.mod no longer differs from the last commit): %v %v", got, s.Refused)
	}
	s.Refuse("docs/a.md", "")
	s.Accept("docs/a.md", "docs/a.md")
	if len(s.Surfaces) != 1 || len(s.Refused) != 0 {
		t.Fatalf("accepting a refused file clears it, once: %v %v", s.Surfaces, s.Refused)
	}
}
