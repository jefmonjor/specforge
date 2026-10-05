package tddloop

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"specforge/internal/app/clarify"
	"specforge/internal/app/conversation"
	"specforge/internal/app/docturn"
	"specforge/internal/app/prompts"
	"specforge/internal/app/protocol"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// converse runs one agent turn under the response contract and turns a
// blocked agent into an error. Answers are recorded in the loop state.
func (s *Service) converse(ctx context.Context, r *run, name prompts.Name, data prompts.Data) (protocol.Response, error) {
	sc, _ := r.st.Scenario()
	origin := s.origin(r, sc)
	req := ports.AgentRequest{Dir: r.o.Root, Model: r.o.Models.For(modelPhase(r.st.Phase)), Env: r.o.AgentEnv, Timeout: r.o.AgentTimeout,
		ReadDirs: docturn.Outside(r.o.Root, r.o.Legacy)}
	touched, err := docturn.Watch(s.d.Workspace, req.ReadDirs)
	if err != nil {
		return protocol.Response{}, err
	}
	render := func(t conversation.Turn) (string, error) {
		d := data
		switch {
		case t.MissingOptions:
			d.Feedback = feedback(r.o.Language, RejectNoOptions)
		case t.Retry:
			d.Feedback = feedback(r.o.Language, RejectNoContract)
		}
		if t.Answer != "" {
			d.Decisions = s.d.Asker.Decisions(origin)
			d.AnsweredQuestion, d.Answer = t.Question, t.Answer
		}
		return prompts.Render(r.o.Language, name, d)
	}
	hooks := conversation.Hooks{
		Working: func() { s.d.Events.AgentWorking(r.st.Phase) },
		Retried: func() { s.d.Events.Rejected(RejectNoContract, "") },
		Answered: func(q, a string) error {
			r.st.Record("question", "answered", q+" → "+a, s.d.Now())
			s.d.Events.Answered(q, a)
			return s.save(r)
		},
	}
	rules := conversation.Rules{MaxQuestions: r.o.MaxClarifications, RequireOptions: true}
	resp, err := conversation.Talk(ctx, s.d.Agent, s.d.Asker, origin, rules, req, render, hooks)
	// The legacy code is the reference: a turn that changed it is refused
	// whatever else it did.
	if changed, werr := touched(); werr != nil || len(changed) > 0 {
		if werr != nil {
			return resp, werr
		}
		return resp, &docturn.ScopeError{Step: strings.ToLower(string(r.st.Phase)) + " (legacy code is read-only)", Files: changed}
	}
	var pending *clarify.PendingQuestionError
	switch {
	case errors.As(err, &pending):
		r.st.Record("question", "pending", resp.Question, s.d.Now())
		return resp, err
	case err != nil:
		return resp, fmt.Errorf("%s: %w", r.st.Phase, err)
	case resp.Status == protocol.Blocked:
		return resp, &tdd.AgentBlockedError{Phase: r.st.Phase, Reason: resp.Reason, SuggestedAction: resp.SuggestedAction}
	}
	return resp, s.raise(r, resp)
}

// modelPhase names the phase whose model an agent turn uses. The one
// correction of a review is a GREEN turn: it writes production code.
func modelPhase(p tdd.Phase) string {
	if p == tdd.PhaseReview {
		return "green"
	}
	return strings.ToLower(string(p))
}

// Phases of the questions SpecForge asks itself, as opposed to those the
// agent asks: they are recorded under their own label so the delivery can
// tell the developer's product decisions from process checks.
const (
	OriginReview = "REVIEW"
	OriginVerify = "VERIFY"
	OriginRisk   = "RISK"
)

// ProcessOrigins are the phases of SpecForge's own questions.
var ProcessOrigins = []string{OriginReview, OriginVerify, OriginRisk}

func (s *Service) origin(r *run, sc tdd.ScenarioRef) clarify.Origin {
	return s.originAs(r, sc, string(r.st.Phase))
}

func (s *Service) originAs(r *run, sc tdd.ScenarioRef, phase string) clarify.Origin {
	return clarify.Origin{
		Phase:         phase,
		Scenario:      sc.Index,
		Marker:        sc.Marker,
		DecisionsFile: r.lay.Decisions(r.o.SpecPath),
		QuestionsFile: r.lay.Questions(r.o.SpecPath),
	}
}

// falseClaims returns the files the agent says it wrote that did not change.
func falseClaims(claimed, changed []string) []string {
	set := map[string]bool{}
	for _, c := range changed {
		set[c] = true
	}
	var bad []string
	for _, c := range claimed {
		if c != "" && !set[c] {
			bad = append(bad, c)
		}
	}
	sort.Strings(bad)
	return bad
}

// changedKeys lists paths added, removed or modified between two hash maps.
func changedKeys(before, after map[string]string) []string {
	return ports.Snapshot(before).Changed(ports.Snapshot(after))
}

func joinPaths(p []string) string { return strings.Join(p, ", ") }
