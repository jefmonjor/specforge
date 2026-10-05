package tddloop

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jefmonjor/specforge/v6/internal/app/clarify"
	"github.com/jefmonjor/specforge/v6/internal/domain/tdd"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// Review modes.
const (
	ReviewScenario = "scenario"
	ReviewRisk     = "risk"
	ReviewOff      = "off"
)

// close finishes a scenario whose REFACTOR passed: the developer reviews it
// (gate R2) and it is recorded as one commit. A review can send the
// scenario back to GREEN with a requested change, or to RED.
func (s *Service) close(ctx context.Context, r *run, sc tdd.ScenarioRef, gates string) error {
	a, err := s.assess(ctx, r, true)
	if err != nil {
		return err
	}
	if !needsReview(r.o.Review, &a) && r.o.Review != ReviewOff {
		r.st.Record("review", "not needed", "passive change: "+strings.Join(a.Reasons, "; "), s.d.Now())
		s.d.Events.ReviewSkipped(sc, a)
	}
	if needsReview(r.o.Review, &a) {
		back, note, err := s.review(ctx, r, sc, gates)
		if err != nil {
			return err
		}
		switch back {
		case tdd.PhaseGreen:
			r.st.SendBack(tdd.PhaseGreen, feedback(r.o.Language, RejectReviewChange, note), s.d.Now())
			r.st.Record("review", "change requested", note, s.d.Now())
			s.d.Events.Rejected(RejectReviewChange, note)
			return s.save(r)
		case tdd.PhaseRed:
			r.st.SendBack(tdd.PhaseRed, feedback(r.o.Language, RejectReviewRed), s.d.Now())
			r.st.Record("review", "back to RED", "", s.d.Now())
			s.d.Events.Rejected(RejectReviewRed, "")
			return s.save(r)
		}
		r.st.Record("review", "accepted", "", s.d.Now())
	}

	files, err := s.scenarioFiles(ctx, r)
	if err != nil {
		return err
	}
	sha, err := s.commit(ctx, r, sc, files, "feat")
	if err != nil {
		return err
	}
	r.st.Scenarios[r.st.Current].Files = files
	r.st.Scenarios[r.st.Current].Commit = sha
	if sha != "" {
		r.st.Record("commit", "recorded", sha, s.d.Now())
	}
	s.d.Events.Committed(sc, sha)
	r.st.Advance(s.d.Now())
	return s.save(r)
}

// review asks the developer to accept the scenario. It returns the phase to
// go back to ("" to accept) and the requested change.
func (s *Service) review(ctx context.Context, r *run, sc tdd.ScenarioRef, gates string) (tdd.Phase, string, error) {
	q := question(r.o.Language, "review")
	files := strings.Join(slices.Sorted(slices.Values(r.st.FilesWritten)), ", ")
	context := strings.TrimSpace(fmt.Sprintf("%s: %s · gates: %s", sc.Marker, files, gates))
	answer, err := s.d.Asker.Ask(ctx, s.originAs(r, sc, OriginReview), ports.Question{Text: fmt.Sprintf(q.text, sc.Index, sc.Title), Context: context, Options: q.options})
	if err != nil {
		var pending *clarify.PendingQuestionError
		if errors.As(err, &pending) {
			r.st.Pending = &tdd.Pending{Kind: tdd.PendingReview, Phase: r.st.Phase, Scenario: sc.Index, Context: gates}
		}
		return "", "", err
	}
	switch pick(answer, q.options) {
	case 0:
		return "", "", nil
	case 1:
		return tdd.PhaseRed, "", nil
	}
	return tdd.PhaseGreen, strings.TrimSpace(answer), nil
}

// commit records the scenario's files: kind "feat" for a scenario that went
// through the loop, "test" for one that was already satisfied. Committing
// is skipped when it is off or the project is not a git repository.
func (s *Service) commit(ctx context.Context, r *run, sc tdd.ScenarioRef, files []string, kind string) (string, error) {
	if !r.o.Commit || s.d.VCS == nil || len(files) == 0 {
		return "", nil
	}
	// The decisions and questions taken for the scenario, and its review
	// and verification records, travel with it.
	for _, doc := range []string{r.lay.Decisions(r.o.SpecPath), r.lay.Questions(r.o.SpecPath),
		r.lay.ReviewRecord(r.o.SpecPath, sc.Marker), r.lay.VerifyRecord(r.o.SpecPath, sc.Marker)} {
		if rel := r.lay.Rel(doc); s.d.Files.Exists(doc) && !slices.Contains(files, rel) {
			files = append(files, rel)
		}
	}
	how := "verified by SpecForge (RED → GREEN → REFACTOR)"
	if kind == "test" {
		how = "already satisfied: the test passed before any change, as the developer confirmed"
	}
	msg := fmt.Sprintf("%s(%s): %s\n\nScenario %d of %s, %s.", kind, sc.Marker, sc.Title, sc.Index, r.lay.Rel(r.o.SpecPath), how)
	sha, err := s.d.VCS.Commit(ctx, r.o.Root, msg, files)
	if errors.Is(err, ports.ErrNotARepository) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("committing scenario %d: %w", sc.Index, err)
	}
	return sha, nil
}
