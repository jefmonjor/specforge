package review

import (
	"fmt"
	"strings"
)

// Severity of a finding.
type Severity string

const (
	Blocker    Severity = "BLOCKER"
	Critical   Severity = "CRITICAL"
	Warning    Severity = "WARNING"
	Suggestion Severity = "SUGGESTION"
)

// Severe reports a finding that may block.
func (s Severity) Severe() bool { return s == Blocker || s == Critical }

// Evidence says how the lens knows.
type Evidence string

const (
	// Deterministic: it follows from the code without assumptions.
	Deterministic Evidence = "deterministic"
	// Inferential: it depends on how the code is used; it is refuted
	// before it can block.
	Inferential Evidence = "inferential"
	// Insufficient: the lens could not tell.
	Insufficient Evidence = "insufficient"
)

// Causal says whether the change caused the problem.
type Causal string

const (
	Introduced        Causal = "introduced"
	BehaviorActivated Causal = "behavior-activated"
	Worsened          Causal = "worsened"
	PreExisting       Causal = "pre-existing"
	BaseOnly          Causal = "base-only"
	Unknown           Causal = "unknown"
)

func (c Causal) caused() bool {
	return c == Introduced || c == BehaviorActivated || c == Worsened
}

// Proof kinds.
const (
	// ProofChangedHunk points at a line the change added or modified.
	ProofChangedHunk = "changed-hunk"
	// ProofNewFile points at a file the change created.
	ProofNewFile = "new-file"
)

// ProofRef is where the evidence of a finding is.
type ProofRef struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
}

func (p ProofRef) String() string {
	if p.Line > 0 {
		return fmt.Sprintf("%s:%d", p.Path, p.Line)
	}
	return p.Path
}

// Location is where a finding is.
type Location struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

// Finding is one problem a review lens reports.
type Finding struct {
	ID       string     `json:"id"`
	Lens     string     `json:"lens,omitempty"`
	Severity Severity   `json:"severity"`
	Location Location   `json:"location"`
	Claim    string     `json:"claim"`
	Evidence Evidence   `json:"evidence_class"`
	Causal   Causal     `json:"causal_disposition"`
	Proof    []ProofRef `json:"proof_refs"`
}

// Discarded is a finding dropped with the reason.
type Discarded struct {
	Finding
	Reason string `json:"reason"`
}

// Verdict sorts findings by what happens to them.
type Verdict struct {
	// Blocking are severe findings the change caused, with valid proof:
	// they are corrected (deterministic) or refuted first (inferential).
	Blocking []Finding `json:"blocking,omitempty"`
	// FollowUps were there before the change: never blocking.
	FollowUps []Finding `json:"follow_ups,omitempty"`
	// Escalated are severe findings whose cause or evidence is unclear:
	// the developer decides.
	Escalated []Finding `json:"escalated,omitempty"`
	// Info are warnings and suggestions.
	Info []Finding `json:"info,omitempty"`
	// Discarded have no proof inside the change, or were refuted.
	Discarded []Discarded `json:"discarded,omitempty"`
}

// Verify checks every finding's proof against the diff and sorts the
// findings. A finding with no proof inside the change is discarded, the
// same way an invented legacy citation is: a claim about a line the change
// did not touch is not about the change. Duplicate IDs keep the first.
func Verify(findings []Finding, d Diff) Verdict {
	var v Verdict
	seen := map[string]bool{}
	for _, f := range findings {
		if seen[f.ID] {
			v.Discarded = append(v.Discarded, Discarded{f, "duplicate id " + f.ID})
			continue
		}
		seen[f.ID] = true
		if why := proofProblem(f, d); why != "" {
			v.Discarded = append(v.Discarded, Discarded{f, why})
			continue
		}
		switch {
		case !f.Severity.Severe():
			v.Info = append(v.Info, f)
		case f.Causal == PreExisting || f.Causal == BaseOnly:
			v.FollowUps = append(v.FollowUps, f)
		case !f.Causal.caused() || f.Evidence == Insufficient:
			v.Escalated = append(v.Escalated, f)
		default:
			v.Blocking = append(v.Blocking, f)
		}
	}
	return v
}

// proofProblem returns why a finding's proof does not hold, or "".
func proofProblem(f Finding, d Diff) string {
	if len(f.Proof) == 0 {
		return "no proof_refs"
	}
	var bad []string
	for _, p := range f.Proof {
		switch {
		case p.Kind == ProofNewFile && d.IsNew(p.Path):
		case p.Kind == ProofChangedHunk && d.Changed(p.Path, p.Line):
		case p.Kind == ProofNewFile:
			bad = append(bad, p.Path+" is not a file the change created")
		case p.Kind == ProofChangedHunk:
			bad = append(bad, p.String()+" is not a changed line")
		default:
			bad = append(bad, "unknown proof kind "+p.Kind)
		}
	}
	if len(bad) == len(f.Proof) {
		return "no proof inside the change: " + strings.Join(bad, "; ")
	}
	return ""
}

// Inferential returns the blocking findings that need a refuter.
func (v Verdict) Inferential() []Finding {
	var out []Finding
	for _, f := range v.Blocking {
		if f.Evidence == Inferential {
			out = append(out, f)
		}
	}
	return out
}

// Refute applies the refuter's verdicts: a refuted finding is discarded
// with the reason, a confirmed one keeps blocking. An inferential finding
// the refuter did not answer is not confirmed: it is escalated.
func (v Verdict) Refute(confirmed map[string]bool, reasons map[string]string) Verdict {
	var keep []Finding
	for _, f := range v.Blocking {
		if f.Evidence != Inferential {
			keep = append(keep, f)
			continue
		}
		ok, answered := confirmed[f.ID]
		switch {
		case !answered:
			v.Escalated = append(v.Escalated, f)
		case ok:
			keep = append(keep, f)
		default:
			v.Discarded = append(v.Discarded, Discarded{f, "refuted: " + reasons[f.ID]})
		}
	}
	v.Blocking = keep
	return v
}

// CorrectionBudget is how many lines one correction may change: half the
// change it corrects, at most 200. A correction larger than that is a
// redesign, and redesigns go to the developer.
func CorrectionBudget(authoredLines int) int {
	budget := (authoredLines + 1) / 2
	return max(min(budget, 200), 1)
}
