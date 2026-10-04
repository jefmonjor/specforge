// Package security models the findings of the adversarial security audit
// and decides which of them block a merge.
package security

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Severity of a finding.
type Severity string

const (
	Info     Severity = "info"
	Low      Severity = "low"
	Medium   Severity = "medium"
	High     Severity = "high"
	Critical Severity = "critical"
)

var rank = map[Severity]int{Info: 0, Low: 1, Medium: 2, High: 3, Critical: 4}

// AtLeast reports whether s is as severe as min.
func (s Severity) AtLeast(min Severity) bool { return rank[s] >= rank[min] }

// ParseSeverity validates a --fail-on value. "confirmed", the SpecForge 3
// default, means any confirmed finding and maps to Info.
func ParseSeverity(v string) (Severity, error) {
	s := Severity(strings.ToLower(strings.TrimSpace(v)))
	if s == "confirmed" || s == "" {
		return Info, nil
	}
	if _, ok := rank[s]; !ok {
		return "", fmt.Errorf("unknown severity %q (use info, low, medium, high or critical)", v)
	}
	return s, nil
}

// Status of a finding after the verifier looked at it.
type Status string

const (
	Confirmed       Status = "confirmed"
	NeedsValidation Status = "needs_validation"
	Rejected        Status = "rejected"
)

// Finding is one candidate vulnerability.
type Finding struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Severity      Severity `json:"severity"`
	Status        Status   `json:"status"`
	File          string   `json:"file"`
	Line          int      `json:"line"`
	AttackClass   string   `json:"attack_class"`
	Description   string   `json:"description"`
	ProofOfImpact string   `json:"proof_of_impact"`
	Remediation   string   `json:"remediation,omitempty"`
	// Decision records a developer verdict on a needs_validation finding.
	Decision string `json:"decision,omitempty"`
}

// Report is the audit result, matching report-schema.json.
type Report struct {
	GeneratedAt          string    `json:"generated_at"`
	TotalConfirmed       int       `json:"total_confirmed"`
	TotalNeedsValidation int       `json:"total_needs_validation"`
	TotalRejected        int       `json:"total_rejected"`
	Findings             []Finding `json:"findings"`
}

// Validate checks the fields the schema cannot express.
func (r *Report) Validate() error {
	var errs []error
	ids := map[string]bool{}
	for i, f := range r.Findings {
		if ids[f.ID] {
			errs = append(errs, fmt.Errorf("finding %d: duplicate id %q", i+1, f.ID))
		}
		ids[f.ID] = true
		if _, ok := rank[f.Severity]; !ok {
			errs = append(errs, fmt.Errorf("finding %s: severity %q", f.ID, f.Severity))
		}
		switch f.Status {
		case Confirmed, NeedsValidation, Rejected:
		default:
			errs = append(errs, fmt.Errorf("finding %s: status %q", f.ID, f.Status))
		}
	}
	return errors.Join(errs...)
}

// Recount recomputes the totals from the findings; the model's own totals
// are never trusted.
func (r *Report) Recount() {
	r.TotalConfirmed, r.TotalNeedsValidation, r.TotalRejected = 0, 0, 0
	for _, f := range r.Findings {
		switch f.Status {
		case Confirmed:
			r.TotalConfirmed++
		case NeedsValidation:
			r.TotalNeedsValidation++
		case Rejected:
			r.TotalRejected++
		}
	}
}

// Merge appends other's findings, prefixing their ids with prefix so ids
// from different chunks never collide.
func (r *Report) Merge(other *Report, prefix string) {
	for _, f := range other.Findings {
		if prefix != "" {
			f.ID = prefix + "-" + f.ID
		}
		r.Findings = append(r.Findings, f)
	}
	r.Recount()
}

// Unresolved returns needs_validation findings at least as severe as
// max(threshold, High): the ones a human must judge before merging.
func (r *Report) Unresolved(threshold Severity) []int {
	min := High
	if threshold.AtLeast(High) {
		min = threshold
	}
	var idx []int
	for i, f := range r.Findings {
		if f.Status == NeedsValidation && f.Severity.AtLeast(min) && f.Decision == "" {
			idx = append(idx, i)
		}
	}
	return idx
}

// Blocking returns the findings that fail the audit: confirmed findings at
// or above threshold, and needs_validation findings at or above
// max(threshold, High) that nobody cleared. Fail closed: doubt blocks.
func (r *Report) Blocking(threshold Severity) []Finding {
	unresolved := map[int]bool{}
	for _, i := range r.Unresolved(threshold) {
		unresolved[i] = true
	}
	var out []Finding
	for i, f := range r.Findings {
		if (f.Status == Confirmed && f.Severity.AtLeast(threshold)) || unresolved[i] {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return rank[out[a].Severity] > rank[out[b].Severity] })
	return out
}
