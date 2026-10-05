package tddloop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"specforge/internal/app/prompts"
	"specforge/internal/app/reviewer"
	"specforge/internal/domain/change"
	"specforge/internal/domain/review"
	"specforge/internal/domain/risk"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// ErrReviewStopped: the developer stopped the loop on a review finding.
var ErrReviewStopped = errors.New("the developer stopped the loop on a review finding")

// afterRefactor sends a scenario whose REFACTOR passed to the lens review
// when there is a reviewer, or straight to the developer's review and the
// commit.
func (s *Service) afterRefactor(ctx context.Context, r *run, sc tdd.ScenarioRef, gates string) error {
	if s.d.Reviewer == nil && s.d.Verifier == nil {
		return s.close(ctx, r, sc, gates)
	}
	r.st.Phase = tdd.PhaseReview
	r.st.Record("review", "started", gates, s.d.Now())
	return s.save(r)
}

// reviewPhase runs the lens review of the current scenario, step by step,
// keeping each step's outcome so --resume continues where it stopped: the
// lenses run once, the escalated findings go to the developer, the
// blocking ones get one correction (judged again by REFACTOR), and the
// correction is validated on the corrected findings only.
func (s *Service) reviewPhase(ctx context.Context, r *run) error {
	sc, _ := r.st.Scenario()
	if p := r.st.PendingFor(); p != nil && p.Kind == tdd.PendingReview {
		return s.close(ctx, r, sc, p.Context)
	}
	ref := &r.st.Scenarios[r.st.Current]
	if ref.Review == nil && s.d.Reviewer == nil {
		ref.Review = &tdd.ReviewRecord{Done: true}
	}
	settled := false
	if ref.Review == nil {
		rec, err := s.runLenses(ctx, r, sc)
		if err != nil {
			return err
		}
		ref.Review, settled = rec, rec.Done
		if err := s.keep(r, sc, rec); err != nil {
			return err
		}
	}
	rec := ref.Review
	if !rec.Done {
		if err := s.settleEscalated(ctx, r, sc, rec); err != nil {
			return err
		}
		switch {
		case len(rec.Verdict.Blocking) > 0 && rec.Corrected == nil:
			return s.correct(ctx, r, sc, rec)
		case rec.Corrected != nil && rec.Validation == nil:
			if err := s.validate(ctx, r, sc, rec); err != nil {
				return err
			}
		}
		rec.Done, settled = true, true
		if err := s.keep(r, sc, rec); err != nil {
			return err
		}
	}
	if settled {
		s.d.Events.Reviewed(sc, *rec)
	}
	if proceed, err := s.verifyStep(ctx, r, sc); err != nil || !proceed {
		return err
	}
	return s.close(ctx, r, sc, lastGates(r.st, sc.Index))
}

// runLenses picks the lenses for the scenario's risk and runs them.
func (s *Service) runLenses(ctx context.Context, r *run, sc tdd.ScenarioRef) (*tdd.ReviewRecord, error) {
	a, err := s.assess(ctx, r, true)
	if err != nil {
		return nil, err
	}
	lenses := r.o.Lenses
	if r.o.LensesAuto {
		lenses = review.Select(a.Tier, r.st.FilesWritten, r.o.Risk)
	}
	if len(lenses) == 0 {
		r.st.Record("review", "no lens", "risk "+string(a.Tier), s.d.Now())
		return &tdd.ReviewRecord{Done: true}, nil
	}
	diff, err := s.patch(ctx, r)
	if err != nil {
		return nil, err
	}
	req := s.reviewRequest(r, sc, lenses, diff)
	req.Blind = r.o.Blind && a.Tier == risk.High
	res, err := s.d.Reviewer.Review(ctx, req)
	if err != nil {
		return nil, err
	}
	rec := &tdd.ReviewRecord{Lenses: lenses, Reported: res.Reported, Verdict: res.Verdict, FollowUps: res.Verdict.FollowUps}
	rec.Done = len(rec.Verdict.Blocking) == 0 && len(rec.Verdict.Escalated) == 0
	r.st.Record("review", "lenses", fmt.Sprintf("%d lens(es), %d reported, %d blocking, %d discarded",
		len(lenses), res.Reported, len(res.Verdict.Blocking), len(res.Verdict.Discarded)), s.d.Now())
	return rec, nil
}

