package delivery

// Slice is a run of consecutive scenarios small enough to review as one
// pull request. Slices stack: each one's branch starts where the previous
// one ends.
type Slice struct {
	N         int    `json:"n"`
	Scenarios []int  `json:"scenarios"`
	Lines     int    `json:"lines"`
	FirstSHA  string `json:"first_commit"`
	LastSHA   string `json:"last_commit"`
	// Oversized marks a single scenario larger than the budget: it is
	// flagged, never cut in half and never "thinned" to fit.
	Oversized bool `json:"oversized,omitempty"`
}

// Slices groups the committed scenarios, in order, into slices of at most
// budget authored lines. Scenarios without a commit are left out: there
// is nothing to put on a branch.
func Slices(scenarios []Scenario, budget int) []Slice {
	var out []Slice
	var cur *Slice
	for _, sc := range scenarios {
		if sc.Commit == "" {
			continue
		}
		if cur != nil && (cur.Lines+sc.Lines > budget || cur.Oversized) {
			out = append(out, *cur)
			cur = nil
		}
		if cur == nil {
			cur = &Slice{N: len(out) + 1, FirstSHA: sc.Commit}
		}
		cur.Scenarios = append(cur.Scenarios, sc.Index)
		cur.Lines += sc.Lines
		cur.LastSHA = sc.Commit
		cur.Oversized = len(cur.Scenarios) == 1 && sc.Lines > budget
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

// Lines adds up the authored lines of every scenario.
func (t Trace) Lines() int {
	n := 0
	for _, sc := range t.Scenarios {
		n += sc.Lines
	}
	return n
}

// OverBudget reports a delivery too large for one review.
func (t Trace) OverBudget() bool { return t.Budget > 0 && t.Lines() > t.Budget }

// Sub is the trace of one slice: its scenarios only, everything else as
// it is, for the slice's own pull request body.
func (t Trace) Sub(s Slice) Trace {
	sub := t
	sub.Scenarios = nil
	for _, sc := range t.Scenarios {
		for _, i := range s.Scenarios {
			if sc.Index == i {
				sub.Scenarios = append(sub.Scenarios, sc)
			}
		}
	}
	sub.Slices = nil
	return sub
}
