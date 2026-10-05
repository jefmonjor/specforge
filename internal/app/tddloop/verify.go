package tddloop

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"specforge/internal/app/verifier"
	"specforge/internal/domain/review"
	"specforge/internal/domain/risk"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
	"specforge/internal/domain/verification"
	"specforge/internal/ports"
)

// Verify modes (verify in specforge.yaml).
const (
	VerifyOff     = "off"
	VerifyHigh    = "high"
	VerifyAlways  = "always"
	VerifyFeature = "feature"
)

// Verifier checks the specification in a copy of the project.
type Verifier interface {
	Verify(ctx context.Context, req verifier.Request) (verifier.Result, error)
}

// verifyStep runs the independent verification of the current scenario
// when its mode asks for it: once, then one correction of what it showed
// broken (judged again by REFACTOR), a re-verification of those
// requirements only, and the regression tests it proposed, offered to the
// developer. proceed is false when the scenario went back to REFACTOR.
func (s *Service) verifyStep(ctx context.Context, r *run, sc tdd.ScenarioRef) (proceed bool, err error) {
	ref := &r.st.Scenarios[r.st.Current]
	if !s.verifies(r, ref) {
		return true, nil
	}
	if ref.Verify == nil {
		res, err := s.d.Verifier.Verify(ctx, s.verifyRequest(r, s.scenarioRequirements(r, sc), ""))
		if err != nil {
			return false, err
		}
		ref.Verify = &tdd.VerifyRecord{Report: res.Report, Skipped: res.Skipped}
		r.st.Record("verify", "ran", verifySummary(res), s.d.Now())
		s.d.Events.Verified(sc, *ref.Verify)
		if err := s.keepVerify(r, sc); err != nil {
			return false, err
		}
	}
	rec := ref.Verify
	if rec.Done || rec.Skipped != "" {
		return true, nil
	}
	blockers := rec.Report.BlockerIDs()
	switch {
	case len(blockers) > 0 && rec.Corrected == nil:
		size, budget, err := s.correction(ctx, r, sc, asFindings(rec.Report.Blockers))
		if err != nil {
			return false, err
		}
		rec.Corrected = blockers
		r.st.Record("verify", "corrected", fmt.Sprintf("%s: %d line(s) of %d", strings.Join(blockers, ", "), size, budget), s.d.Now())
		if err := s.keepVerify(r, sc); err != nil {
			return false, err
		}
		return false, s.reopen(ctx, r, sc, size, budget)
	case rec.Corrected != nil && rec.Recheck == nil:
		res, err := s.d.Verifier.Verify(ctx, s.verifyRequest(r, rec.Corrected, ""))
		if err != nil {
			return false, err
		}
		recheck := res.Report.Only(rec.Corrected)
		rec.Recheck = &recheck
		r.st.Record("verify", "rechecked", fmt.Sprintf("%d still broken", len(recheck.Blockers)), s.d.Now())
		if err := s.keepVerify(r, sc); err != nil {
			return false, err
		}
	}
	if err := s.settleUnmet(ctx, r, sc, rec); err != nil {
		return false, err
	}
	if err := s.offerRegressionTests(ctx, r, sc, rec); err != nil {
		return false, err
	}
	rec.Done = true
	return true, s.keepVerify(r, sc)
}

// scenarioRequirements are what a scenario's verification answers for:
// the scenario and the invariants it names. Other invariants belong to
// other scenarios, which may not exist yet; verify: feature checks them
// all at the end.
func (s *Service) scenarioRequirements(r *run, sc tdd.ScenarioRef) []string {
	defined := spec.InvariantIDs(r.md)
	var out []string
	for _, d := range r.doc.Scenarios {
		if d.Index != sc.Index {
			continue
		}
		for _, id := range spec.InvariantRefs(d.Title + "\n" + d.Source) {
			if slices.Contains(defined, id) {
				out = append(out, id)
			}
		}
	}
	return append(out, sc.Marker)
}

func (s *Service) verifies(r *run, ref *tdd.ScenarioRef) bool {
	if s.d.Verifier == nil {
		return false
	}
	switch r.o.Verify {
	case VerifyAlways:
		return true
	case VerifyHigh:
		return ref.Risk != nil && ref.Risk.Tier == risk.High
	}
	return false
}

// settleUnmet asks the developer about each requirement still broken
// after the correction: there is never a second automatic correction.
func (s *Service) settleUnmet(ctx context.Context, r *run, sc tdd.ScenarioRef, rec *tdd.VerifyRecord) error {
	if rec.Recheck == nil {
		return nil
	}
	for _, b := range rec.Recheck.Blockers {
		if slices.Contains(rec.FollowUps, b.ID) {
			continue
		}
		q := question(r.o.Language, "unmet")
		text := fmt.Sprintf(q.text, sc.Index, b.ID, b.Command, b.Observed, b.Expected)
		answer, err := s.d.Asker.Ask(ctx, s.originAs(r, sc, OriginVerify), ports.Question{Text: text, Options: q.options, Strict: true})
		if err != nil {
			return err
		}
		if pick(answer, q.options) != 0 {
			return fmt.Errorf("%w (%s still broken)", ErrReviewStopped, b.ID)
		}
		rec.FollowUps = append(rec.FollowUps, b.ID)
		if err := s.keepVerify(r, sc); err != nil {
			return err
		}
	}
	return nil
}

