// Package tddloop is the Red → Green → Refactor use case.
//
// The agent writes code; this package decides whether it ships. Every step
// is verified against reality instead of the agent's word: the files that
// changed (workspace snapshots), the test verdict (the runner's report),
// the integrity of the tests (hashes taken after RED) and the quality gates.
// When the agent lacks information it must ask, and the question reaches
// the developer through package clarify.
package tddloop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"specforge/internal/app/clarify"
	"specforge/internal/app/layout"
	"specforge/internal/domain/quality"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/stack"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// Errors returned before the loop starts.
var (
	// ErrLoopInProgress: a loop for this spec is unfinished.
	ErrLoopInProgress = errors.New("a loop for this specification is in progress: continue it with --resume or start over with --restart")
	// ErrNothingToResume: --resume without saved state.
	ErrNothingToResume = errors.New("there is no saved loop for this specification to resume")
	// ErrTooManyQuestions: the agent keeps asking within one phase.
	ErrTooManyQuestions = errors.New("the agent asked too many questions in a single phase; the specification probably needs clarification")
)

// GatesError reports quality gates that still block after the agent's
// attempts, or that cannot run at all.
type GatesError struct {
	Report       quality.Report
	Strict       bool
	SuiteFailure string
}

func (e *GatesError) Error() string {
	if e.SuiteFailure != "" {
		return "the full test suite fails after REFACTOR"
	}
	return fmt.Sprintf("%d quality gate(s) block the scenario", len(e.Report.Blocking(e.Strict)))
}

// Deps are the collaborators of the loop.
type Deps struct {
	Agent     ports.Agent
	Tests     ports.TestRunner
	Gates     []ports.Gate
	Workspace ports.Workspace
	Files     ports.Files
	Asker     *clarify.Asker
	Events    Events
	Log       *slog.Logger
	Now       func() time.Time
}

// Options select the specification and tune the loop.
type Options struct {
	// Root and SpecPath are absolute.
	Root     string
	SpecPath string
	Profile  stack.Profile
	Language string

	Resume  bool
	Restart bool

	MaxAttempts       int
	MaxClarifications int
	AgentTimeout      time.Duration
	TestTimeout       time.Duration
	Model             string
	AgentEnv          []string
	Strict            bool
}

func (o Options) withDefaults() Options {
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 3
	}
	if o.MaxClarifications <= 0 {
		o.MaxClarifications = 5
	}
	if o.AgentTimeout <= 0 {
		o.AgentTimeout = 20 * time.Minute
	}
	if o.TestTimeout <= 0 {
		o.TestTimeout = 10 * time.Minute
	}
	if o.Language == "" {
		o.Language = "en"
	}
	return o
}

// Service runs the loop.
type Service struct {
	d Deps
}

// New returns a loop service.
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Events == nil {
		d.Events = NopEvents{}
	}
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{d: d}
}

// run carries everything one invocation needs.
type run struct {
	o      Options
	lay    layout.Layout
	doc    *spec.Document
	md     string
	specID string
	st     *tdd.State
}

// Run executes the loop until every scenario is done or a step stops it.
// The state is saved after every step, so --resume continues exactly there.
func (s *Service) Run(ctx context.Context, opts Options) (*tdd.State, error) {
	o := opts.withDefaults()
	r := &run{o: o, lay: layout.Layout{Root: o.Root}, specID: spec.IDFromPath(o.SpecPath)}

	if err := s.loadSpec(r); err != nil {
		return nil, err
	}
	if err := s.loadState(r); err != nil {
		return nil, err
	}
	s.d.Events.Started(r.st, r.doc)

	for !r.st.Done() {
		if err := ctx.Err(); err != nil {
			return r.st, errors.Join(err, s.save(r))
		}
		sc, _ := r.st.Scenario()
		s.d.Events.Phase(r.st, sc)

		var err error
		switch r.st.Phase {
		case tdd.PhaseRed:
			err = s.red(ctx, r)
		case tdd.PhaseGreen:
			err = s.green(ctx, r)
		case tdd.PhaseRefactor:
			err = s.refactor(ctx, r)
		}
		if err != nil {
			return r.st, errors.Join(err, s.save(r))
		}
	}
	s.d.Events.Finished(r.st)
	return r.st, s.save(r)
}

func (s *Service) loadSpec(r *run) error {
	data, err := s.d.Files.ReadFile(r.o.SpecPath)
	if err != nil {
		return fmt.Errorf("reading the specification: %w", err)
	}
	r.md = string(data)
	if err := spec.Verify(r.md); err != nil {
		return err
	}
	if qs := spec.OpenQuestions(r.md); len(qs) > 0 {
		return &tdd.OpenQuestionsError{Questions: qs}
	}
	doc, err := spec.Parse(r.md, spec.ParseOptions{Languages: []string{r.o.Language}})
	if err != nil {
		return err
	}
	r.doc = doc
	return nil
}

func (s *Service) loadState(r *run) error {
	seal, _ := spec.ReadSeal(r.md)
	refs := make([]tdd.ScenarioRef, len(r.doc.Scenarios))
	for i, sc := range r.doc.Scenarios {
		refs[i] = tdd.ScenarioRef{Index: sc.Index, Title: sc.Title, Marker: spec.Marker(r.specID, sc.Index), Fingerprint: sc.Fingerprint()}
	}
	fresh := tdd.NewState(r.lay.Rel(r.o.SpecPath), r.specID, seal.Hash, refs, s.d.Now())

	statePath := r.lay.State(r.o.SpecPath)
	saved, err := s.readState(statePath)
	switch {
	case err != nil && !errors.Is(err, errNoState):
		return err
	case r.o.Restart || (saved == nil && !r.o.Resume):
		r.st = fresh
		return s.save(r)
	case saved == nil:
		return ErrNothingToResume
	case !r.o.Resume && !saved.Done():
		return ErrLoopInProgress
	case saved.SpecPath != fresh.SpecPath:
		return fmt.Errorf("%w (%s)", tdd.ErrStateMismatch, saved.SpecPath)
	case saved.SpecHash != seal.Hash:
		// An amended specification: keep the scenarios that did not change
		// and redo the rest, whether or not the previous loop had finished.
		pending := fresh.Carry(saved, s.d.Now())
		fresh.Record("spec-amended", "resumed", fmt.Sprintf("%d scenario(s) to (re)do", len(pending)), s.d.Now())
		s.d.Events.Amended(pending)
		r.st = fresh
		return s.save(r)
	case !r.o.Resume:
		// Finished and unchanged: report it instead of redoing everything;
		// --restart runs it again on purpose.
		r.st = saved
		return nil
	default:
		saved.Attempts = 0 // a resume is a deliberate new try
		r.st = saved
		return nil
	}
}

var errNoState = errors.New("no state")

func (s *Service) readState(path string) (*tdd.State, error) {
	if !s.d.Files.Exists(path) {
		return nil, errNoState
	}
	data, err := s.d.Files.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading loop state: %w", err)
	}
	var st tdd.State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("loop state %s is corrupt: %w", path, err)
	}
	if st.Version != tdd.StateVersion {
		return nil, fmt.Errorf("loop state %s was written by an incompatible version: start over with --restart", path)
	}
	return &st, nil
}

func (s *Service) save(r *run) error {
	if r.st == nil {
		return nil
	}
	data, err := json.MarshalIndent(r.st, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding loop state: %w", err)
	}
	if err := s.d.Files.WriteFile(r.lay.State(r.o.SpecPath), data); err != nil {
		return fmt.Errorf("saving loop state: %w", err)
	}
	return nil
}
