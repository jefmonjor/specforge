package tdd

import (
	"slices"
	"strings"
	"time"
)

// TestRef names one test the way its runner reports it: the suite (Go
// package, JUnit class, test file) and the test inside it.
type TestRef struct {
	Suite string `json:"suite,omitempty"`
	Name  string `json:"name"`
}

// String renders the reference for people: "suite › name".
func (t TestRef) String() string {
	if t.Suite == "" {
		return t.Name
	}
	return t.Suite + " › " + t.Name
}

// Baseline is what already failed before the loop changed anything. Those
// tests are evidence about the branch, not the agent's job: they never
// block REFACTOR, and the agent is told to leave them alone.
type Baseline struct {
	At       time.Time `json:"at"`
	Command  string    `json:"command"`
	Failures []TestRef `json:"failures,omitempty"`
}

// NewBaseline records the failures of a full-suite run. It returns nil when
// the run cannot name them (it did not build, or the runner has no report):
// a failure without a name cannot be told apart from a new one later.
func NewBaseline(o Outcome, command string, at time.Time) *Baseline {
	if !o.Compiled || !o.Exact || o.Failed > len(o.Failures) {
		return nil
	}
	return &Baseline{At: at, Command: command, Failures: sortedRefs(o.Failures)}
}

// Has reports whether t failed in the baseline.
func (b *Baseline) Has(t TestRef) bool {
	return b != nil && slices.Contains(b.Failures, t)
}

// SuiteVerdict is a full-suite run judged against the baseline.
type SuiteVerdict struct {
	// OK: the suite built, ran, and every failure is a known one.
	OK bool
	// Fresh are failures the baseline does not have: they block.
	Fresh []TestRef
	// Known are baseline failures that still fail: they warn.
	Known []TestRef
	// Fixed are baseline failures that now pass: Retire drops them.
	Fixed []TestRef
}

// Judge compares a full-suite run with the baseline. Without a baseline it
// is the plain verdict: every failure blocks. A failure the runner counted
// but could not name is treated as fresh, so nothing slips through.
func (b *Baseline) Judge(o Outcome) SuiteVerdict {
	if b == nil {
		return SuiteVerdict{OK: o.Green(), Fresh: sortedRefs(o.Failures)}
	}
	var v SuiteVerdict
	failing := map[TestRef]bool{}
	for _, f := range o.Failures {
		failing[f] = true
		if b.Has(f) {
			v.Known = append(v.Known, f)
		} else {
			v.Fresh = append(v.Fresh, f)
		}
	}
	if o.Compiled && o.Exact {
		for _, f := range b.Failures {
			if !failing[f] {
				v.Fixed = append(v.Fixed, f)
			}
		}
	}
	unnamed := o.Failed - len(o.Failures)
	v.OK = o.Compiled && o.Ran() > 0 && len(v.Fresh) == 0 && unnamed <= 0
	v.Fresh, v.Known, v.Fixed = sortedRefs(v.Fresh), sortedRefs(v.Known), sortedRefs(v.Fixed)
	return v
}

// Retire drops baseline failures that pass now: once fixed, a test that
// fails again is a regression like any other.
func (b *Baseline) Retire(fixed []TestRef) {
	if b == nil || len(fixed) == 0 {
		return
	}
	b.Failures = slices.DeleteFunc(b.Failures, func(t TestRef) bool { return slices.Contains(fixed, t) })
}

// Names renders refs for messages and prompts, one per line.
func Names(refs []TestRef) string {
	lines := make([]string, len(refs))
	for i, r := range refs {
		lines[i] = r.String()
	}
	return strings.Join(lines, "\n")
}

func sortedRefs(in []TestRef) []TestRef {
	if len(in) == 0 {
		return nil
	}
	out := slices.Clone(in)
	slices.SortFunc(out, func(a, b TestRef) int {
		if c := strings.Compare(a.Suite, b.Suite); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return slices.Compact(out)
}
