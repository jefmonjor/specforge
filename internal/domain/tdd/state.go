// Package tdd holds the pure model of the Red → Green → Refactor loop: the
// persisted state machine, the outcome of a test run and the errors that
// stop the loop. It performs no I/O; time is passed in by the caller.
package tdd

import (
	"fmt"
	"slices"
	"time"

	"specforge/internal/domain/risk"
)

// StateVersion is bumped whenever State changes. Older states that only
// lack the newer fields are upgraded by Upgrade.
const StateVersion = 3

// oldestUpgradable is the first version Upgrade can read.
const oldestUpgradable = 2

// Phase is the step of the loop the current scenario is in.
type Phase string

const (
	PhaseRed       Phase = "RED"
	PhaseGreen     Phase = "GREEN"
	PhaseRefactor  Phase = "REFACTOR"
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
}

// Checkpoint records one completed step for the audit trail.
type Checkpoint struct {
	At       time.Time `json:"at"`
	Scenario int       `json:"scenario"`
	Phase    Phase     `json:"phase"`
	Step     string    `json:"step"`
	Status   string    `json:"status"`
	Details  string    `json:"details,omitempty"`
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

// Carry adopts the progress of old after the specification was amended:
// scenarios whose content is unchanged (same fingerprint) keep their done
// and satisfied flags, everything else starts from RED. It returns the
// titles of the scenarios that still need work.
func (s *State) Carry(old *State, now time.Time) []string {
	done := map[string]ScenarioRef{}
	for _, sc := range old.Scenarios {
		if sc.Done {
			done[sc.Fingerprint] = sc
		}
	}
	var pending []string
	for i := range s.Scenarios {
		if prev, ok := done[s.Scenarios[i].Fingerprint]; ok {
			s.Scenarios[i].Done = true
			s.Scenarios[i].Satisfied = prev.Satisfied
			s.Scenarios[i].Files = prev.Files
			s.Scenarios[i].Commit = prev.Commit
			continue
		}
		pending = append(pending, s.Scenarios[i].Title)
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
	case PhaseRefactor:
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
	case PhaseRed, PhaseGreen, PhaseRefactor:
	default:
		return fmt.Errorf("cannot start a scenario at %q: use red, green or refactor", phase)
	}
	s.Current = index - 1
	s.Scenarios[s.Current].Done = false
	s.Scenarios[s.Current].Satisfied = false
	s.Scenarios[s.Current].Commit = ""
	s.Scenarios[s.Current].Files = nil
	s.Scenarios[s.Current].Risk = nil
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

// Record appends a checkpoint for the current scenario and phase.
func (s *State) Record(step, status, details string, now time.Time) {
	s.Checkpoints = append(s.Checkpoints, Checkpoint{
		At:       now,
		Scenario: s.Current + 1,
		Phase:    s.Phase,
		Step:     step,
		Status:   status,
		Details:  details,
	})
	s.UpdatedAt = now
}
