// Package tdd holds the pure model of the Red → Green → Refactor loop: the
// persisted state machine, the outcome of a test run and the errors that
// stop the loop. It performs no I/O; time is passed in by the caller.
package tdd

import (
	"fmt"
	"slices"
	"time"

	"github.com/jefmonjor/specforge/v6/internal/domain/review"
	"github.com/jefmonjor/specforge/v6/internal/domain/risk"
	"github.com/jefmonjor/specforge/v6/internal/domain/verification"
)

// StateVersion is bumped whenever State changes. Older states that only
// lack the newer fields are upgraded by Upgrade.
const StateVersion = 3

// oldestUpgradable is the first version Upgrade can read.
const oldestUpgradable = 2

// Phase is the step of the loop the current scenario is in.
type Phase string

const (
	PhaseRed      Phase = "RED"
	PhaseGreen    Phase = "GREEN"
	PhaseRefactor Phase = "REFACTOR"
	// PhaseReview runs the review lenses over a scenario whose REFACTOR
	// passed, and its one correction.
	PhaseReview    Phase = "REVIEW"
	PhaseCompleted Phase = "COMPLETED"
)

// ScenarioRef identifies a scenario inside the state.
type ScenarioRef struct {
	Index       int    `json:"index"`
	Title       string `json:"title"`
	Marker      string `json:"marker"`
	Fingerprint string `json:"fingerprint"`
	// Done marks a scenario that went through the whole loop.
	Done bool `json:"done,omitempty"`
	// Satisfied marks a scenario the developer accepted as already
	// implemented when its test passed during RED.
	Satisfied bool `json:"satisfied,omitempty"`
	// Files and Commit trace a finished scenario: what changed for it and
	// the commit that recorded it ("" when nothing was committed).
	Files  []string `json:"files,omitempty"`
	Commit string   `json:"commit,omitempty"`
	// Risk is the latest assessment of the scenario's change. RaisedTo and
	// RaisedWhy keep the agent's request for more scrutiny, which every
	// later assessment applies again.
	Risk      *risk.Assessment `json:"risk,omitempty"`
	RaisedTo  risk.Tier        `json:"raised_to,omitempty"`
	RaisedWhy string           `json:"raised_why,omitempty"`
	// Review is the lens review of the scenario's change, kept so a resume
	// never runs the lenses twice.
	Review *ReviewRecord `json:"review,omitempty"`
	// Verify is the independent verification of the scenario.
	Verify *VerifyRecord `json:"verify,omitempty"`
	// Amended marks a scenario redone because the specification changed
	// it after it was implemented: its test exists and must be updated.
	Amended bool `json:"amended,omitempty"`
}

// VerifyRecord is the independent verification of one scenario and what
// came of it.
type VerifyRecord struct {
	Report verification.Report `json:"report"`
	// Skipped says why the verifier did not run.
	Skipped string `json:"skipped,omitempty"`
	// Corrected are the blockers sent to the one correction; Recheck is
	// the verification of those only, afterwards.
	Corrected []string             `json:"corrected,omitempty"`
	Recheck   *verification.Report `json:"recheck,omitempty"`
	// FollowUps are blockers the developer accepted as they are.
	FollowUps []string `json:"follow_ups,omitempty"`
	// Tests are the regression tests the developer added; TestsDecided is
	// set once they were offered.
	Tests        []string `json:"tests,omitempty"`
	TestsDecided bool     `json:"tests_decided,omitempty"`
	Done         bool     `json:"done,omitempty"`
}

// ReviewRecord is the lens review of one scenario and what came of it.
type ReviewRecord struct {
	Lenses   []review.Lens  `json:"lenses"`
	Reported int            `json:"reported"`
	Verdict  review.Verdict `json:"verdict"`
	// Correction: the findings sent to the one correction, its size in
	// lines and the budget it had.
	Corrected  []review.Finding        `json:"corrected,omitempty"`
	Lines      int                     `json:"correction_lines,omitempty"`
	Budget     int                     `json:"correction_budget,omitempty"`
	Validation map[string]review.Check `json:"validation,omitempty"`
	// FollowUps are findings left for later: pre-existing ones, and those
	// the developer accepted (escalated or regressed).
	FollowUps []review.Finding `json:"follow_ups,omitempty"`
	Done      bool             `json:"done,omitempty"`
}

// Checkpoint records one completed step for the audit trail.
type Checkpoint struct {
	At time.Time `json:"at"`
	// Scenario is the scenario's position when it was recorded; Marker
	// its identity, which positions do not change.
	Scenario int    `json:"scenario"`
	Marker   string `json:"marker,omitempty"`
	Phase    Phase  `json:"phase"`
	Step     string `json:"step"`
	Status   string `json:"status"`
	Details  string `json:"details,omitempty"`
}

