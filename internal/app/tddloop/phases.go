package tddloop

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"specforge/internal/app/prompts"
	"specforge/internal/domain/quality"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// red asks for a failing test and accepts it only when a test file carrying
// the scenario marker changed, it compiles, it ran, and it failed.
func (s *Service) red(ctx context.Context, r *run) error {
	sc, _ := r.st.Scenario()
	fb := ""
	for {
		if r.st.Attempts >= r.o.MaxAttempts {
			return fmt.Errorf("%w (RED, scenario %d)", tdd.ErrAttemptsExhausted, sc.Index)
		}
		data := s.promptData(r, sc)
		data.TestFiles = s.specTests(r, nil)
		data.LastFailure = r.st.LastFailure
		data.Feedback = fb

		testsBefore, err := s.d.Workspace.HashFiles(r.o.Root, r.o.Profile.IsTestFile)
		if err != nil {
			return err
		}
		resp, changed, err := s.agentStep(ctx, r, prompts.Red, data)
		if err != nil {
			return err
		}
		if bad := falseClaims(resp.FilesWritten, changed); len(bad) > 0 {
			fb = s.reject(r, RejectFalseClaim, joinPaths(bad), "")
			continue
		}

		testsAfter, err := s.d.Workspace.HashFiles(r.o.Root, r.o.Profile.IsTestFile)
		if err != nil {
			return err
		}
		newTests := changedKeys(testsBefore, testsAfter)
		if len(newTests) == 0 {
			fb = s.reject(r, RejectNoTest, "", "")
			continue
		}
		if !s.anyContains(r, newTests, sc.Marker) {
			fb = s.reject(r, RejectNoMarker, sc.Marker, "")
			continue
		}

		out, err := s.runTests(ctx, r, sc.Marker)
		if err != nil {
			return err
		}
		switch out.Red() {
		case tdd.RedNotCompiled:
			fb = s.reject(r, RejectNotCompiled, "", out.Output)
			continue
		case tdd.RedNothingRan:
			fb = s.reject(r, RejectNothingRan, sc.Marker, out.Output)
			continue
		case tdd.RedPremature:
			accepted, stricter, err := s.decidePremature(ctx, r, sc)
			if err != nil || accepted {
				return err
			}
			fb = stricter
			continue
		}
		if !out.Exact {
			ok, why, err := s.confirmInexact(ctx, r, out)
			if err != nil {
				return err
			}
			if !ok {
				fb = s.reject(r, RejectUnconfirmed, why, out.Output)
				continue
			}
		}

		r.st.TestHashes = testsAfter
		r.st.AddFiles(changed...)
		r.st.LastFailure = out.Output
		r.st.Record("red", "accepted", fmt.Sprintf("%d failing test(s)", out.Failed), s.d.Now())
		s.d.Events.Accepted(tdd.PhaseRed, sc)
		r.st.Advance(s.d.Now())
		return s.save(r)
	}
}

// green asks for the minimum implementation and accepts it only when the
// scenario's test passes and no test file changed.
func (s *Service) green(ctx context.Context, r *run) error {
	sc, _ := r.st.Scenario()
	fb := ""
	for {
		if r.st.Attempts >= r.o.MaxAttempts {
			return fmt.Errorf("%w (GREEN, scenario %d)", tdd.ErrAttemptsExhausted, sc.Index)
		}
		data := s.promptData(r, sc)
		data.TestFiles = s.specTests(r, r.st.FilesWritten)
		data.LastFailure = r.st.LastFailure
		data.Feedback = fb

		resp, changed, err := s.agentStep(ctx, r, prompts.Green, data)
		if err != nil {
			return err
		}
		if err := s.checkTampering(r); err != nil {
			return err
		}
		if bad := falseClaims(resp.FilesWritten, changed); len(bad) > 0 {
			fb = s.reject(r, RejectFalseClaim, joinPaths(bad), r.st.LastFailure)
			continue
		}
		r.st.AddFiles(changed...)

		out, err := s.runTests(ctx, r, sc.Marker)
		if err != nil {
			return err
		}
		if !out.Green() {
			reason := RejectStillFailing
			if !out.Compiled {
				reason = RejectNotCompiled
			}
			fb = s.reject(r, reason, "", out.Output)
			continue
		}
		r.st.Record("green", "accepted", fmt.Sprintf("%d passing test(s)", out.Passed), s.d.Now())
		s.d.Events.Accepted(tdd.PhaseGreen, sc)
		r.st.Advance(s.d.Now())
		return s.save(r)
	}
}

