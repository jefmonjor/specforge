// Package interview completes a specification in a conversation that
// SpecForge owns. Each turn is one headless agent call: the agent writes
// the developer's last answer into the specification and returns the next
// single question with what is still unknown, or reports that nothing is.
// SpecForge asks the developer, records every question and answer in
// interview.jsonl and the decisions log, checks that only the
// specification changed, and finishes only when the lint finds no
// placeholder or missing structure left.
package interview

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jefmonjor/specforge/v6/internal/app/clarify"
	"github.com/jefmonjor/specforge/v6/internal/app/conversation"
	"github.com/jefmonjor/specforge/v6/internal/app/layout"
	"github.com/jefmonjor/specforge/v6/internal/app/prompts"
	"github.com/jefmonjor/specforge/v6/internal/app/protocol"
	"github.com/jefmonjor/specforge/v6/internal/domain/spec"
	"github.com/jefmonjor/specforge/v6/internal/domain/tdd"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// DefaultMaxTurns bounds one interview run.
const DefaultMaxTurns = 25

// maxRounds bounds how often the agent is sent back after claiming it was
// done while the lint still finds placeholders.
const maxRounds = 3

// ScopeError reports files the agent changed besides the specification.
type ScopeError struct{ Files []string }

func (e *ScopeError) Error() string {
	return "the interview may only write the specification, but these files changed too: " + strings.Join(e.Files, ", ")
}

// IncompleteError reports a specification the agent called complete that
// still has blocking lint issues.
type IncompleteError struct{ Issues []spec.Issue }

func (e *IncompleteError) Error() string {
	return fmt.Sprintf("the interview ended with %d issue(s) left in the specification", len(e.Issues))
}

// Event is one line of interview.jsonl (or change.jsonl).
type Event struct {
	At       time.Time `json:"at"`
	Kind     string    `json:"kind"` // "question", "answer"; "request" and "done" bound a change
	Question string    `json:"question"`
	Why      string    `json:"why,omitempty"`
	Section  string    `json:"section,omitempty"`
	Options  []string  `json:"options,omitempty"`
	Unknowns []string  `json:"unknowns,omitempty"`
	Answer   string    `json:"answer,omitempty"`
}

// Events reports progress.
type Events interface {
	Working()
	Asked(section string, unknowns int)
	Answered()
	Rejected(reason string)
}

// Deps are the collaborators.
type Deps struct {
	Agent     ports.Agent
	Workspace ports.Workspace
	Files     ports.Files
	Asker     *clarify.Asker
	Events    Events
	Log       *slog.Logger
	Now       func() time.Time
}

// Options select the specification.
type Options struct {
	Root, SpecPath, Language, Model string
	AgentTimeout                    time.Duration
	MaxTurns                        int
	// Request turns the interview into a change request (spec change):
	// the agent applies it to the specification, asking what it leaves
	// open. Its conversation is kept in change.jsonl.
	Request string
}

// Result is how the interview ended.
type Result struct {
	Questions int
	// Changed is false when a change request left the specification as it
	// was.
	Changed bool
	// Open are the questions recorded as [NEEDS CLARIFICATION].
	Open []string
	// Advice are the non-blocking lint issues.
	Advice []spec.Issue
}