// settleEscalated asks the developer about each severe finding whose cause
// or evidence the lens could not establish.
func (s *Service) settleEscalated(ctx context.Context, r *run, sc tdd.ScenarioRef, rec *tdd.ReviewRecord) error {
	for len(rec.Verdict.Escalated) > 0 {
		f := rec.Verdict.Escalated[0]
		picked, err := s.choose(ctx, r, sc, OriginReview, "escalated", f.ID, f.Location.Path, f.Location.Line, f.Claim)
		if err != nil {
			return err
		}
		switch picked {
		case 0:
			rec.FollowUps = append(rec.FollowUps, f)
		case 1:
			rec.Verdict.Blocking = append(rec.Verdict.Blocking, f)
		default:
			return fmt.Errorf("%w (%s)", ErrReviewStopped, f.ID)
		}
		rec.Verdict.Escalated = rec.Verdict.Escalated[1:]
		if err := s.keep(r, sc, rec); err != nil {
			return err
		}
	}
	return nil
}

// correct runs the one correction of the blocking findings and sends the
// scenario back to REFACTOR, where the tests and the gates judge it before
// it is validated.
func (s *Service) correct(ctx context.Context, r *run, sc tdd.ScenarioRef, rec *tdd.ReviewRecord) error {
	size, budget, err := s.correction(ctx, r, sc, rec.Verdict.Blocking)
	if err != nil {
		return err
	}
	rec.Corrected, rec.Lines, rec.Budget = rec.Verdict.Blocking, size, budget
	r.st.Record("review", "corrected", fmt.Sprintf("%d finding(s), %d line(s) of %d", len(rec.Corrected), size, budget), s.d.Now())
	if err := s.keep(r, sc, rec); err != nil {
		return err
	}
	return s.reopen(ctx, r, sc, size, budget)
}

// correction runs the one correction turn for findings: tests untouched,
// files within the plan, its size measured line by line. It returns the
// size and the budget it had.
func (s *Service) correction(ctx context.Context, r *run, sc tdd.ScenarioRef, findings []review.Finding) (size, budget int, err error) {
	lines := 0
	if sc.Risk != nil {
		lines = sc.Risk.Lines
	}
	budget = review.CorrectionBudget(lines)
	before := s.contents(r, r.st.FilesWritten)
	data := s.promptData(r, sc)
	data.TestFiles = s.specTests(r, r.st.FilesWritten)
	data.Findings, data.Budget = findings, budget
	t, err := s.turnOf(ctx, r, sc, prompts.Correct, data)
	if err != nil {
		return 0, 0, err
	}
	if err := s.checkTampering(r); err != nil {
		return 0, 0, err
	}
	if _, err := s.checkSurfaces(ctx, r, sc, t); err != nil {
		s.pendVerify(r, sc, t, nil, err)
		return 0, 0, err
	}
	changed := notRefused(r.st, t.Changed)
	for _, p := range changed {
		now, _ := s.d.Files.ReadFile(r.lay.Abs(p))
		size += change.Between(p, before[p], now).Lines()
	}
	r.st.AddFiles(changed...)
	return size, budget, nil
}

// reopen asks the developer about a correction over its budget, then
// returns the scenario to REFACTOR.
func (s *Service) reopen(ctx context.Context, r *run, sc tdd.ScenarioRef, size, budget int) error {
	if size > budget {
		if err := s.acceptOverBudget(ctx, r, sc, size, budget); err != nil {
			return err
		}
	}
	r.st.Reopen(s.d.Now())
	return s.save(r)
}

func (s *Service) acceptOverBudget(ctx context.Context, r *run, sc tdd.ScenarioRef, size, budget int) error {
	picked, err := s.choose(ctx, r, sc, OriginReview, "budget", size, budget)
	if err != nil {
		return err
	}
	if picked != 0 {
		return fmt.Errorf("%w: the correction changed %d lines, over its budget of %d", ErrReviewStopped, size, budget)
	}
	return nil
}