// refactor runs the whole suite and the quality gates; while something
// blocks, it asks the agent to fix it without touching the tests.
func (s *Service) refactor(ctx context.Context, r *run) error {
	sc, _ := r.st.Scenario()
	fb := ""
	for {
		suite, err := s.runTests(ctx, r, "")
		if err != nil {
			return err
		}
		report, err := s.runGates(ctx, r)
		if err != nil {
			return err
		}
		s.d.Events.Gates(report)

		suiteFailure := ""
		if !suite.Green() {
			suiteFailure = suite.Output
		}
		if suiteFailure == "" && report.OK(r.o.Strict) {
			r.st.Record("refactor", "accepted", gateSummary(report), s.d.Now())
			s.d.Events.Accepted(tdd.PhaseRefactor, sc)
			r.st.Advance(s.d.Now())
			return s.save(r)
		}
		if suiteFailure == "" && onlySkipped(report, r.o.Strict) {
			// The agent cannot install tools: stop and tell the developer.
			return &GatesError{Report: report, Strict: r.o.Strict}
		}
		if r.st.Attempts >= r.o.MaxAttempts {
			return &GatesError{Report: report, Strict: r.o.Strict, SuiteFailure: suiteFailure}
		}

		data := s.promptData(r, sc)
		data.TestFiles = s.specTests(r, r.st.FilesWritten)
		data.SuiteFailure = suiteFailure
		data.GateReport = report.Explain(r.o.Strict)
		data.Feedback = fb

		resp, changed, err := s.agentStep(ctx, r, prompts.Refactor, data)
		if err != nil {
			return err
		}
		if err := s.checkTampering(r); err != nil {
			return err
		}
		r.st.AddFiles(changed...)
		r.st.Fail(strings.TrimSpace(suiteFailure+"\n"+data.GateReport), s.d.Now())
		if bad := falseClaims(resp.FilesWritten, changed); len(bad) > 0 {
			fb = feedback(r.o.Language, RejectFalseClaim, joinPaths(bad))
		} else {
			fb = ""
		}
		if err := s.save(r); err != nil {
			return err
		}
	}
}

// agentStep runs one agent turn and returns what the agent says plus what
// really changed on disk.
func (s *Service) agentStep(ctx context.Context, r *run, name prompts.Name, data prompts.Data) (respFiles, []string, error) {
	before, err := s.d.Workspace.Snapshot(ctx, r.o.Root)
	if err != nil {
		return respFiles{}, nil, err
	}
	resp, err := s.converse(ctx, r, name, data)
	if err != nil {
		return respFiles{}, nil, err
	}
	after, err := s.d.Workspace.Snapshot(ctx, r.o.Root)
	if err != nil {
		return respFiles{}, nil, err
	}
	return respFiles{FilesWritten: resp.FilesWritten}, before.Changed(after), nil
}

type respFiles struct{ FilesWritten []string }

// checkTampering compares the test files with the fingerprints taken when
// RED was accepted. Any difference stops the loop: an implementation that
// passes by editing its test proves nothing.
func (s *Service) checkTampering(r *run) error {
	now, err := s.d.Workspace.HashFiles(r.o.Root, r.o.Profile.IsTestFile)
	if err != nil {
		return err
	}
	if changed := changedKeys(r.st.TestHashes, now); len(changed) > 0 {
		r.st.Record("tampering", "stopped", joinPaths(changed), s.d.Now())
		return &tdd.TamperingError{Phase: r.st.Phase, Changed: changed}
	}
	return nil
}