// Run interviews the developer until the specification is complete. A
// question nobody can answer now stops it with a *clarify.PendingQuestionError;
// running it again picks the answer up (from questions.md or the terminal)
// and continues.
func Run(ctx context.Context, d Deps, o Options) (Result, error) {
	if o.MaxTurns <= 0 {
		o.MaxTurns = DefaultMaxTurns
	}
	s := newSession(d, o)
	before, err := d.Workspace.Snapshot(ctx, o.Root)
	if err != nil {
		return Result{}, err
	}
	original, _ := d.Files.ReadFile(o.SpecPath)
	if err := s.begin(); err != nil {
		return Result{}, err
	}
	resumed, err := s.resume(ctx)
	if err != nil {
		return s.res, err
	}
	for round := 1; ; round++ {
		resp, err := conversation.Talk(ctx, d.Agent, d.Asker, s.origin, conversation.Rules{MaxQuestions: o.MaxTurns},
			ports.AgentRequest{Dir: o.Root, Model: o.Model, Timeout: o.AgentTimeout}, s.render(resumed), s.hooks())
		if err != nil {
			return s.res, err
		}
		if resp.Status == protocol.Blocked {
			return s.res, &tdd.AgentBlockedError{Phase: "INTERVIEW", Reason: resp.Reason, SuggestedAction: resp.SuggestedAction}
		}
		if err := checkScope(ctx, d, o, before, s.specRel, s.lay.SpecDir(o.SpecPath)); err != nil {
			return s.res, err
		}
		blocking, err := s.finish()
		if err != nil {
			return s.res, err
		}
		if len(blocking) == 0 {
			now, _ := d.Files.ReadFile(o.SpecPath)
			s.res.Changed = string(now) != string(original)
			return s.res, s.end()
		}
		if round >= maxRounds {
			return s.res, &IncompleteError{Issues: blocking}
		}
		s.sendBack(blocking)
		resumed = nil
	}
}

// session holds one run of the interview.
type session struct {
	d          Deps
	o          Options
	lay        layout.Layout
	specRel    string
	transcript string
	origin     clarify.Origin
	feedback   string
	res        Result
}

func newSession(d Deps, o Options) *session {
	lay := layout.Layout{Root: o.Root}
	s := &session{
		d: d, o: o, lay: lay,
		specRel:    lay.Rel(o.SpecPath),
		transcript: filepath.Join(lay.SpecDir(o.SpecPath), "interview.jsonl"),
		origin:     clarify.Origin{Phase: "INTERVIEW", DecisionsFile: lay.Decisions(o.SpecPath), QuestionsFile: lay.Questions(o.SpecPath)},
	}
	if o.Request != "" {
		s.transcript = filepath.Join(lay.SpecDir(o.SpecPath), "change.jsonl")
		s.origin.Phase = "CHANGE"
	}
	return s
}

// begin opens a change request in its transcript, unless this same request
// is still open (a question left pending): then it continues.
func (s *session) begin() error {
	if s.o.Request == "" {
		return nil
	}
	all := load(s.d.Files, s.transcript)
	for i := len(all) - 1; i >= 0; i-- {
		switch all[i].Kind {
		case "done":
			i = 0
		case "request":
			if all[i].Question == s.o.Request {
				return nil
			}
			i = 0
		}
	}
	return s.record(Event{Kind: "request", Question: s.o.Request})
}

// end closes a change request.
func (s *session) end() error {
	if s.o.Request == "" {
		return nil
	}
	return s.record(Event{Kind: "done", Question: s.o.Request})
}

// history is the conversation so far: since the request, for a change.
func (s *session) history() []Event {
	all := load(s.d.Files, s.transcript)
	if s.o.Request == "" {
		return all
	}
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Kind == "request" {
			return all[i+1:]
		}
	}
	return all
}

func (s *session) record(e Event) error {
	e.At = s.d.Now().UTC()
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return s.d.Files.AppendFile(s.transcript, append(line, '\n'))
}

// resume answers a question an earlier run left open and returns it as the
// turn the agent continues from, or nil.
func (s *session) resume(ctx context.Context) (*conversation.Turn, error) {
	q, ok := unanswered(s.history())
	if !ok {
		return nil, nil
	}
	answer, err := s.d.Asker.Ask(ctx, s.origin, ports.Question{Text: q.Question, Context: q.Why, Options: q.Options})
	if err != nil {
		return nil, err
	}
	if err := s.record(Event{Kind: "answer", Question: q.Question, Answer: answer}); err != nil {
		return nil, err
	}
	s.d.Events.Answered()
	return &conversation.Turn{Question: q.Question, Answer: answer}, nil
}

