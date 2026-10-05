package ui

import (
	"fmt"
	"strings"
	"sync"

	"specforge/internal/app/audit"
	"specforge/internal/app/reviewer"
	"specforge/internal/app/tddloop"
	"specforge/internal/app/verifier"
	"specforge/internal/domain/e2e"
	"specforge/internal/domain/quality"
	"specforge/internal/domain/review"
	"specforge/internal/domain/risk"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
	"specforge/internal/domain/verification"
)

// LoopEvents prints the TDD loop.
type LoopEvents struct {
	C     *Console
	Agent string
	Stack string

	mu   sync.Mutex
	stop func()
}

var (
	_ tddloop.Events  = (*LoopEvents)(nil)
	_ reviewer.Events = (*LoopEvents)(nil)
	_ verifier.Events = (*LoopEvents)(nil)
)

func (e *LoopEvents) halt() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stop != nil {
		e.stop()
		e.stop = nil
	}
}

func (e *LoopEvents) start(label string) {
	e.halt()
	if e.C.Quiet {
		return
	}
	e.mu.Lock()
	e.stop = Activity(e.C.Err, label)
	e.mu.Unlock()
}

func (e *LoopEvents) Started(st *tdd.State, doc *spec.Document) {
	e.C.Title(e.C.T("loop.title", doc.Title))
	e.C.Info(e.C.T("loop.stack", e.Stack, e.Agent, len(st.Scenarios)))
}

func (e *LoopEvents) Amended(pending []string) {
	e.C.Warn(e.C.T("loop.amended", len(pending), strings.Join(pending, " · ")))
}

func (e *LoopEvents) Orphaned(tests []string) {
	e.C.Warn(e.C.T("loop.orphaned", strings.Join(tests, " · ")))
}

func (e *LoopEvents) CheckpointFailed(reason string) {
	e.halt()
	e.C.Warn(e.C.T("loop.checkpoint.failed", reason))
}

func (e *LoopEvents) Baseline(b *tdd.Baseline, builds bool) {
	e.halt()
	switch {
	case !builds:
		e.C.Warn(e.C.T("loop.baseline.nobuild"))
	case b == nil:
		e.C.Warn(e.C.T("loop.baseline.unnamed"))
	case len(b.Failures) == 0:
		e.C.OK(e.C.T("loop.baseline.clean"))
	default:
		e.C.Warn(e.C.T("loop.baseline.known", len(b.Failures)))
		for _, f := range b.Failures {
			e.C.Detail(f.String())
		}
	}
}

func (e *LoopEvents) Phase(st *tdd.State, sc tdd.ScenarioRef) {
	e.halt()
	e.C.Title(e.C.T("loop.scenario", sc.Index, len(st.Scenarios), st.Phase, sc.Title))
}

func (e *LoopEvents) AgentWorking(p tdd.Phase) { e.start(e.C.T("agent.working", e.Agent, p)) }

func (e *LoopEvents) RunningTests(cmd string) { e.start(e.C.T("tests.running", cmd)) }

func (e *LoopEvents) Rejected(r tddloop.Rejection, detail string) {
	e.halt()
	reason := T(e.C.Lang, "reject."+string(r))
	if strings.Contains(reason, "%s") {
		reason = fmt.Sprintf(reason, detail)
	}
	e.C.Warn(e.C.T("loop.rejected", reason))
}

