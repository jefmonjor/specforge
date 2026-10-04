// Package quality models the result of the REFACTOR quality gates.
//
// A gate has three outcomes, not two: a gate whose tool could not run is
// Skipped, never Passed. In strict mode a skipped gate blocks like a failed
// one.
package quality

import (
	"fmt"
	"strings"
)

// Status of one gate.
type Status string

const (
	Passed  Status = "passed"
	Failed  Status = "failed"
	Skipped Status = "skipped"
)

// Result of one gate.
type Result struct {
	Gate    string
	Status  Status
	Summary string
	// Details is the tool output that explains a failure.
	Details string
}

// Report is the result of every gate run for a project.
type Report []Result

// Blocking returns the results that stop the loop: failures, and skipped
// gates when strict is set.
func (r Report) Blocking(strict bool) []Result {
	var out []Result
	for _, res := range r {
		if res.Status == Failed || (strict && res.Status == Skipped) {
			out = append(out, res)
		}
	}
	return out
}

// OK reports whether nothing blocks.
func (r Report) OK(strict bool) bool { return len(r.Blocking(strict)) == 0 }

// Explain renders the blocking results for a prompt or a terminal.
func (r Report) Explain(strict bool) string {
	var b strings.Builder
	for _, res := range r.Blocking(strict) {
		fmt.Fprintf(&b, "[%s] %s: %s\n", res.Status, res.Gate, res.Summary)
		if d := strings.TrimSpace(res.Details); d != "" {
			b.WriteString(d + "\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

// Thresholds are the project's numeric quality bars.
type Thresholds struct {
	// MaxDuplicationPercent is the highest share of duplicated lines allowed.
	MaxDuplicationPercent float64 `yaml:"max_duplication_percent" json:"max_duplication_percent"`
	// MinMutationScore is the lowest Stryker mutation score allowed.
	MinMutationScore float64 `yaml:"min_mutation_score" json:"min_mutation_score"`
	// Strict turns skipped gates into blocking ones.
	Strict bool `yaml:"strict" json:"strict"`
}

// DefaultThresholds matches the SpecForge constitution: no duplication and
// a mutation score of at least 80%.
func DefaultThresholds() Thresholds {
	return Thresholds{MaxDuplicationPercent: 0, MinMutationScore: 80}
}

// MutationScore computes Stryker's score: detected mutants over valid ones.
func MutationScore(killed, timeout, survived, noCoverage int) float64 {
	valid := killed + timeout + survived + noCoverage
	if valid == 0 {
		return 100
	}
	return float64(killed+timeout) * 100 / float64(valid)
}