// validate checks the corrected findings on the code as it is now; the
// lenses do not run again. A regression goes to the developer: there is
// never a second automatic correction.
func (s *Service) validate(ctx context.Context, r *run, sc tdd.ScenarioRef, rec *tdd.ReviewRecord) error {
	diff, err := s.patch(ctx, r)
	if err != nil {
		return err
	}
	results, err := s.d.Reviewer.Validate(ctx, s.reviewRequest(r, sc, nil, diff), rec.Corrected)
	if err != nil {
		return err
	}
	rec.Validation = results
	if err := s.keep(r, sc, rec); err != nil {
		return err
	}
	for _, f := range rec.Corrected {
		v := rec.Validation[f.ID]
		if v.Status == "resolved" || slices.ContainsFunc(rec.FollowUps, func(x review.Finding) bool { return x.ID == f.ID }) {
			continue
		}
		picked, err := s.choose(ctx, r, sc, OriginReview, "regression", f.ID, v.Reason)
		if err != nil {
			return err
		}
		if picked != 0 {
			return fmt.Errorf("%w (%s not resolved)", ErrReviewStopped, f.ID)
		}
		rec.FollowUps = append(rec.FollowUps, f)
		if err := s.keep(r, sc, rec); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) reviewRequest(r *run, sc tdd.ScenarioRef, lenses []review.Lens, diff string) reviewer.Request {
	source := r.docScenario(sc).Source
	return reviewer.Request{
		Root: r.o.Root, Language: r.o.Language, Stack: r.o.Profile.Name(), Lenses: lenses, Diff: diff,
		SpecTitle: r.doc.Title, Marker: sc.Marker, Scenario: source,
		Invariants: spec.Section(r.md, spec.InvariantsTitle), Plan: r.plan,
		Model: r.o.Models, Env: r.o.AgentEnv, Timeout: r.o.AgentTimeout,
	}
}

// patch is the unified diff of the scenario's files: from git, or every
// file as new outside a repository.
func (s *Service) patch(ctx context.Context, r *run) (string, error) {
	paths := sortedStrings(r.st.FilesWritten)
	if s.d.VCS != nil {
		diff, err := s.d.VCS.Patch(ctx, r.o.Root, paths)
		if !errors.Is(err, ports.ErrNotARepository) {
			return diff, err
		}
	}
	var b strings.Builder
	for _, p := range paths {
		if data, err := s.d.Files.ReadFile(r.lay.Abs(p)); err == nil {
			b.WriteString(review.NewFileDiff(p, data))
		}
	}
	return b.String(), nil
}

// contents reads files, for measuring a correction afterwards.
func (s *Service) contents(r *run, paths []string) map[string][]byte {
	out := map[string][]byte{}
	for _, p := range paths {
		if data, err := s.d.Files.ReadFile(r.lay.Abs(p)); err == nil {
			out[p] = data
		}
	}
	return out
}

// keep saves the review record next to the specification and the state.
func (s *Service) keep(r *run, sc tdd.ScenarioRef, rec *tdd.ReviewRecord) error {
	return s.keepRecord(r, r.lay.ReviewRecord(r.o.SpecPath, sc.Marker), rec)
}

// keepRecord writes a scenario's record next to the specification, where
// the commit and the delivery find it, and saves the loop state.
func (s *Service) keepRecord(r *run, path string, rec any) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := s.d.Files.WriteFile(path, append(data, '\n')); err != nil {
		return fmt.Errorf("saving %s: %w", r.lay.Rel(path), err)
	}
	return s.save(r)
}

// lastGates is the gate summary REFACTOR accepted for scenario index.
func lastGates(st *tdd.State, index int) string {
	for i := len(st.Checkpoints) - 1; i >= 0; i-- {
		c := st.Checkpoints[i]
		if c.Scenario == index && c.Step == "refactor" && c.Status == "accepted" {
			return c.Details
		}
	}
	return ""
}
