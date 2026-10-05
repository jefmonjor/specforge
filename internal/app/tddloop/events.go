package tddloop

import (
	"specforge/internal/domain/quality"
	"specforge/internal/domain/risk"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
)

// Rejection says why an attempt was not accepted.
type Rejection string

const (
	RejectNoContract    Rejection = "no-contract"
	RejectFalseClaim    Rejection = "false-claim"
	RejectNoTest        Rejection = "no-test"
	RejectNoMarker      Rejection = "no-marker"
	RejectNotCompiled   Rejection = "not-compiled"
	RejectNothingRan    Rejection = "nothing-ran"
	RejectStillFailing  Rejection = "still-failing"
	RejectPremature     Rejection = "premature-green"
	RejectGatesBlocking Rejection = "gates-blocking"
	RejectUnconfirmed   Rejection = "unconfirmed"
	RejectReviewChange  Rejection = "review-change"
	RejectReviewRed     Rejection = "review-red"
	RejectNewFailures   Rejection = "new-failures"
	RejectNoOptions     Rejection = "no-options"
	RejectOutsidePlan   Rejection = "outside-plan"
)

// Events lets the presentation layer follow the loop. Implementations must
// not block.
type Events interface {
	Started(st *tdd.State, doc *spec.Document)
	Amended(pending []string)
	// Orphaned reports tests still carrying the marker of a scenario the
	// specification no longer has.
	Orphaned(tests []string)
	// Baseline reports the tests that already failed before the loop (nil
	// when the runner cannot name them) and whether the project built.
	Baseline(b *tdd.Baseline, builds bool)
	Phase(st *tdd.State, sc tdd.ScenarioRef)
	AgentWorking(phase tdd.Phase)
	RunningTests(command string)
	Rejected(reason Rejection, detail string)
	Answered(question, answer string)
	Gates(report quality.Report)
	// Risk reports the risk assessment of a scenario's change.
	Risk(sc tdd.ScenarioRef, a risk.Assessment)
	// GateNotRun reports a gate left out for a change of this tier.
	GateNotRun(gate string, tier risk.Tier)
	// ReviewSkipped reports a scenario accepted without the developer's
	// review because its change is passive (review: risk).
	ReviewSkipped(sc tdd.ScenarioRef, a risk.Assessment)
	// Reviewed reports the outcome of a scenario's lens review.
	Reviewed(sc tdd.ScenarioRef, rec tdd.ReviewRecord)
	// Verified reports the independent verification of a scenario.
	Verified(sc tdd.ScenarioRef, rec tdd.VerifyRecord)
	Accepted(phase tdd.Phase, sc tdd.ScenarioRef)
	Satisfied(sc tdd.ScenarioRef)
	// Committed reports the commit that recorded a scenario; sha is ""
	// when committing is off or the project is not a git repository.
	Committed(sc tdd.ScenarioRef, sha string)
	Finished(st *tdd.State)
	// Parallel reports scenarios starting side by side, each in a sandbox.
	Parallel(markers []string)
	// ParallelSkipped reports a scenario of a batch that runs again on its
	// own, and why.
	ParallelSkipped(sc tdd.ScenarioRef, why string)
	// SeamFailed reports a batch whose combined work fails the suite.
	SeamFailed(failure string)
	// Integrated reports a scenario of a batch brought into the project.
	Integrated(sc tdd.ScenarioRef)
}

// NopEvents ignores every event.
type NopEvents struct{}

func (NopEvents) Started(*tdd.State, *spec.Document)             {}
func (NopEvents) Amended([]string)                               {}
func (NopEvents) Orphaned([]string)                              {}
func (NopEvents) Baseline(*tdd.Baseline, bool)                   {}
func (NopEvents) Phase(*tdd.State, tdd.ScenarioRef)              {}
func (NopEvents) AgentWorking(tdd.Phase)                         {}
func (NopEvents) RunningTests(string)                            {}
func (NopEvents) Rejected(Rejection, string)                     {}
func (NopEvents) Answered(string, string)                        {}
func (NopEvents) Gates(quality.Report)                           {}
func (NopEvents) Risk(tdd.ScenarioRef, risk.Assessment)          {}
func (NopEvents) GateNotRun(string, risk.Tier)                   {}
func (NopEvents) ReviewSkipped(tdd.ScenarioRef, risk.Assessment) {}
func (NopEvents) Reviewed(tdd.ScenarioRef, tdd.ReviewRecord)     {}
func (NopEvents) Verified(tdd.ScenarioRef, tdd.VerifyRecord)     {}
func (NopEvents) Accepted(tdd.Phase, tdd.ScenarioRef)            {}
func (NopEvents) Satisfied(tdd.ScenarioRef)                      {}
func (NopEvents) Committed(tdd.ScenarioRef, string)              {}
func (NopEvents) Finished(*tdd.State)                            {}
func (NopEvents) Parallel([]string)                              {}
func (NopEvents) ParallelSkipped(tdd.ScenarioRef, string)        {}
func (NopEvents) SeamFailed(string)                              {}
func (NopEvents) Integrated(tdd.ScenarioRef)                     {}