func (e *LoopEvents) Committed(_ tdd.ScenarioRef, sha string) {
	if sha != "" {
		e.halt()
		e.C.OK(e.C.T("loop.committed", short(sha)))
	}
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func (e *LoopEvents) Answered(string, string) {
	e.halt()
	e.C.OK(e.C.T("loop.answered"))
}

func (e *LoopEvents) Gates(r quality.Report) {
	e.halt()
	for _, res := range r {
		line := e.C.T("gate.line", res.Gate, res.Status, res.Summary)
		switch res.Status {
		case quality.Passed:
			e.C.OK(line)
		case quality.Skipped:
			e.C.Warn(line)
		default:
			e.C.Bad(line)
		}
	}
}

func (e *LoopEvents) Risk(_ tdd.ScenarioRef, a risk.Assessment) {
	e.halt()
	line := e.C.T("loop.risk", a.Tier, strings.Join(a.Reasons, " · "))
	if a.Tier == risk.High {
		e.C.Warn(line)
		return
	}
	e.C.Info(line)
}

func (e *LoopEvents) GateNotRun(gate string, tier risk.Tier) {
	e.halt()
	e.C.Info(e.C.T("loop.gate.notrun", gate, tier))
}

func (e *LoopEvents) ReviewSkipped(_ tdd.ScenarioRef, a risk.Assessment) {
	e.halt()
	e.C.OK(e.C.T("loop.review.skipped", strings.Join(a.Reasons, " · ")))
}

func (e *LoopEvents) Reviewed(_ tdd.ScenarioRef, rec tdd.ReviewRecord) {
	e.halt()
	if len(rec.Lenses) == 0 {
		e.C.Info(e.C.T("review.none"))
		return
	}
	e.C.OK(e.C.T("review.done", len(rec.Lenses), rec.Reported, len(rec.Corrected), len(rec.FollowUps), len(rec.Verdict.Discarded)))
	for _, d := range rec.Verdict.Discarded {
		e.C.Detail(e.C.T("review.discarded", d.ID, d.Reason))
	}
}

func (e *LoopEvents) Verified(_ tdd.ScenarioRef, rec tdd.VerifyRecord) {
	e.halt()
	if rec.Skipped != "" {
		e.C.Warn(e.C.T("verify.skipped", rec.Skipped))
		return
	}
	r := rec.Report
	line := e.C.T("verify.done", r.Count(verification.Met), r.Count(verification.Unmet), r.Count(verification.Unverified))
	if len(r.Blockers) == 0 {
		e.C.OK(line)
		return
	}
	e.C.Bad(line)
	for _, b := range r.Blockers {
		e.C.Detail(e.C.T("verify.blocker", b.ID, b.Command, b.Observed, b.Expected))
	}
}

func (e *LoopEvents) Parallel(markers []string) {
	e.halt()
	e.C.Title(e.C.T("loop.parallel", strings.Join(markers, " · ")))
}

func (e *LoopEvents) ParallelSkipped(sc tdd.ScenarioRef, why string) {
	e.halt()
	e.C.Warn(e.C.T("loop.parallel.skipped", sc.Marker, why))
}

func (e *LoopEvents) SeamFailed(string) {
	e.halt()
	e.C.Warn(e.C.T("loop.parallel.seam"))
}

func (e *LoopEvents) Integrated(sc tdd.ScenarioRef) {
	e.halt()
	e.C.OK(e.C.T("loop.parallel.integrated", sc.Marker))
}

// Verifying implements verifier.Events.
func (e *LoopEvents) Verifying(n int) { e.start(e.C.T("verify.running", n)) }

// Lens implements reviewer.Events.
func (e *LoopEvents) Lens(l review.Lens) { e.start(e.C.T("review.lens", l)) }

// Refuting implements reviewer.Events.
func (e *LoopEvents) Refuting(n int) { e.start(e.C.T("review.refuting", n)) }

// Validating implements reviewer.Events.
func (e *LoopEvents) Validating(n int) { e.start(e.C.T("review.validating", n)) }

// Retried implements reviewer.Events.
func (e *LoopEvents) Retried(step string) {
	e.halt()
	e.C.Warn(e.C.T("review.retried", step))
}

func (e *LoopEvents) Accepted(p tdd.Phase, _ tdd.ScenarioRef) {
	e.halt()
	e.C.OK(e.C.T("loop.accepted", p))
}

func (e *LoopEvents) Satisfied(sc tdd.ScenarioRef) {
	e.halt()
	e.C.OK(e.C.T("loop.satisfied", sc.Index))
}

func (e *LoopEvents) Finished(st *tdd.State) {
	e.halt()
	satisfied := 0
	for _, sc := range st.Scenarios {
		if sc.Satisfied {
			satisfied++
		}
	}
	if satisfied == 0 {
		e.C.Title(e.C.T("loop.finished", len(st.Scenarios)))
		return
	}
	e.C.Title(e.C.T("loop.finished.satisfied", len(st.Scenarios), len(st.Scenarios)-satisfied, satisfied))
}

// AuditEvents prints the audit.
type AuditEvents struct {
	C    *Console
	stop func()
}

var _ audit.Events = (*AuditEvents)(nil)

func (e *AuditEvents) Target(scope audit.Scope, base string, chunks int) {
	if scope == audit.ScopeDiff {
		e.C.Info(e.C.T("audit.target.diff", base, chunks))
	} else {
		e.C.Info(e.C.T("audit.target.full", chunks))
	}
}

func (e *AuditEvents) Step(chunk, total int, step string) {
	e.Done()
	if !e.C.Quiet {
		e.stop = Activity(e.C.Err, e.C.T("audit.step", chunk, total, step))
	}
}

func (e *AuditEvents) Retried(step string) {
	e.Done()
	e.C.Warn(e.C.T("audit.retry", step))
}

// Done stops the activity indicator.
func (e *AuditEvents) Done() {
	if e.stop != nil {
		e.stop()
		e.stop = nil
	}
}

// E2EEvents prints an E2E run.
type E2EEvents struct{ C *Console }

func (e *E2EEvents) Scenario(i, total int, title string) {
	e.C.Title(e.C.T("e2e.scenario", i, total, title))
}

func (e *E2EEvents) Step(n int, a e2e.Action, outcome string) {
	target := a.Selector
	if a.Type == e2e.Assert && a.Evidence != nil {
		target = fmt.Sprintf("%s %q", a.Evidence.Kind, a.Evidence.Value)
	}
	e.C.Info(e.C.T("e2e.step", n, a.Type, target, outcome))
}

func (e *E2EEvents) Result(r e2e.ScenarioResult) {
	verified := 0
	for _, t := range r.Thens {
		if t.Verified {
			verified++
		}
	}
	line := e.C.T("e2e.result", r.Status, verified, len(r.Thens))
	if r.Status == e2e.Passed {
		e.C.OK(line)
	} else {
		e.C.Bad(line + " — " + r.Reason)
	}
}

// PlanEvents prints the drafting of a plan.
type PlanEvents struct {
	C     *Console
	Agent string
	// Phase labels the activity indicator (PLAN when empty).
	Phase string
	stop  func()
}

func (e *PlanEvents) Working() {
	e.Done()
	if !e.C.Quiet {
		phase := e.Phase
		if phase == "" {
			phase = "PLAN"
		}
		e.stop = Activity(e.C.Err, e.C.T("agent.working", e.Agent, phase))
	}
}

func (e *PlanEvents) Rejected(reason string) {
	e.Done()
	e.C.Warn(e.C.T("loop.rejected", reason))
}

func (e *PlanEvents) Answered(string, string) {
	e.Done()
	e.C.OK(e.C.T("loop.answered"))
}

// Done stops the activity indicator.
func (e *PlanEvents) Done() {
	if e.stop != nil {
		e.stop()
		e.stop = nil
	}
}

// InterviewEvents prints the progress of an interview.
type InterviewEvents struct {
	C     *Console
	Agent string
	// Phase names the work in progress ("INTERVIEW" when empty).
	Phase string
	stop  func()
}

func (e *InterviewEvents) Working() {
	e.Done()
	if !e.C.Quiet {
		phase := e.Phase
		if phase == "" {
			phase = "INTERVIEW"
		}
		e.stop = Activity(e.C.Err, e.C.T("agent.working", e.Agent, phase))
	}
}

func (e *InterviewEvents) Asked(section string, unknowns int) {
	e.Done()
	if section == "" {
		section = "—"
	}
	e.C.Info(e.C.T("interview.progress", section, unknowns))
}

func (e *InterviewEvents) Answered() { e.Done() }

func (e *InterviewEvents) Rejected(reason string) {
	e.Done()
	e.C.Warn(e.C.T("interview.notdone", reason))
}

// Done stops the activity indicator.
func (e *InterviewEvents) Done() {
	if e.stop != nil {
		e.stop()
		e.stop = nil
	}
}