// About reports whether the checkpoint is about scenario ref: by marker,
// or by position for a checkpoint recorded before markers were kept.
func (c Checkpoint) About(ref ScenarioRef) bool {
	if c.Marker != "" {
		return c.Marker == ref.Marker
	}
	return c.Scenario == ref.Index
}

// PendingKind says what a pending question interrupted.
type PendingKind string

const (
	// PendingAgent: the agent asked during its turn. The resumed turn
	// starts with the answer, from the same baselines.
	PendingAgent PendingKind = "agent"
	// PendingVerify: SpecForge asked while verifying the agent's work.
	// The resumed step verifies again without calling the agent.
	PendingVerify PendingKind = "verify"
	// PendingReview: the developer's review of a finished scenario.
	PendingReview PendingKind = "review"
)

// Pending is an interrupted step. The baselines are the files as they were
// before the interrupted agent turn, so what the agent wrote before asking
// still counts as written in that turn.
type Pending struct {
	Kind     PendingKind `json:"kind"`
	Phase    Phase       `json:"phase"`
	Scenario int         `json:"scenario"`
	Question string      `json:"question,omitempty"`
	Context  string      `json:"context,omitempty"`
	Options  []string    `json:"options,omitempty"`
	// Answer is filled once the developer answered.
	Answer string `json:"answer,omitempty"`
	// Claimed are the files the agent reported for the interrupted turn.
	Claimed []string `json:"claimed,omitempty"`
	// Workspace and Tests are the snapshots taken before that turn.
	Workspace map[string]string `json:"workspace,omitempty"`
	Tests     map[string]string `json:"tests,omitempty"`
}

// PendingFor returns the pending step if it belongs to the current phase
// and scenario.
func (s *State) PendingFor() *Pending {
	sc, ok := s.Scenario()
	if !ok || s.Pending == nil || s.Pending.Phase != s.Phase || s.Pending.Scenario != sc.Index {
		return nil
	}
	return s.Pending
}

