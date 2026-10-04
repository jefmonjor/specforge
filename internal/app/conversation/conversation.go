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

	"specforge/internal/app/clarify"
	"specforge/internal/app/protocol"
	"specforge/internal/ports"
)

// ErrTooManyQuestions: the agent keeps asking within one turn.
var ErrTooManyQuestions = errors.New("the agent asked too many questions in a single step; the specification probably needs clarification")

// DefaultMaxQuestions bounds the questions of one turn.
const DefaultMaxQuestions = 5

// Turn tells the prompt renderer what happened so far in this turn.
type Turn struct {
	// Retry is true after an answer without the closing JSON.
	Retry bool
	// Question and Answer are set after the developer answered.
	Question, Answer string
}

// Hooks let the caller show progress and persist answers. All optional.
type Hooks struct {
	Working  func()
	Retried  func()
	Answered func(question, answer string) error
}

// Talk runs the turn. render builds the prompt for each exchange; req
// carries everything but the prompt. A blocked agent is returned as a
// response with Status Blocked and a nil error; a question nobody can
// answer now returns the response with a *clarify.PendingQuestionError.
func Talk(ctx context.Context, agent ports.Agent, asker *clarify.Asker, origin clarify.Origin, maxQuestions int,
	req ports.AgentRequest, render func(Turn) (string, error), h Hooks) (protocol.Response, error) {
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
		if err != nil {
			if turn.Retry {
				return protocol.Response{}, err
			}
			turn.Retry = true
			if h.Retried != nil {
				h.Retried()
			}
			continue
		}
		turn.Retry = false

		switch resp.Status {
		case protocol.Done, protocol.Blocked:
			return resp, nil
		case protocol.NeedsClarification:
			questions++
			if questions > maxQuestions {
				return resp, ErrTooManyQuestions
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
