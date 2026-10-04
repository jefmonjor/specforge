package conversation

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"specforge/internal/adapters/fsys"
	"specforge/internal/app/clarify"
	"specforge/internal/app/protocol"
	"specforge/internal/ports"
)

type agent struct {
	replies []string
	prompts []string
}

func (a *agent) Name() string { return "fake" }
func (a *agent) Run(_ context.Context, r ports.AgentRequest) (string, error) {
	a.prompts = append(a.prompts, r.Prompt)
	if len(a.replies) == 0 {
		return "", errors.New("no more replies")
	}
	out := a.replies[0]
	a.replies = a.replies[1:]
	return out, nil
}
func (a *agent) Interactive(context.Context, ports.AgentRequest) error { return nil }

type prompter struct{ answer string }

func (p prompter) Ask(context.Context, ports.Question) (string, error) {
	if p.answer == "" {
		return "", ports.ErrNonInteractive
	}
	return p.answer, nil
}

func setup(t *testing.T, answer string) (*clarify.Asker, clarify.Origin) {
	dir := t.TempDir()
	return &clarify.Asker{Prompter: prompter{answer}, Files: fsys.OS{}, Lang: "en", Now: time.Now},
		clarify.Origin{Phase: "PLAN", DecisionsFile: filepath.Join(dir, "d.md"), QuestionsFile: filepath.Join(dir, "q.md")}
}

const (
	done     = "ok\n```json\n{\"status\":\"done\",\"files_written\":[\"a.go\"]}\n```"
	question = "```json\n{\"status\":\"needs_clarification\",\"question\":\"Which DB?\",\"options\":[\"pg\",\"sqlite\"]}\n```"
	blocked  = "```json\n{\"status\":\"blocked\",\"reason\":\"no go\"}\n```"
)

func render(t Turn) (string, error) {
	switch {
	case t.Retry:
		return "retry", nil
	case t.Answer != "":
		return "answered " + t.Question + " = " + t.Answer, nil
	}
	return "first", nil
}

func TestRetryThenDone(t *testing.T) {
	a := &agent{replies: []string{"prose only", done}}
	asker, o := setup(t, "")
	retried := 0
	resp, err := Talk(context.Background(), a, asker, o, 0, ports.AgentRequest{}, render, Hooks{Retried: func() { retried++ }})
	if err != nil || resp.Status != protocol.Done || retried != 1 || a.prompts[1] != "retry" {
		t.Fatalf("resp=%+v err=%v retried=%d prompts=%v", resp, err, retried, a.prompts)
	}
}

func TestTwoAnswersWithoutContractFail(t *testing.T) {
	a := &agent{replies: []string{"prose", "more prose"}}
	asker, o := setup(t, "")
	if _, err := Talk(context.Background(), a, asker, o, 0, ports.AgentRequest{}, render, Hooks{}); !errors.Is(err, protocol.ErrNoContract) {
		t.Fatalf("want ErrNoContract, got %v", err)
	}
}

func TestQuestionAnsweredAndRecorded(t *testing.T) {
	a := &agent{replies: []string{question, done}}
	asker, o := setup(t, "2")
	var got string
	resp, err := Talk(context.Background(), a, asker, o, 0, ports.AgentRequest{}, render, Hooks{Answered: func(q, ans string) error { got = q + "=" + ans; return nil }})
	if err != nil || resp.Status != protocol.Done {
		t.Fatal(err)
	}
	if got != "Which DB?=2" || !strings.HasPrefix(a.prompts[1], "answered Which DB? = 2") {
		t.Fatalf("got %q prompts %v", got, a.prompts)
	}
}

func TestPendingQuestionAndLimits(t *testing.T) {
	asker, o := setup(t, "")
	_, err := Talk(context.Background(), &agent{replies: []string{question}}, asker, o, 0, ports.AgentRequest{}, render, Hooks{})
	var p *clarify.PendingQuestionError
	if !errors.As(err, &p) {
		t.Fatalf("want pending, got %v", err)
	}
	asker2, o2 := setup(t, "pg")
	_, err = Talk(context.Background(), &agent{replies: []string{question, question}}, asker2, o2, 1, ports.AgentRequest{}, render, Hooks{})
	if !errors.Is(err, ErrTooManyQuestions) {
		t.Fatalf("want ErrTooManyQuestions, got %v", err)
	}
	resp, err := Talk(context.Background(), &agent{replies: []string{blocked}}, asker2, o2, 0, ports.AgentRequest{}, render, Hooks{})
	if err != nil || resp.Status != protocol.Blocked || resp.Reason != "no go" {
		t.Fatalf("blocked: %+v %v", resp, err)
	}
}
