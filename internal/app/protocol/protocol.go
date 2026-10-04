// Package protocol is the contract between SpecForge and a coding agent.
//
// Every prompt ends by asking the agent to close its answer with one JSON
// object stating what happened: it finished, it needs the developer to
// answer a question, or it is blocked. SpecForge acts on that status and
// never treats prose as success. This is how "ask, don't invent" becomes a
// mechanism instead of a sentence in a prompt.
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"specforge/internal/jsontext"
)

// Status is what the agent reports about its turn.
type Status string

const (
	// Done: the task is complete; FilesWritten lists what changed.
	Done Status = "done"
	// NeedsClarification: something required is not in the spec, plan, code
	// or decisions; Question must be answered by the developer.
	NeedsClarification Status = "needs_clarification"
	// Blocked: the agent cannot continue for a reason the developer must
	// fix.
	Blocked Status = "blocked"
)

// Response is the JSON object that closes every agent answer.
type Response struct {
	Status       Status   `json:"status"`
	FilesWritten []string `json:"files_written,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	// Lesson is the rule that would have avoided a rejected attempt, in
	// one sentence. SpecForge keeps it in specs/LESSONS.md.
	Lesson string `json:"lesson,omitempty"`

	Question string   `json:"question,omitempty"`
	Options  []string `json:"options,omitempty"`
	Context  string   `json:"context,omitempty"`
	// Section and Unknowns are used by the interview: the specification
	// section the question completes, and what is still unknown.
	Section  string   `json:"section,omitempty"`
	Unknowns []string `json:"unknowns,omitempty"`

	Reason          string `json:"reason,omitempty"`
	SuggestedAction string `json:"suggested_action,omitempty"`
}

// ErrNoContract reports an answer without a valid closing JSON object.
var ErrNoContract = errors.New("the agent's answer does not end with the required JSON status object")

// Parse finds the status object in an agent's answer. It prefers the last
// fenced JSON block and falls back to the last bare JSON object, so a
// status quoted earlier in the answer never wins over the final one.
func Parse(answer string) (Response, error) {
	for _, c := range jsontext.Candidates(answer) {
		var r Response
		if json.Unmarshal([]byte(c), &r) == nil && r.validate() == nil {
			return r, nil
		}
	}
	return Response{}, ErrNoContract
}

func (r *Response) validate() error {
	r.Status = Status(strings.ToLower(strings.TrimSpace(string(r.Status))))
	switch r.Status {
	case Done:
		for i, f := range r.FilesWritten {
			r.FilesWritten[i] = strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(f), "\\", "/"), "./")
		}
		return nil
	case NeedsClarification:
		if strings.TrimSpace(r.Question) == "" {
			return fmt.Errorf("status %q requires a question", r.Status)
		}
		return nil
	case Blocked:
		if strings.TrimSpace(r.Reason) == "" {
			return fmt.Errorf("status %q requires a reason", r.Status)
		}
		return nil
	default:
		return fmt.Errorf("unknown status %q", r.Status)
	}
}
