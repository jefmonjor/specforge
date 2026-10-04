package tddloop

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"specforge/internal/app/clarify"
	"specforge/internal/app/prompts"
	"specforge/internal/app/protocol"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// converse sends a prompt and handles the protocol until the agent reports
// done: one retry for a missing contract, questions routed to the
// developer, and blocked turned into an error.
func (s *Service) converse(ctx context.Context, r *run, name prompts.Name, data prompts.Data) (protocol.Response, error) {
	retried := false
	questions := 0
	sc, _ := r.st.Scenario()
	for {
		prompt, err := prompts.Render(r.o.Language, name, data)
		if err != nil {
			return protocol.Response{}, err
		}
		s.d.Events.AgentWorking(r.st.Phase)
		out, err := s.d.Agent.Run(ctx, ports.AgentRequest{
			Prompt: prompt, Dir: r.o.Root, Model: r.o.Model, Env: r.o.AgentEnv, Timeout: r.o.AgentTimeout,
		})
		if err != nil {
			return protocol.Response{}, fmt.Errorf("agent %s during %s: %w", s.d.Agent.Name(), r.st.Phase, err)
		}

		resp, err := protocol.Parse(out)
		if err != nil {
			if retried {
				return protocol.Response{}, fmt.Errorf("%s: %w", r.st.Phase, err)
			}
			retried = true
			s.d.Events.Rejected(RejectNoContract, "")
			data.Feedback = feedback(r.o.Language, RejectNoContract)
			continue
		}

		switch resp.Status {
		case protocol.Done:
			return resp, nil
		case protocol.Blocked:
			return resp, &tdd.AgentBlockedError{Phase: r.st.Phase, Reason: resp.Reason, SuggestedAction: resp.SuggestedAction}
		case protocol.NeedsClarification:
			questions++
			if questions > r.o.MaxClarifications {
				return resp, ErrTooManyQuestions
			}
			origin := s.origin(r, sc)
			answer, err := s.d.Asker.Ask(ctx, origin, ports.Question{Text: resp.Question, Context: resp.Context, Options: resp.Options})
			if err != nil {
				var pending *clarify.PendingQuestionError
				if errors.As(err, &pending) {
					r.st.Record("question", "pending", resp.Question, s.d.Now())
				}
				return resp, err
			}
			r.st.Record("question", "answered", resp.Question+" → "+answer, s.d.Now())
			if err := s.save(r); err != nil {
				return resp, err
			}
			s.d.Events.Answered(resp.Question, answer)
			data.Decisions = s.d.Asker.Decisions(origin)
			data.AnsweredQuestion, data.Answer = resp.Question, answer
		}
	}
}

func (s *Service) origin(r *run, sc tdd.ScenarioRef) clarify.Origin {
	return clarify.Origin{
		Phase:         string(r.st.Phase),
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
