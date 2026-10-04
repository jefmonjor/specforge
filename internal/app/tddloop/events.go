package tddloop

import (
	"specforge/internal/domain/quality"
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
)

// Events lets the presentation layer follow the loop. Implementations must
// not block.
type Events interface {
	Started(st *tdd.State, doc *spec.Document)
	Amended(pending []string)
	Phase(st *tdd.State, sc tdd.ScenarioRef)
	AgentWorking(phase tdd.Phase)
	RunningTests(command string)
	Rejected(reason Rejection, detail string)
	Answered(question, answer string)
	Gates(report quality.Report)
	Accepted(phase tdd.Phase, sc tdd.ScenarioRef)
	Satisfied(sc tdd.ScenarioRef)
	// Committed reports the commit that recorded a scenario; sha is ""
	// when committing is off or the project is not a git repository.
	Committed(sc tdd.ScenarioRef, sha string)
	Finished(st *tdd.State)
}

// NopEvents ignores every event.
type NopEvents struct{}

func (NopEvents) Started(*tdd.State, *spec.Document)  {}
func (NopEvents) Amended([]string)                    {}
func (NopEvents) Phase(*tdd.State, tdd.ScenarioRef)   {}
func (NopEvents) AgentWorking(tdd.Phase)              {}
func (NopEvents) RunningTests(string)                 {}
func (NopEvents) Rejected(Rejection, string)          {}
func (NopEvents) Answered(string, string)             {}
func (NopEvents) Gates(quality.Report)                {}
func (NopEvents) Accepted(tdd.Phase, tdd.ScenarioRef) {}
func (NopEvents) Satisfied(tdd.ScenarioRef)           {}
func (NopEvents) Committed(tdd.ScenarioRef, string)   {}
func (NopEvents) Finished(*tdd.State)                 {}
