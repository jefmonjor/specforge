package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"specforge/internal/domain/review"
	"specforge/internal/domain/risk"
	"specforge/internal/ports"
)

// BlockedError reports a branch review whose findings block: severe ones
// the change caused (refuted findings excluded), or ones whose cause the
// lenses could not establish. Outside the loop there is no automatic
// correction: the developer fixes them and reviews again.
type BlockedError struct {
	Blocking, Escalated []review.Finding
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("%d review finding(s) block and %d need your judgement", len(e.Blocking), len(e.Escalated))
}

// BranchOptions select a branch review.
type BranchOptions struct {
	Root, Base, Language, Stack string
	// LensesAuto picks the lenses by risk; otherwise Lenses run.
	LensesAuto bool
	Lenses     []review.Lens
	Rules      risk.Rules
	// SpecTitle, Invariants and Plan give context when the branch
	// implements a specification.
	SpecTitle, Invariants, Plan string
	// Report is where the review is written.
	Report  string
	Model   ports.ModelFor
	Timeout time.Duration
	Now     time.Time
}

// BranchResult is a finished branch review.
type BranchResult struct {
	Base   string          `json:"base"`
	At     time.Time       `json:"at"`
	Risk   risk.Assessment `json:"risk"`
	Result `json:"review"`
	// Empty is true when the branch has no changes.
	Empty bool `json:"-"`
}

// Branch reviews a whole branch against its base: for code that did not
// come out of the loop. The report is written whatever the outcome.
func (s *Service) Branch(ctx context.Context, diffs ports.DiffSource, files ports.Files, o BranchOptions) (BranchResult, error) {
	base := o.Base
	if base == "" {
		var err error
		if base, err = diffs.DefaultBase(ctx, o.Root); err != nil {
			return BranchResult{}, err
		}
	}
	text, err := diffs.Diff(ctx, o.Root, base)
	if err != nil {
		return BranchResult{}, err
	}
	d, err := review.ParseDiff(text)
	if err != nil {
		return BranchResult{}, err
	}
	out := BranchResult{Base: base, At: o.Now.UTC()}
	if len(d.Files) == 0 {
		out.Empty = true
		return out, nil
	}
	out.Risk = risk.Classify(d.Changes(), o.Rules)
	lenses := o.Lenses
	if o.LensesAuto {
		lenses = review.Select(out.Risk.Tier, d.Files, o.Rules.Sensitive, func(p string) bool { return !o.Rules.Documentation(p) })
	}
	if len(lenses) == 0 {
		out.Result = Result{}
		return out, s.write(files, o.Report, out)
	}
	res, err := s.Review(ctx, Request{
		Root: o.Root, Language: o.Language, Stack: o.Stack, Lenses: lenses, Diff: text,
		SpecTitle: o.SpecTitle, Invariants: o.Invariants, Plan: o.Plan,
		Model: o.Model, Timeout: o.Timeout,
	})
	if err != nil {
		return out, err
	}
	out.Result = res
	if err := s.write(files, o.Report, out); err != nil {
		return out, err
	}
	if v := res.Verdict; len(v.Blocking) > 0 || len(v.Escalated) > 0 {
		return out, &BlockedError{Blocking: v.Blocking, Escalated: v.Escalated}
	}
	return out, nil
}

func (s *Service) write(files ports.Files, path string, r BranchResult) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return files.WriteFile(path, append(data, '\n'))
}