// offerRegressionTests offers the verifier's regression tests. Added, each
// must be a new test file inside the project and the whole suite must
// still pass; then their fingerprints join the scenario's tests.
func (s *Service) offerRegressionTests(ctx context.Context, r *run, sc tdd.ScenarioRef, rec *tdd.VerifyRecord) error {
	tests := s.usableTests(r, rec.Report.RegressionTests)
	if rec.TestsDecided || len(tests) == 0 {
		rec.TestsDecided = true
		return nil
	}
	var paths []string
	for _, t := range tests {
		paths = append(paths, t.Path+" ("+strings.Join(t.Covers, ", ")+")")
	}
	q := question(r.o.Language, "regression-tests")
	answer, err := s.d.Asker.Ask(ctx, s.originAs(r, sc, OriginVerify), ports.Question{Text: fmt.Sprintf(q.text, sc.Index, strings.Join(paths, "; ")), Options: q.options, Strict: true})
	if err != nil {
		return err
	}
	rec.TestsDecided = true
	if pick(answer, q.options) != 0 {
		return s.keepVerify(r, sc)
	}
	var written []string
	for _, t := range tests {
		if err := s.d.Files.WriteFile(r.lay.Abs(t.Path), []byte(t.Content)); err != nil {
			return err
		}
		written = append(written, t.Path)
	}
	failure, err := s.runSuite(ctx, r)
	if err != nil {
		return err
	}
	if failure != "" {
		for _, p := range written {
			if err := s.d.Files.Remove(r.lay.Abs(p)); err != nil {
				return err
			}
		}
		r.st.Record("verify", "regression tests failed", failure, s.d.Now())
		rec.Report.Advisories = append(rec.Report.Advisories, "the proposed regression tests failed and were not added: "+firstLine(failure))
		return s.keepVerify(r, sc)
	}
	hashes, err := s.d.Workspace.HashFiles(r.o.Root, r.o.Profile.IsTestFile)
	if err != nil {
		return err
	}
	r.st.TestHashes = hashes
	r.st.AddFiles(written...)
	rec.Tests = written
	r.st.Record("verify", "regression tests added", strings.Join(written, ", "), s.d.Now())
	return s.keepVerify(r, sc)
}

// usableTests keeps the proposed tests that are new test files inside the
// project.
func (s *Service) usableTests(r *run, tests []verification.RegressionTest) []verification.RegressionTest {
	var out []verification.RegressionTest
	for _, t := range tests {
		p := path.Clean(strings.ReplaceAll(t.Path, "\\", "/"))
		if path.IsAbs(p) || strings.HasPrefix(p, "..") || !r.o.Profile.IsTestFile(p) || s.d.Files.Exists(r.lay.Abs(p)) {
			continue
		}
		t.Path = p
		out = append(out, t)
	}
	return out
}

// verifyFeature verifies the whole specification once every scenario is
// done (verify: feature). There is no automatic correction at this level:
// what is broken stops the command for the developer.
func (s *Service) verifyFeature(ctx context.Context, r *run) error {
	if s.d.Verifier == nil || r.o.Verify != VerifyFeature {
		return nil
	}
	required := spec.InvariantIDs(r.md)
	for _, sc := range r.st.Scenarios {
		required = append(required, sc.Marker)
	}
	res, err := s.d.Verifier.Verify(ctx, s.verifyRequest(r, required, r.lay.FeatureVerify(r.o.SpecPath)))
	if err != nil {
		return err
	}
	r.st.Record("verify", "feature", verifySummary(res), s.d.Now())
	if len(res.Report.Blockers) > 0 {
		return &verifier.BlockedError{Blockers: res.Report.Blockers}
	}
	return nil
}

func (s *Service) verifyRequest(r *run, required []string, report string) verifier.Request {
	return verifier.Request{
		Root: r.o.Root, Language: r.o.Language, Stack: r.o.Profile.Name(), Base: "HEAD",
		SpecTitle: r.doc.Title, Spec: strings.TrimSpace(spec.StripSeal(r.md)), Required: required,
		Report: report, Model: r.o.Models, Env: r.o.AgentEnv, Timeout: r.o.AgentTimeout, Now: s.d.Now(),
	}
}

func (s *Service) keepVerify(r *run, sc tdd.ScenarioRef) error {
	data, err := json.MarshalIndent(r.st.Scenarios[r.st.Current].Verify, "", "  ")
	if err != nil {
		return err
	}
	if err := s.d.Files.WriteFile(r.lay.VerifyRecord(r.o.SpecPath, sc.Marker), append(data, '\n')); err != nil {
		return fmt.Errorf("saving the verification of %s: %w", sc.Marker, err)
	}
	return s.save(r)
}

// asFindings presents the verifier's blockers to the correction turn.
func asFindings(blockers []verification.Blocker) []review.Finding {
	out := make([]review.Finding, len(blockers))
	for i, b := range blockers {
		out[i] = review.Finding{ID: b.ID, Severity: review.Blocker,
			Claim: fmt.Sprintf("`%s` printed `%s`; the specification expects: %s", b.Command, b.Observed, b.Expected)}
	}
	return out
}

func verifySummary(res verifier.Result) string {
	if res.Skipped != "" {
		return "skipped: " + res.Skipped
	}
	r := res.Report
	return fmt.Sprintf("%d met · %d unmet · %d unverified", r.Count(verification.Met), r.Count(verification.Unmet), r.Count(verification.Unverified))
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}