// render builds each turn's prompt from the specification as it is now.
func (s *session) render(resumed *conversation.Turn) func(conversation.Turn) (string, error) {
	first := true
	return func(t conversation.Turn) (string, error) {
		if first && resumed != nil && t.Answer == "" && !t.Retry {
			t = *resumed
		}
		first = false
		current, err := s.d.Files.ReadFile(s.o.SpecPath)
		if err != nil {
			return "", err
		}
		title := s.specRel
		if m, ok, _ := spec.ReadMeta(string(current)); ok && m.Title != "" {
			title = m.Title
		}
		data := prompts.Data{
			SpecTitle: title, SpecPath: s.specRel, Draft: string(current),
			Decisions: summary(s.history()), Feedback: s.feedback, Request: s.o.Request,
			AnsweredQuestion: t.Question, Answer: t.Answer,
		}
		if t.Retry {
			data.Feedback = strings.TrimSpace(s.feedback + "\nYour last answer did not end with the JSON object. Answer again and end with it.")
		}
		if s.o.Request != "" {
			return prompts.RenderChangeTurn(s.o.Language, data)
		}
		return prompts.RenderInterviewTurn(s.o.Language, data)
	}
}

func (s *session) hooks() conversation.Hooks {
	return conversation.Hooks{
		Working: s.d.Events.Working,
		Asked: func(r protocol.Response) error {
			s.res.Questions++
			s.d.Events.Asked(r.Section, len(r.Unknowns))
			return s.record(Event{Kind: "question", Question: r.Question, Why: r.Context, Section: r.Section, Options: r.Options, Unknowns: r.Unknowns})
		},
		Answered: func(q, a string) error {
			s.d.Events.Answered()
			return s.record(Event{Kind: "answer", Question: q, Answer: a})
		},
	}
}

// finish lints the specification. Open questions are an honest outcome of
// an interview (clarify and approve deal with them); anything else blocking
// is unfinished work and is returned.
func (s *session) finish() ([]spec.Issue, error) {
	content, err := s.d.Files.ReadFile(s.o.SpecPath)
	if err != nil {
		return nil, err
	}
	issues := spec.Lint(string(content), spec.ParseOptions{Languages: []string{s.o.Language}})
	blocking := slices.DeleteFunc(spec.Blocking(issues), func(i spec.Issue) bool { return i.Rule == spec.RuleOpenQuestion })
	if len(blocking) > 0 {
		return blocking, nil
	}
	s.res.Open = spec.OpenQuestions(string(content))
	for _, i := range issues {
		if !i.Blocking {
			s.res.Advice = append(s.res.Advice, i)
		}
	}
	return nil, nil
}

func (s *session) sendBack(blocking []spec.Issue) {
	var lines []string
	for _, i := range blocking {
		lines = append(lines, "- "+i.String())
	}
	s.feedback = "You reported the work as finished, but the specification still has:\n" + strings.Join(lines, "\n") +
		"\nAsk the developer what is needed to resolve them."
	s.d.Events.Rejected(strings.Join(lines, "; "))
}

// checkScope allows the specification and its own directory (decisions,
// questions, transcript); anything else the agent changed is an error.
func checkScope(ctx context.Context, d Deps, o Options, before ports.Snapshot, specRel, specDir string) error {
	after, err := d.Workspace.Snapshot(ctx, o.Root)
	if err != nil {
		return err
	}
	dirRel := filepath.ToSlash(layout.Layout{Root: o.Root}.Rel(specDir)) + "/"
	outside := slices.DeleteFunc(before.Changed(after), func(p string) bool {
		return p == specRel || strings.HasPrefix(p, dirRel)
	})
	if len(outside) > 0 {
		return &ScopeError{Files: outside}
	}
	return nil
}

func load(files ports.Files, path string) []Event {
	data, err := files.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []Event
	for _, line := range strings.Split(string(data), "\n") {
		var e Event
		if strings.TrimSpace(line) != "" && json.Unmarshal([]byte(line), &e) == nil {
			out = append(out, e)
		}
	}
	return out
}

// unanswered returns the last question when no answer followed it.
func unanswered(history []Event) (Event, bool) {
	if n := len(history); n > 0 && history[n-1].Kind == "question" {
		return history[n-1], true
	}
	return Event{}, false
}

// summary renders the questions and answers so far for the prompt.
func summary(history []Event) string {
	var lines []string
	for _, e := range history {
		if e.Kind == "answer" {
			lines = append(lines, fmt.Sprintf("- %s → %s", e.Question, e.Answer))
		}
	}
	return strings.Join(lines, "\n")
}
