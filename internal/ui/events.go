package ui

import (
	"fmt"
	"strings"
	"sync"

	"specforge/internal/app/audit"
	"specforge/internal/app/tddloop"
	"specforge/internal/domain/e2e"
	"specforge/internal/domain/quality"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
)

// LoopEvents prints the TDD loop.
type LoopEvents struct {
	C     *Console
	Agent string
	Stack string

	mu   sync.Mutex
	stop func()
}

var _ tddloop.Events = (*LoopEvents)(nil)

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
	stop  func()
}

func (e *InterviewEvents) Working() {
	e.Done()
	if !e.C.Quiet {
		e.stop = Activity(e.C.Err, e.C.T("agent.working", e.Agent, "INTERVIEW"))
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
