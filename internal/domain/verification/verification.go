// Package verification judges what an independent verifier reports about a
// specification: a verdict for every invariant and scenario, observed
// output for every failure. It is the check of the request, not of the
// writer's work: the writer's test can pin a bug, the verifier's probes
// come from the specification.
package verification

import (
	"fmt"
	"slices"
	"strings"
)

// Status of one requirement.
type Status string

const (
	Met        Status = "met"
	Unmet      Status = "unmet"
	Unverified Status = "unverified"
)

// Verdict is the verifier's answer for one requirement: an invariant
// (INV-01) or a scenario (SDD_0001_002).
type Verdict struct {
	ID       string `json:"id"`
	Status   Status `json:"status"`
	Command  string `json:"command,omitempty"`
	Observed string `json:"observed,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// Blocker is a requirement the code does not meet, with the command that
// shows it and what it printed.
type Blocker struct {
	ID       string `json:"id"`
	Command  string `json:"command"`
	Observed string `json:"observed"`
	Expected string `json:"expected"`
}

// RegressionTest is a test the verifier proposes, to keep a requirement
// covered.
type RegressionTest struct {
	Path    string   `json:"path"`
	Covers  []string `json:"covers"`
	Content string   `json:"content"`
}

// Report is what the verifier returns.
type Report struct {
	Verdicts        []Verdict        `json:"verdicts"`
	Blockers        []Blocker        `json:"blockers"`
	Advisories      []string         `json:"advisories"`
	RegressionTests []RegressionTest `json:"regression_tests"`
}

// Problems lists why the report cannot be trusted: a requirement without a
// verdict, an unmet one without a blocker, a blocker without the command
// and the output that show it, or about something that was not asked.
// None means the report is complete.
func (r Report) Problems(required []string) []string {
	var out []string
	verdicts := map[string]Verdict{}
	for _, v := range r.Verdicts {
		verdicts[v.ID] = v
	}
	for _, id := range required {
		if _, ok := verdicts[id]; !ok {
			out = append(out, "no verdict for "+id)
		}
	}
	blocked := map[string]bool{}
	for _, b := range r.Blockers {
		blocked[b.ID] = true
		switch {
		case !slices.Contains(required, b.ID):
			out = append(out, fmt.Sprintf("blocker %s is not a requirement that was asked", b.ID))
		case strings.TrimSpace(b.Command) == "" || strings.TrimSpace(b.Observed) == "":
			out = append(out, fmt.Sprintf("blocker %s has no command or no observed output", b.ID))
		}
	}
	for _, id := range required {
		if v, ok := verdicts[id]; ok && v.Status == Unmet && !blocked[id] {
			out = append(out, fmt.Sprintf("%s is unmet but has no blocker with the command that shows it", id))
		}
	}
	return out
}

// Only keeps the verdicts and blockers about ids, for a re-verification.
func (r Report) Only(ids []string) Report {
	out := Report{Advisories: r.Advisories}
	for _, v := range r.Verdicts {
		if slices.Contains(ids, v.ID) {
			out.Verdicts = append(out.Verdicts, v)
		}
	}
	for _, b := range r.Blockers {
		if slices.Contains(ids, b.ID) {
			out.Blockers = append(out.Blockers, b)
		}
	}
	return out
}

// IDs of the blockers.
func (r Report) BlockerIDs() []string {
	ids := make([]string, len(r.Blockers))
	for i, b := range r.Blockers {
		ids[i] = b.ID
	}
	return ids
}

// Count returns how many verdicts have status s.
func (r Report) Count(s Status) int {
	n := 0
	for _, v := range r.Verdicts {
		if v.Status == s {
			n++
		}
	}
	return n
}