func (s *Service) runTests(ctx context.Context, r *run, filter string) (tdd.Outcome, error) {
	s.d.Events.RunningTests(r.o.Profile.TestCommand(filter))
	out, err := s.d.Tests.Run(ctx, ports.TestRequest{Root: r.o.Root, Profile: r.o.Profile, Filter: filter, Timeout: r.o.TestTimeout})
	if err != nil {
		return out, fmt.Errorf("running %q: %w", r.o.Profile.TestCommand(filter), err)
	}
	return out, nil
}

func (s *Service) runGates(ctx context.Context, r *run) (quality.Report, error) {
	var report quality.Report
	for _, g := range s.d.Gates {
		if !g.Applies(r.o.Profile) {
			continue
		}
		res, err := g.Check(ctx, r.o.Root, r.o.Profile)
		if err != nil {
			return report, err
		}
		report = append(report, res)
	}
	return report, nil
}

func (s *Service) reject(r *run, why Rejection, arg, output string) string {
	text := feedback(r.o.Language, why)
	if arg != "" {
		text = feedback(r.o.Language, why, arg)
	}
	s.d.Events.Rejected(why, arg)
	failure := output
	if failure == "" {
		failure = text
	}
	r.st.Fail(failure, s.d.Now())
	r.st.Record(strings.ToLower(string(r.st.Phase)), "rejected", string(why), s.d.Now())
	return text
}

func (s *Service) anyContains(r *run, paths []string, needle string) bool {
	for _, p := range paths {
		data, err := s.d.Files.ReadFile(r.lay.Abs(p))
		if err == nil && strings.Contains(string(data), needle) {
			return true
		}
	}
	return false
}

// decidePremature asks the developer what a test that passes before any
// implementation means. accepted is true when the scenario was closed.
func (s *Service) decidePremature(ctx context.Context, r *run, sc tdd.ScenarioRef) (accepted bool, stricterFeedback string, err error) {
	q := question(r.o.Language, "premature")
	answer, err := s.d.Asker.Ask(ctx, s.origin(r, sc), ports.Question{Text: fmt.Sprintf(q.text, sc.Index), Options: q.options})
	if err != nil {
		return false, "", fmt.Errorf("%w: %w", tdd.ErrPrematureGreen, err)
	}
	switch pick(answer, q.options) {
	case 0:
		r.st.Record("red", "satisfied", "developer confirmed the behaviour already exists", s.d.Now())
		s.d.Events.Satisfied(sc)
		r.st.MarkSatisfied(s.d.Now())
		return true, "", s.save(r)
	case 1:
		return false, s.reject(r, RejectPremature, "", ""), nil
	default:
		return false, "", tdd.ErrPrematureGreen
	}
}

func (s *Service) confirmInexact(ctx context.Context, r *run, out tdd.Outcome) (bool, string, error) {
	sc, _ := r.st.Scenario()
	q := question(r.o.Language, "inexact")
	text := fmt.Sprintf(q.text, r.o.Profile.Runner, out.Output)
	answer, err := s.d.Asker.Ask(ctx, s.origin(r, sc), ports.Question{Text: text, Options: q.options})
	if err != nil {
		return false, "", err
	}
	if pick(answer, q.options) == 0 {
		return true, "", nil
	}
	return false, answer, nil
}

func onlySkipped(r quality.Report, strict bool) bool {
	blocking := r.Blocking(strict)
	if len(blocking) == 0 {
		return false
	}
	for _, b := range blocking {
		if b.Status != quality.Skipped {
			return false
		}
	}
	return true
}

func gateSummary(r quality.Report) string {
	var parts []string
	for _, res := range r {
		parts = append(parts, res.Gate+"="+string(res.Status))
	}
	return strings.Join(parts, " ")
}

func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
