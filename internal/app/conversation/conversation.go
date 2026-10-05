// Package conversation runs one agent turn under the response contract:
// it sends the prompt, retries once when the closing JSON is missing,
// routes questions to the developer and returns when the agent reports
// done or blocked. Every use case that talks to an agent goes through it,
// so "ask, don't invent" behaves the same everywhere.
package conversation

import (
	"context"
	"errors"
	"fmt"

	"github.com/jefmonjor/specforge/v6/internal/app/clarify"
	"github.com/jefmonjor/specforge/v6/internal/app/protocol"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// ErrTooManyQuestions: the agent keeps asking within one turn.
var ErrTooManyQuestions = errors.New("the agent asked too many questions in a single step; the specification probably needs clarification")

// ErrNoOptions: the agent asked an open question where it had to derive
// the candidate answers itself. It is a broken contract.
var ErrNoOptions = fmt.Errorf("%w: a question without options where options are required", protocol.ErrNoContract)

// MinOptions is how many candidate answers a question must offer when
// options are required: one option is not a choice.
const MinOptions = 2

// DefaultMaxQuestions bounds the questions of one turn.
const DefaultMaxQuestions = 5

// Rules bound a turn.
type Rules struct {
	// MaxQuestions bounds the questions of the turn (DefaultMaxQuestions
	// when zero).
	MaxQuestions int
	// RequireOptions refuses a question without at least MinOptions
	// options: about files, commands or technical alternatives, the agent
	// derives the candidates and the developer picks. The interview, whose
	// questions are about the business, leaves it off.
	RequireOptions bool
}

// Turn tells the prompt renderer what happened so far in this turn.
type Turn struct {
	// Retry is true after an answer that broke the contract.
	Retry bool
	// MissingOptions is true when that answer was a question without the
	// options the rules require.
	MissingOptions bool
	// Question and Answer are set after the developer answered.
	Question, Answer string
}

// Hooks let the caller show progress and persist answers. All optional.
type Hooks struct {
	Working func()
	Retried func()
	// Asked sees every question before the developer does.
	Asked    func(resp protocol.Response) error
	Answered func(question, answer string) error
}

// Talk runs the turn. render builds the prompt for each exchange; req
// carries everything but the prompt. A blocked agent is returned as a
// response with Status Blocked and a nil error; a question nobody can
// answer now returns the response with a *clarify.PendingQuestionError.
func Talk(ctx context.Context, agent ports.Agent, asker *clarify.Asker, origin clarify.Origin, rules Rules,
	req ports.AgentRequest, render func(Turn) (string, error), h Hooks) (protocol.Response, error) {
	maxQuestions := rules.MaxQuestions
	if maxQuestions <= 0 {
		maxQuestions = DefaultMaxQuestions
	}
	var turn Turn
	questions := 0
	for {
		prompt, err := render(turn)
		if err != nil {
			return protocol.Response{}, err
		}
		if h.Working != nil {
			h.Working()
		}
		req.Prompt = prompt
		out, err := agent.Run(ctx, req)
		if err != nil {
			return protocol.Response{}, fmt.Errorf("agent %s: %w", agent.Name(), err)
		}

		resp, err := protocol.Parse(out)
		missing := err == nil && rules.RequireOptions && resp.Status == protocol.NeedsClarification && len(resp.Options) < MinOptions
		if err != nil || missing {
			if turn.Retry {
				if missing {
					return resp, ErrNoOptions
				}
				return protocol.Response{}, err
			}
			turn.Retry, turn.MissingOptions = true, missing
			if h.Retried != nil {
				h.Retried()
			}
			continue
		}
		turn.Retry, turn.MissingOptions = false, false

		switch resp.Status {
		case protocol.Done, protocol.Blocked:
			return resp, nil
		case protocol.NeedsClarification:
			questions++
			if questions > maxQuestions {
				return resp, ErrTooManyQuestions
			}
			if h.Asked != nil {
				if err := h.Asked(resp); err != nil {
					return resp, err
				}
			}
			answer, err := asker.Ask(ctx, origin, ports.Question{Text: resp.Question, Context: resp.Context, Options: resp.Options})
			if err != nil {
				return resp, err
			}
			if h.Answered != nil {
				if err := h.Answered(resp.Question, answer); err != nil {
					return resp, err
				}
			}
			turn.Question, turn.Answer = resp.Question, answer
		}
	}
}
