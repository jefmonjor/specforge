package tdd

// Outcome is the verdict of one test run, parsed from the runner's
// machine-readable report rather than guessed from its exit code.
type Outcome struct {
	// Compiled is false when the tests could not be built or collected:
	// that is never a valid RED.
	Compiled bool
	Passed   int
	Failed   int
	Skipped  int
	// Failures names the tests that failed, when the report has names.
	// Failed can be larger: a runner may count what it cannot name.
	Failures []TestRef
	// Exact is false when the runner offers no machine-readable report and
	// the verdict comes from the exit code alone.
	Exact bool
	// Output is the relevant part of the runner's output: build errors and
	// the output of failing tests.
	Output string
}

// Ran is the number of tests that executed.
func (o Outcome) Ran() int { return o.Passed + o.Failed }

// Green reports a successful run of at least one test.
func (o Outcome) Green() bool { return o.Compiled && o.Failed == 0 && o.Ran() > 0 }

// RedVerdict classifies a run made right after the agent wrote the test.
type RedVerdict int

const (
	// RedValid: the tests compiled, ran, and at least one failed.
	RedValid RedVerdict = iota
	// RedNotCompiled: the test does not build or cannot be collected.
	RedNotCompiled
	// RedNothingRan: no test carrying the scenario marker executed.
	RedNothingRan
	// RedPremature: the test passed before any implementation.
	RedPremature
)

// Red classifies o as a RED step.
func (o Outcome) Red() RedVerdict {
	switch {
	case !o.Compiled:
		return RedNotCompiled
	case o.Ran() == 0:
		return RedNothingRan
	case o.Failed == 0:
		return RedPremature
	default:
		return RedValid
	}
}
