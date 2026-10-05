// Package delivery models what a finished specification hands over: a
// trace from each scenario to its tests, files, commit and gates, plus the
// decisions taken, the questions still open and the checks that ran. It is
// built only from artifacts SpecForge wrote along the way, never from the
// agent's word, and it says what is missing as clearly as what is done.
package delivery

import "time"

// Status of a scenario in the delivery.
type Status string

const (
	Done      Status = "done"      // went through RED → GREEN → REFACTOR
	Satisfied Status = "satisfied" // the behaviour existed; the test documents it
	Pending   Status = "pending"   // not finished
)

// Approval is who approved a document, when, and the seal.
type Approval struct {
	Path       string `json:"path"`
	ApprovedBy string `json:"approved_by,omitempty"`
	ApprovedAt string `json:"approved_at,omitempty"`
	Seal       string `json:"seal,omitempty"`
}

// Test is a test file and the test names that carry the scenario marker.
type Test struct {
	File  string   `json:"file"`
	Names []string `json:"names,omitempty"`
}

// Scenario is one row of the trace.
type Scenario struct {
	Index       int      `json:"index"`
	Marker      string   `json:"marker"`
	Title       string   `json:"title"`
	Status      Status   `json:"status"`
	Tests       []Test   `json:"tests,omitempty"`
	Files       []string `json:"files,omitempty"`
	Commit      string   `json:"commit,omitempty"`
	Gates       string   `json:"gates,omitempty"`
	Rejections  int      `json:"rejections"`
	ReviewNotes int      `json:"review_changes"`
	// Risk is the tier of the scenario's change and why; nil when the loop
	// did not assess it.
	Risk *Risk `json:"risk,omitempty"`
	// Lines are the authored lines of the scenario's commit (lock files,
	// vendored and golden files excluded).
	Lines int `json:"lines"`
}

// Risk is how much scrutiny a scenario's change got, and why.
type Risk struct {
	Tier    string   `json:"tier"`
	Lines   int      `json:"lines"`
	Reasons []string `json:"reasons"`
}

// Checks summarises the verifications that ran outside the loop.
type Checks struct {
	// Security is nil when no audit report exists.
	Security *Security `json:"security,omitempty"`
	// E2E is nil when no E2E report exists for the specification.
	E2E *E2E `json:"e2e,omitempty"`
}

// Security is the audit summary.
type Security struct {
	Confirmed       int `json:"confirmed"`
	NeedsValidation int `json:"needs_validation"`
	Rejected        int `json:"rejected"`
}

// E2E is the browser verification summary.
type E2E struct {
	Scenarios int     `json:"scenarios"`
	Passed    int     `json:"passed"`
	PassRate  float64 `json:"pass_rate"`
}

// Baseline is what already failed before the loop changed anything.
type Baseline struct {
	At       time.Time `json:"at"`
	Command  string    `json:"command"`
	Failures []string  `json:"failures"`
}

// Trace is the machine-readable delivery (trace.json).
type Trace struct {
	GeneratedAt time.Time  `json:"generated_at"`
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Spec        Approval   `json:"spec"`
	Plan        *Approval  `json:"plan,omitempty"`
	Scenarios   []Scenario `json:"scenarios"`
	Decisions   []string   `json:"decisions,omitempty"`
	Pending     []string   `json:"pending_questions,omitempty"`
	Lessons     []string   `json:"lessons,omitempty"`
	OutOfScope  string     `json:"out_of_scope,omitempty"`
	Checks      Checks     `json:"checks"`
	// Baseline is nil when the loop never ran or its runner cannot name
	// failing tests.
	Baseline *Baseline `json:"baseline,omitempty"`
	// Budget is the size of a reviewable pull request, in authored lines;
	// Slices are proposed when the delivery is larger.
	Budget int     `json:"budget_lines"`
	Slices []Slice `json:"slices,omitempty"`
}

// Count returns how many scenarios have status s.
func (t Trace) Count(s Status) int {
	n := 0
	for _, sc := range t.Scenarios {
		if sc.Status == s {
			n++
		}
	}
	return n
}

// Complete reports whether every scenario is done or satisfied and no
// question is waiting.
func (t Trace) Complete() bool {
	return t.Count(Pending) == 0 && len(t.Pending) == 0 && len(t.Scenarios) > 0
}