// State is everything --resume needs to continue exactly where the loop
// stopped.
type State struct {
	Version   int           `json:"version"`
	SpecPath  string        `json:"spec_path"`
	SpecID    string        `json:"spec_id"`
	SpecHash  string        `json:"spec_hash"`
	Scenarios []ScenarioRef `json:"scenarios"`
	// Current is the 0-based index into Scenarios.
	Current int   `json:"current"`
	Phase   Phase `json:"phase"`
	// Attempts counts failed attempts in the current phase.
	Attempts int `json:"attempts"`
	// LastFailure is the output of the latest failing test run, fed to the
	// next prompt so the agent never works blind.
	LastFailure string `json:"last_failure,omitempty"`
	// TestHashes holds the SHA-256 of every test file once RED is accepted.
	// GREEN and REFACTOR must leave them untouched.
	TestHashes map[string]string `json:"test_hashes,omitempty"`
	// FilesWritten lists files changed while working on the current scenario.
	FilesWritten []string `json:"files_written,omitempty"`
	// ReviewNote is what the reviewer asked for when sending the scenario
	// back; the next prompt carries it.
	ReviewNote string `json:"review_note,omitempty"`
	// Pending is the step a question interrupted when nobody could answer
	// it. --resume answers it first and continues that same step.
	Pending *Pending `json:"pending,omitempty"`
	// Baseline lists the tests that already failed before the loop began.
	// Nil when the runner cannot name failures, or for an upgraded state.
	Baseline *Baseline `json:"baseline,omitempty"`
	// Surfaces are files outside the approved plan that the developer
	// accepted for this specification.
	Surfaces []string `json:"surfaces,omitempty"`
	// Refused maps each file outside the plan the developer refused to its
	// fingerprint before the agent changed it ("" when it matched the last
	// commit): the agent has to put it back.
	Refused     map[string]string `json:"refused,omitempty"`
	Checkpoints []Checkpoint      `json:"checkpoints"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// NewState starts a loop at RED of the first scenario not yet done.
func NewState(specPath, specID, specHash string, scenarios []ScenarioRef, now time.Time) *State {
	s := &State{
		Version:     StateVersion,
		SpecPath:    specPath,
		SpecID:      specID,
		SpecHash:    specHash,
		Scenarios:   scenarios,
		Current:     -1,
		Checkpoints: []Checkpoint{},
		UpdatedAt:   now,
	}
	s.seek()
	return s
}

// Carry adopts the progress of old after the specification was amended.
// Scenarios are matched by marker, which moving, inserting or removing
// others does not change. A done scenario whose content is unchanged
// keeps everything (done, files, commit, risk, review, verification); one
// whose content changed is marked Amended and starts from RED. same
// compares an old and a new fingerprint (a fingerprint from an older
// SpecForge included). It returns the titles of the scenarios that still
// need work.
func (s *State) Carry(old *State, now time.Time, same func(old, cur ScenarioRef) bool) []string {
	prev := map[string]ScenarioRef{}
	for _, sc := range old.Scenarios {
		prev[sc.Marker] = sc
	}
	var pending []string
	for i := range s.Scenarios {
		cur := &s.Scenarios[i]
		p, ok := prev[cur.Marker]
		switch {
		case ok && p.Done && same(p, *cur):
			p.Index, p.Title, p.Fingerprint = cur.Index, cur.Title, cur.Fingerprint
			*cur = p
			continue
		case ok && (p.Done || len(p.Files) > 0) && !same(p, *cur):
			cur.Amended = true
		}
		pending = append(pending, cur.Title)
	}
	s.Checkpoints = append(old.Checkpoints, s.Checkpoints...)
	s.Baseline = old.Baseline
	s.Surfaces, s.Refused = old.Surfaces, old.Refused
	s.Current = -1
	s.seek()
	s.UpdatedAt = now
	return pending
}

// Upgrade brings a state written by an older SpecForge to StateVersion.
// The newer fields start empty (no baseline): nothing is invented. It
// reports false for a state it cannot read.
func (s *State) Upgrade() bool {
	if s.Version < oldestUpgradable || s.Version > StateVersion {
		return false
	}
	s.Version = StateVersion
	return true
}

// Done reports whether every scenario went through the loop.
func (s *State) Done() bool {
	return s.Phase == PhaseCompleted || s.Current < 0 || s.Current >= len(s.Scenarios)
}

// Scenario returns the current scenario, or false when the loop is done.
func (s *State) Scenario() (ScenarioRef, bool) {
	if s.Done() {
		return ScenarioRef{}, false
	}
	return s.Scenarios[s.Current], true
}

// Advance moves to the next phase: RED → GREEN → REFACTOR → next scenario's
// RED, and COMPLETED after the last one. It clears per-phase bookkeeping but
// keeps LastFailure when entering GREEN, because the RED failure is exactly
// what the first GREEN prompt needs.
func (s *State) Advance(now time.Time) {
	s.Attempts = 0
	s.Pending = nil
	s.ReviewNote = ""
	switch s.Phase {
	case PhaseRed:
		s.Phase = PhaseGreen
	case PhaseGreen:
		s.Phase = PhaseRefactor
		s.LastFailure = ""
	case PhaseRefactor, PhaseReview:
		s.nextScenario()
	}
	s.UpdatedAt = now
}

// MarkSatisfied closes the current scenario without GREEN and REFACTOR,
// after the developer confirmed the behaviour already exists.
func (s *State) MarkSatisfied(now time.Time) {
	if s.Done() {
		return
	}
	s.Scenarios[s.Current].Satisfied = true
	s.Attempts = 0
	s.Pending = nil
	s.nextScenario()
	s.UpdatedAt = now
}

// SendBack returns the current scenario to RED or GREEN after a review, with
// the reviewer's note for the next prompt. Going back to RED drops the test
// fingerprints: the test itself is what has to change.
func (s *State) SendBack(phase Phase, note string, now time.Time) {
	s.Phase = phase
	s.Attempts = 0
	s.Pending = nil
	s.ReviewNote = note
	s.LastFailure = ""
	if phase == PhaseRed {
		s.TestHashes = nil
	}
	if !s.Done() {
		// A changed scenario is reviewed and verified again.
		s.Scenarios[s.Current].Review, s.Scenarios[s.Current].Verify = nil, nil
	}
	s.UpdatedAt = now
}

// Reopen returns a corrected scenario to REFACTOR, keeping its review, so
// the suite and the gates judge the correction before it is validated.
func (s *State) Reopen(now time.Time) {
	s.Phase = PhaseRefactor
	s.Attempts = 0
	s.Pending = nil
	s.UpdatedAt = now
}

// Jump moves to scenario index (1-based) at phase, for a developer who
// wants to redo part of the loop. The scenario is no longer done; the
// others keep their state.
func (s *State) Jump(index int, phase Phase, now time.Time) error {
	if index < 1 || index > len(s.Scenarios) {
		return fmt.Errorf("there is no scenario %d (the specification has %d)", index, len(s.Scenarios))
	}
	switch phase {
	case PhaseRed, PhaseGreen, PhaseRefactor, PhaseReview:
	default:
		return fmt.Errorf("cannot start a scenario at %q: use red, green, refactor or review", phase)
	}
	s.Current = index - 1
	s.Scenarios[s.Current].Done = false
	s.Scenarios[s.Current].Satisfied = false
	s.Scenarios[s.Current].Commit = ""
	s.Scenarios[s.Current].Files = nil
	s.Scenarios[s.Current].Risk = nil
	s.Scenarios[s.Current].Review = nil
	s.Scenarios[s.Current].Verify = nil
	s.Scenarios[s.Current].RaisedTo, s.Scenarios[s.Current].RaisedWhy = "", ""
	s.Phase = phase
	s.Attempts = 0
	s.Pending = nil
	s.ReviewNote = ""
	s.LastFailure = ""
	s.FilesWritten = nil
	if phase == PhaseRed {
		s.TestHashes = nil
	}
	s.UpdatedAt = now
	return nil
}

func (s *State) nextScenario() {
	s.Scenarios[s.Current].Done = true
	s.seek()
}

// seek moves to RED of the first scenario not done yet, or to COMPLETED.
func (s *State) seek() {
	s.LastFailure = ""
	s.ReviewNote = ""
	s.TestHashes = nil
	s.FilesWritten = nil
	s.Attempts = 0
	for i := range s.Scenarios {
		if !s.Scenarios[i].Done {
			s.Current = i
			s.Phase = PhaseRed
			return
		}
	}
	s.Current = len(s.Scenarios)
	s.Phase = PhaseCompleted
}

// Raise records the agent's request for more scrutiny of the current
// scenario. Only a stricter tier than the one already requested counts.
func (s *State) Raise(to risk.Tier, why string) bool {
	if s.Done() || !to.AtLeast(risk.Medium) {
		return false
	}
	sc := &s.Scenarios[s.Current]
	if sc.RaisedTo != "" && sc.RaisedTo.AtLeast(to) {
		return false
	}
	sc.RaisedTo, sc.RaisedWhy = to, why
	return true
}

// Accept adds files outside the plan the developer accepted.
func (s *State) Accept(paths ...string) {
	for _, p := range paths {
		if !slices.Contains(s.Surfaces, p) {
			s.Surfaces = append(s.Surfaces, p)
		}
		delete(s.Refused, p)
	}
	slices.Sort(s.Surfaces)
}

// Refuse records a file outside the plan the developer refused, with its
// fingerprint before the change. A file refused twice keeps the first one:
// that is the content to go back to.
func (s *State) Refuse(path, before string) {
	if s.Refused == nil {
		s.Refused = map[string]string{}
	}
	if _, ok := s.Refused[path]; !ok {
		s.Refused[path] = before
	}
}

// Unreverted checks the refused files against now (the fingerprints of the
// files that differ from the last commit): those back to their content
// leave the list; the others are returned, sorted.
func (s *State) Unreverted(now map[string]string) []string {
	var out []string
	for p, before := range s.Refused {
		if now[p] == before {
			delete(s.Refused, p)
			continue
		}
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// Fail records a failed attempt in the current phase.
func (s *State) Fail(output string, now time.Time) {
	s.Attempts++
	s.LastFailure = output
	s.UpdatedAt = now
}

// AddFiles records files changed for the current scenario, without
// duplicates and keeping order.
func (s *State) AddFiles(paths ...string) {
	seen := make(map[string]bool, len(s.FilesWritten))
	for _, p := range s.FilesWritten {
		seen[p] = true
	}
	for _, p := range paths {
		if !seen[p] {
			seen[p] = true
			s.FilesWritten = append(s.FilesWritten, p)
		}
	}
}

// Adopt takes over scenario i as another state left it (a scenario run on
// its own, in a sandbox): everything it learned and its checkpoints, but
// not done nor committed until this state closes it.
func (s *State) Adopt(other *State, i int) {
	ref := other.Scenarios[i]
	ref.Done, ref.Commit = false, ""
	s.Scenarios[i] = ref
	for _, c := range other.Checkpoints {
		if c.About(ref) {
			s.Checkpoints = append(s.Checkpoints, c)
		}
	}
}

// Record appends a checkpoint for the current scenario and phase.
func (s *State) Record(step, status, details string, now time.Time) {
	c := Checkpoint{
		At:       now,
		Scenario: s.Current + 1,
		Phase:    s.Phase,
		Step:     step,
		Status:   status,
		Details:  details,
	}
	if sc, ok := s.Scenario(); ok {
		c.Marker = sc.Marker
	}
	s.Checkpoints = append(s.Checkpoints, c)
	s.UpdatedAt = now
}
