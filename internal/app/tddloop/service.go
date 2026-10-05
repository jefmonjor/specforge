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
	"strings"
	"time"

	"github.com/jefmonjor/specforge/v6/internal/app/clarify"
	"github.com/jefmonjor/specforge/v6/internal/app/conversation"
	"github.com/jefmonjor/specforge/v6/internal/app/layout"
	"github.com/jefmonjor/specforge/v6/internal/app/reviewer"
	"github.com/jefmonjor/specforge/v6/internal/app/specs"
	"github.com/jefmonjor/specforge/v6/internal/domain/quality"
	"github.com/jefmonjor/specforge/v6/internal/domain/review"
	"github.com/jefmonjor/specforge/v6/internal/domain/risk"
	"github.com/jefmonjor/specforge/v6/internal/domain/spec"
	"github.com/jefmonjor/specforge/v6/internal/domain/stack"
	"github.com/jefmonjor/specforge/v6/internal/domain/tdd"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// Errors returned before the loop starts.
var (
	// ErrLoopInProgress: a loop for this spec is unfinished.
	ErrLoopInProgress = errors.New("a loop for this specification is in progress: continue it with --resume or start over with --restart")
	// ErrNothingToResume: --resume without saved state.
	ErrNothingToResume = errors.New("there is no saved loop for this specification to resume")
	// ErrPlanNotApproved: plan.md exists but is a draft or changed since
	// its approval. Without plan.md the loop runs from the specification.
	ErrPlanNotApproved = errors.New("the plan is not approved: review specs/<spec>/plan.md and run `specforge plan approve <spec>`")
	// ErrPlanOutdated: the approved plan does not match the specification's
	// scenarios: one is missing, or one it names was removed.
	ErrPlanOutdated = errors.New("the plan does not match the specification's scenarios: update it with `specforge plan <spec>` and approve it again")
	// ErrTooManyQuestions: the agent keeps asking within one phase.
	ErrTooManyQuestions = conversation.ErrTooManyQuestions
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
	// VCS measures each scenario's change and records it as a commit;
	// nil measures files without git and commits nothing.
	VCS ports.VCS
	// Reviewer runs the review lenses after REFACTOR; nil skips them.
	Reviewer Reviewer
	// Verifier checks the specification in a copy; nil skips it.
	Verifier Verifier
	// Scratch makes the sandboxes of a parallel loop; nil runs every
	// scenario in turn.
	Scratch ports.Scratch
	// Checkpoints save the working tree before every agent turn, so what
	// an agent destroys can be restored; nil saves none.
	Checkpoints ports.Checkpoints
	Events      Events
	Log         *slog.Logger
	Now         func() time.Time
}

