package tddloop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"specforge/internal/app/protocol"
	"specforge/internal/domain/change"
	"specforge/internal/domain/risk"
	"specforge/internal/ports"
)

// assess classifies the scenario's change from what version control says
// it touched, applies the agent's request for more scrutiny, and keeps the
// result in the state. quiet skips the event when the tier did not change.
func (s *Service) assess(ctx context.Context, r *run, quiet bool) (risk.Assessment, error) {
	files, err := s.changes(ctx, r)
	if err != nil {
		return risk.Assessment{}, err
	}
	ref := &r.st.Scenarios[r.st.Current]
	a := risk.Classify(files, r.o.Risk).Escalate(ref.RaisedTo, ref.RaisedWhy)
	changed := ref.Risk == nil || ref.Risk.Tier != a.Tier
	ref.Risk = &a
	if changed || !quiet {
		r.st.Record("risk", string(a.Tier), strings.Join(a.Reasons, "; "), s.d.Now())
		s.d.Events.Risk(*ref, a)
	}
	return a, nil
}

// changes measures the scenario's files: with git when the project is a
// repository, otherwise by counting the lines of each file as new.
func (s *Service) changes(ctx context.Context, r *run) ([]change.File, error) {
	paths := sortedStrings(r.st.FilesWritten)
	if s.d.VCS != nil {
		files, err := s.d.VCS.Changes(ctx, r.o.Root, paths)
		if !errors.Is(err, ports.ErrNotARepository) {
			return files, err
		}
	}
	var files []change.File
	for _, p := range paths {
		if data, err := s.d.Files.ReadFile(r.lay.Abs(p)); err == nil {
			files = append(files, change.FromContent(p, data))
		}
	}
	return files, nil
}

// raise keeps the agent's request for more scrutiny and records it as a
// decision. The tier itself is applied by the next assessment.
func (s *Service) raise(r *run, resp protocol.Response) error {
	to, err := risk.ParseTier(resp.Risk)
	if err != nil || !r.st.Raise(to, resp.RiskReason) {
		return nil // validated by the contract; a lower request changes nothing
	}
	sc, _ := r.st.Scenario()
	q := ports.Question{Text: fmt.Sprintf(question(r.o.Language, "raised").text, sc.Index)}
	r.st.Record("risk", "raised by the agent", string(to)+": "+resp.RiskReason, s.d.Now())
	return s.d.Asker.Record(s.originAs(r, sc, OriginRisk), q, string(to)+" · "+resp.RiskReason)
}

// needsReview reports whether the developer reviews this scenario (R2).
func needsReview(mode string, a *risk.Assessment) bool {
	switch mode {
	case ReviewScenario:
		return true
	case ReviewRisk:
		return a == nil || a.Tier.AtLeast(risk.Medium)
	default:
		return false
	}
}

// gateRuns reports whether a gate runs for a change of tier t. Only the
// mutation gate is proportional: it is the slow one.
func gateRuns(name string, t risk.Tier, mutationFrom risk.Tier) bool {
	return name != "mutation" || t.AtLeast(mutationFrom)
}
