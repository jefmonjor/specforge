package tddloop

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"specforge/internal/app/clarify"
	"specforge/internal/app/prompts"
	"specforge/internal/domain/lessons"
	"specforge/internal/domain/quality"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// red asks for a failing test and accepts it only when a test file carrying
// the scenario marker changed, it compiles, it ran, and it failed.
func (s *Service) red(ctx context.Context, r *run) error {
	sc, _ := r.st.Scenario()
	fb := ""
	if r.st.PendingFor() == nil && r.st.ReviewNote == "" {
		done, feedback, err := s.existingRed(ctx, r, sc)
		if err != nil || done {
			return err
		}
		fb = feedback
	}
	for {
		if r.st.Attempts >= r.o.MaxAttempts {
			return fmt.Errorf("%w (RED, scenario %d)", tdd.ErrAttemptsExhausted, sc.Index)
		}
		data := s.promptData(r, sc)
		data.TestFiles = s.specTests(r, nil)
		data.LastFailure = r.st.LastFailure
		data.Feedback = fb
		if fb == "" {
			data.Feedback = r.st.ReviewNote
		}

		pending, err := s.answerPending(ctx, r, sc)
		if err != nil {
			return err
		}
		var (
			t           turn
			testsBefore map[string]string
		)
		switch {
		case pending != nil && pending.Kind == tdd.PendingVerify:
			testsBefore = pending.Tests
			t, err = s.observe(ctx, r, pending)
		case pending != nil:
			testsBefore = pending.Tests
			s.withAnswer(r, sc, &data, pending)
			t, err = s.agentStep(ctx, r, prompts.Red, data, pending.Workspace)
		default:
			if testsBefore, err = s.d.Workspace.HashFiles(r.o.Root, r.o.Profile.IsTestFile); err != nil {
				return err
			}
			t, err = s.agentStep(ctx, r, prompts.Red, data, nil)
		}
		if err != nil {
			if r.st.Pending != nil {
				r.st.Pending.Tests = testsBefore
			}
			return err
		}
		if bad := falseClaims(t.Claimed, t.Changed); len(bad) > 0 {
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
			accepted, stricter, err := s.decidePremature(ctx, r, sc, t.Changed)
			if err != nil {
				s.pendVerify(r, sc, t, testsBefore, err)
				return err
			}
			if accepted {
				return nil
			}
			fb = stricter
			continue
		}
		if !out.Exact {
			ok, why, err := s.confirmInexact(ctx, r, out)
			if err != nil {
				s.pendVerify(r, sc, t, testsBefore, err)
				return err
			}
			if !ok {
				fb = s.reject(r, RejectUnconfirmed, why, out.Output)
				continue
			}
		}

		r.st.TestHashes = testsAfter
		r.st.AddFiles(t.Changed...)
		r.st.LastFailure = out.Output
		if err := s.learn(r); err != nil {
			return err
		}
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
		if fb == "" {
			data.Feedback = r.st.ReviewNote
		}

		pending, err := s.answerPending(ctx, r, sc)
		if err != nil {
			return err
		}
		var before ports.Snapshot
		if pending != nil {
			before = pending.Workspace
			s.withAnswer(r, sc, &data, pending)
		}
		t, err := s.agentStep(ctx, r, prompts.Green, data, before)
		if err != nil {
			return err
		}
		if err := s.checkTampering(r); err != nil {
			return err
		}
		if bad := falseClaims(t.Claimed, t.Changed); len(bad) > 0 {
			fb = s.reject(r, RejectFalseClaim, joinPaths(bad), r.st.LastFailure)
			continue
		}
		r.st.AddFiles(t.Changed...)

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
		if err := s.learn(r); err != nil {
			return err
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
	if p := r.st.PendingFor(); p != nil && p.Kind == tdd.PendingReview {
		// Everything was verified already; only the review is missing.
		return s.close(ctx, r, sc, p.Context)
	}
	for {
		pending, err := s.answerPending(ctx, r, sc)
		if err != nil {
			return err
		}
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
			if err := s.learn(r); err != nil {
				return err
			}
			r.st.Record("refactor", "accepted", gateSummary(report), s.d.Now())
			s.d.Events.Accepted(tdd.PhaseRefactor, sc)
			return s.close(ctx, r, sc, gateSummary(report))
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
		if fb == "" {
			data.Feedback = r.st.ReviewNote
		}

		var before ports.Snapshot
		if pending != nil {
			before = pending.Workspace
			s.withAnswer(r, sc, &data, pending)
		}
		t, err := s.agentStep(ctx, r, prompts.Refactor, data, before)
		if err != nil {
			return err
		}
		if err := s.checkTampering(r); err != nil {
			return err
		}
		r.st.AddFiles(t.Changed...)
		r.st.Fail(strings.TrimSpace(suiteFailure+"\n"+data.GateReport), s.d.Now())
		if bad := falseClaims(t.Claimed, t.Changed); len(bad) > 0 {
			fb = feedback(r.o.Language, RejectFalseClaim, joinPaths(bad))
		} else {
			fb = ""
		}
		if err := s.save(r); err != nil {
			return err
		}
	}
}

// turn is one agent turn as observed on disk.
type turn struct {
	// Claimed are the files the agent says it wrote.
	Claimed []string
	// Before is the snapshot the turn is measured from.
	Before ports.Snapshot
	// Changed are the files that really changed since Before.
	Changed []string
}

// agentStep runs one agent turn and returns what the agent says plus what
// really changed on disk since before (a fresh snapshot when nil). When the
// agent asks something nobody can answer now, the turn is saved as pending
// with its baseline, so --resume continues it instead of starting over.
func (s *Service) agentStep(ctx context.Context, r *run, name prompts.Name, data prompts.Data, before ports.Snapshot) (turn, error) {
	if before == nil {
		var err error
		if before, err = s.d.Workspace.Snapshot(ctx, r.o.Root); err != nil {
			return turn{}, err
		}
	}
	resp, err := s.converse(ctx, r, name, data)
	if err != nil {
		var pending *clarify.PendingQuestionError
		if errors.As(err, &pending) {
			sc, _ := r.st.Scenario()
			r.st.Pending = &tdd.Pending{
				Kind: tdd.PendingAgent, Phase: r.st.Phase, Scenario: sc.Index,
				Question: resp.Question, Context: resp.Context, Options: resp.Options,
				Workspace: before,
			}
		}
		return turn{}, err
	}
	r.st.Pending = nil
	if resp.Lesson != "" {
		r.lesson = resp.Lesson
	}
	after, err := s.d.Workspace.Snapshot(ctx, r.o.Root)
	if err != nil {
		return turn{}, err
	}
	return turn{Claimed: resp.FilesWritten, Before: before, Changed: before.Changed(after)}, nil
}

// answerPending answers the question that interrupted the current step,
// from the questions file or the terminal, before anything else runs. It
// returns the interrupted step, or nil when nothing is pending. Without an
// answer it returns the pending error again and the agent is not called.
func (s *Service) answerPending(ctx context.Context, r *run, sc tdd.ScenarioRef) (*tdd.Pending, error) {
	p := r.st.PendingFor()
	if p == nil || p.Kind != tdd.PendingAgent {
		return p, nil
	}
	q := ports.Question{Text: p.Question, Context: p.Context, Options: p.Options}
	answer, err := s.d.Asker.Ask(ctx, s.origin(r, sc), q)
	if err != nil {
		return nil, err
	}
	p.Answer = answer
	r.st.Record("question", "answered", p.Question+" → "+answer, s.d.Now())
	s.d.Events.Answered(p.Question, answer)
	return p, s.save(r)
}

// learn keeps the agent's lesson when the phase needed more than one
// attempt: that is when there was a mistake worth not repeating.
func (s *Service) learn(r *run) error {
	lesson := r.lesson
	r.lesson = ""
	if lesson == "" || r.st.Attempts == 0 {
		return nil
	}
	path := r.lay.Lessons()
	current := ""
	if data, err := s.d.Files.ReadFile(path); err == nil {
		current = string(data)
	}
	next, added := lessons.Add(current, lessons.Lesson{Stack: string(r.o.Profile.Kind), Text: lesson})
	if !added {
		return nil
	}
	r.st.Record("lesson", "kept", lesson, s.d.Now())
	return s.d.Files.WriteFile(path, []byte(next))
}

// withAnswer adds the answer to a pending question to the prompt, with the
// decisions log as it is now.
func (s *Service) withAnswer(r *run, sc tdd.ScenarioRef, data *prompts.Data, p *tdd.Pending) {
	data.Decisions = s.d.Asker.Decisions(s.origin(r, sc))
	data.AnsweredQuestion, data.Answer = p.Question, p.Answer
}

// observe measures an interrupted turn again without calling the agent.
func (s *Service) observe(ctx context.Context, r *run, p *tdd.Pending) (turn, error) {
	after, err := s.d.Workspace.Snapshot(ctx, r.o.Root)
	if err != nil {
		return turn{}, err
	}
	r.st.Pending = nil
	before := ports.Snapshot(p.Workspace)
	return turn{Claimed: p.Claimed, Before: before, Changed: before.Changed(after)}, nil
}

// pendVerify saves a RED step whose verification asked a question nobody
// could answer, so --resume verifies it again with the answer.
func (s *Service) pendVerify(r *run, sc tdd.ScenarioRef, t turn, tests map[string]string, err error) {
	var pending *clarify.PendingQuestionError
	if errors.As(err, &pending) {
		r.st.Pending = &tdd.Pending{
			Kind: tdd.PendingVerify, Phase: r.st.Phase, Scenario: sc.Index,
			Claimed: t.Claimed, Workspace: t.Before, Tests: tests,
		}
	}
}

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

// existingRed handles a scenario whose test already exists, typically
// written by a run that stopped before RED was accepted. The test is run
// before the agent is called: a valid RED is accepted as it is, a passing
// test goes to the developer, and anything else becomes feedback for the
// agent. done is true when the phase is settled.
func (s *Service) existingRed(ctx context.Context, r *run, sc tdd.ScenarioRef) (done bool, feedback string, err error) {
	hashes, err := s.d.Workspace.HashFiles(r.o.Root, r.o.Profile.IsTestFile)
	if err != nil {
		return false, "", err
	}
	var withMarker []string
	for path := range hashes {
		withMarker = append(withMarker, path)
	}
	withMarker = slices.DeleteFunc(withMarker, func(p string) bool { return !s.anyContains(r, []string{p}, sc.Marker) })
	if len(withMarker) == 0 {
		return false, "", nil
	}
	slices.Sort(withMarker)
	out, err := s.runTests(ctx, r, sc.Marker)
	if err != nil || !out.Exact {
		return false, "", err
	}
	switch out.Red() {
	case tdd.RedValid:
		r.st.TestHashes = hashes
		r.st.AddFiles(withMarker...)
		r.st.LastFailure = out.Output
		r.st.Record("red", "accepted", "existing test "+strings.Join(withMarker, ", "), s.d.Now())
		s.d.Events.Accepted(tdd.PhaseRed, sc)
		r.st.Advance(s.d.Now())
		return true, "", s.save(r)
	case tdd.RedPremature:
		// Unanswered, the question waits in questions.md; the next run
		// comes back here and finds the answer there.
		return s.decidePremature(ctx, r, sc, withMarker)
	}
	// It does not compile or nothing ran: the agent fixes it.
	r.st.LastFailure = out.Output
	return false, "", nil
}

// decidePremature asks the developer what a test that passes before any
// implementation means. accepted is true when the scenario was closed.
func (s *Service) decidePremature(ctx context.Context, r *run, sc tdd.ScenarioRef, written []string) (accepted bool, stricterFeedback string, err error) {
	q := question(r.o.Language, "premature")
	answer, err := s.d.Asker.Ask(ctx, s.origin(r, sc), ports.Question{Text: fmt.Sprintf(q.text, sc.Index), Options: q.options, Strict: true})
	if err != nil {
		return false, "", fmt.Errorf("%w: %w", tdd.ErrPrematureGreen, err)
	}
	switch pick(answer, q.options) {
	case 0:
		// The test still documents the behaviour: record it like any scenario.
		r.st.AddFiles(written...)
		files := slices.Sorted(slices.Values(r.st.FilesWritten))
		sha, err := s.commit(ctx, r, sc, files, "test")
		if err != nil {
			return false, "", err
		}
		r.st.Scenarios[r.st.Current].Files = files
		r.st.Scenarios[r.st.Current].Commit = sha
		r.st.Record("red", "satisfied", "developer confirmed the behaviour already exists", s.d.Now())
		s.d.Events.Satisfied(sc)
		s.d.Events.Committed(sc, sha)
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