// Reviewer runs review lenses over a change and validates a correction.
type Reviewer interface {
	Review(ctx context.Context, req reviewer.Request) (reviewer.Result, error)
	Validate(ctx context.Context, req reviewer.Request, fixed []review.Finding) (map[string]review.Check, error)
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
	// Scenario and From redo part of the loop: scenario Scenario (1-based)
	// starting at phase From (RED when empty).
	Scenario int
	From     tdd.Phase

	// Review "scenario" asks the developer to review every finished
	// scenario (gate R2); "risk" only those of medium or high risk; "off"
	// never.
	Review string
	// Risk classifies each scenario's change; MutationFrom is the lowest
	// tier that runs the mutation gate.
	Risk         risk.Rules
	MutationFrom risk.Tier
	// Commit records every finished scenario as one commit.
	Commit bool
	// Surfaces is what happens when the agent changes a file the approved
	// plan does not name: "ask" (default), "strict" or "off".
	Surfaces string
	// LensesAuto picks the review lenses by risk; otherwise Lenses are the
	// lenses of every scenario (none: no lens review).
	LensesAuto bool
	Lenses     []review.Lens
	// Blind doubles the lenses of high-risk scenarios.
	Blind bool
	// Parallel is how many scenarios with disjoint plan surfaces may run
	// side by side, each in its own sandbox (1: one at a time).
	Parallel int
	// Single runs only Scenario and stops: a scenario of a parallel loop.
	Single bool
	// Seed is the baseline a new loop starts from instead of running the
	// suite again.
	Seed *tdd.Baseline
	// Verify is when the independent verifier runs: "high" (scenarios of
	// high risk, the default), "always", "feature" (once, at the end) or
	// "off".
	Verify string

	MaxAttempts       int
	MaxClarifications int
	AgentTimeout      time.Duration
	TestTimeout       time.Duration
	// Models picks the agent's model for each phase.
	Models   ports.ModelFor
	AgentEnv []string
	Strict   bool

	// Legacy is the legacy repository of a rewrite, absolute: the agent
	// reads it as the reference of the behaviour and may never change it.
	// JavaRelease and ForbiddenImports are the targets of the rewrite.
	Legacy           string
	JavaRelease      int
	ForbiddenImports []string
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
	if o.Risk.High == nil {
		o.Risk = risk.DefaultRules()
	}
	if o.MutationFrom == "" {
		o.MutationFrom = risk.Passive
	}
	if o.Surfaces == "" {
		o.Surfaces = SurfacesAsk
	}
	if o.Verify == "" {
		o.Verify = VerifyHigh
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
	// markers are the scenarios' markers, by position; ledger keeps them
	// across amendments.
	markers []string
	ledger  spec.Ledger
	plan    string
	// surfaces are the files the approved plan allows the agent to edit.
	surfaces spec.Surfaces
	// lesson is the latest lesson the agent offered in this phase.
	lesson string
	st     *tdd.State
	// fresh is true when this invocation started a new loop: the baseline
	// is taken before its first RED.
	fresh bool
	// sequential are scenarios a parallel batch could not finish: they run
	// one at a time.
	sequential map[int]bool
	// checkpointFailed is true once a failed checkpoint was reported.
	checkpointFailed bool
}

// Run executes the loop until every scenario is done or a step stops it.
// The state is saved after every step, so --resume continues exactly there.
func (s *Service) Run(ctx context.Context, opts Options) (*tdd.State, error) {
	o := opts.withDefaults()
	r := &run{o: o, lay: layout.Layout{Root: o.Root}, specID: spec.IDFromPath(o.SpecPath)}

	if err := s.loadSpec(r); err != nil {
		return nil, err
	}
	if r.o.Scenario > 0 && !r.o.Restart {
		// Redoing a scenario continues the saved loop when there is one.
		r.o.Resume = s.d.Files.Exists(r.lay.State(r.o.SpecPath))
	}
	if err := s.loadState(r); err != nil {
		return nil, err
	}
	if r.o.Scenario > 0 {
		if err := s.jump(r); err != nil {
			return nil, err
		}
	}
	s.d.Events.Started(r.st, r.doc)
	if r.fresh {
		if err := s.startBaseline(ctx, r); err != nil {
			return r.st, errors.Join(err, s.save(r))
		}
	}

	for !r.st.Done() {
		if err := ctx.Err(); err != nil {
			return r.st, errors.Join(err, s.save(r))
		}
		if r.o.Single && r.st.Current != r.o.Scenario-1 {
			break // a parallel loop's scenario is finished
		}
		if batch := s.batch(r); len(batch) > 1 {
			if err := s.runBatch(ctx, r, batch); err != nil {
				return r.st, errors.Join(err, s.save(r))
			}
			continue
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
		case tdd.PhaseReview:
			err = s.reviewPhase(ctx, r)
		}
		if err != nil {
			return r.st, errors.Join(err, s.save(r))
		}
	}
	if r.o.Single {
		return r.st, s.save(r)
	}
	if err := s.verifyFeature(ctx, r); err != nil {
		return r.st, errors.Join(err, s.save(r))
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
	if r.ledger, err = specs.ReadLedger(s.d.Files, r.lay, r.o.SpecPath); err != nil {
		return err
	}
	byIndex := spec.Markers(r.ledger, r.specID, r.md, doc)
	r.markers = make([]string, len(doc.Scenarios))
	for i, sc := range doc.Scenarios {
		r.markers[i] = byIndex[sc.Index]
	}
	return s.loadPlan(r)
}

// loadPlan reads plan.md when there is one. A plan must be approved and
// must place every scenario: the developer reviewed it for a reason.
func (s *Service) loadPlan(r *run) error {
	path := r.lay.Plan(r.o.SpecPath)
	if !s.d.Files.Exists(path) {
		return nil
	}
	data, err := s.d.Files.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading the plan: %w", err)
	}
	content := string(data)
	if err := spec.Verify(content); err != nil {
		return fmt.Errorf("%w (%w)", ErrPlanNotApproved, err)
	}
	if issues := spec.Blocking(spec.LintPlan(content, r.markers)); len(issues) > 0 {
		return fmt.Errorf("%w: %s", ErrPlanOutdated, issues[0].Message)
	}
	r.plan = strings.TrimSpace(spec.StripSeal(content))
	r.surfaces = spec.PlanSurfaces(r.plan)
	return nil
}

func (s *Service) loadState(r *run) error {
	seal, _ := spec.ReadSeal(r.md)
	refs := make([]tdd.ScenarioRef, len(r.doc.Scenarios))
	// What an older SpecForge fingerprinted (the scenario's text alone),
	// so a loop it ran is still recognised.
	legacy := map[string]string{}
	for i, sc := range r.doc.Scenarios {
		refs[i] = tdd.ScenarioRef{Index: sc.Index, Title: sc.Title, Marker: r.markers[i], Fingerprint: spec.Fingerprint(r.md, sc)}
		legacy[r.markers[i]] = sc.Fingerprint()
	}
	fresh := tdd.NewState(r.lay.Rel(r.o.SpecPath), r.specID, seal.Hash, refs, s.d.Now())

	statePath := r.lay.State(r.o.SpecPath)
	saved, err := s.readState(statePath)
	switch {
	case err != nil && !errors.Is(err, errNoState):
		return err
	case r.o.Restart || (saved == nil && !r.o.Resume):
		r.st, r.fresh = fresh, true
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
		pending := fresh.Carry(saved, s.d.Now(), func(old, cur tdd.ScenarioRef) bool {
			return old.Fingerprint == cur.Fingerprint || old.Fingerprint == legacy[cur.Marker]
		})
		fresh.Record("spec-amended", "resumed", fmt.Sprintf("%d scenario(s) to (re)do", len(pending)), s.d.Now())
		s.d.Events.Amended(pending)
		if orphans := s.orphans(r); len(orphans) > 0 {
			fresh.Record("spec-amended", "orphaned tests", strings.Join(orphans, ", "), s.d.Now())
			s.d.Events.Orphaned(orphans)
		}
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

// jump moves to the scenario and phase the developer asked for. Starting
// after RED takes the current test files as the reviewed ones.
func (s *Service) jump(r *run) error {
	from := r.o.From
	if from == "" {
		from = tdd.PhaseRed
	}
	if err := r.st.Jump(r.o.Scenario, from, s.d.Now()); err != nil {
		return err
	}
	if from != tdd.PhaseRed {
		hashes, err := s.d.Workspace.HashFiles(r.o.Root, r.o.Profile.IsTestFile)
		if err != nil {
			return err
		}
		r.st.TestHashes = hashes
	}
	r.st.Record("jump", "requested", fmt.Sprintf("scenario %d from %s", r.o.Scenario, from), s.d.Now())
	return s.save(r)
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
	if !st.Upgrade() {
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

// orphans are test files that still carry the marker of a scenario the
// specification no longer has: nothing runs them for a scenario any more.
func (s *Service) orphans(r *run) []string {
	removed := r.ledger.RemovedMarkers()
	if len(removed) == 0 {
		return nil
	}
	hashes, err := s.d.Workspace.HashFiles(r.o.Root, r.o.Profile.IsTestFile)
	if err != nil {
		return nil
	}
	var out []string
	for path := range hashes {
		for _, m := range removed {
			if s.anyContains(r, []string{path}, m) {
				out = append(out, path+" ("+m+")")
			}
		}
	}
	return sortedStrings(out)
}
