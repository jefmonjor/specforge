// Package clarify turns an agent's question into a developer decision.
//
// When someone is at the terminal the question is asked and the answer is
// appended to the specification's decisions log, which every later prompt
// includes, so the agent is never asked the same thing twice. Without a
// terminal (CI) the question is written to questions.md and the run stops
// with a PendingQuestionError instead of letting the agent guess.
package clarify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"specforge/internal/ports"
)

// PendingQuestionError reports a question that could not be asked.
type PendingQuestionError struct {
	Question string
	File     string
}

func (e *PendingQuestionError) Error() string {
	return fmt.Sprintf("a question needs the developer's answer (saved to %s): %s", e.File, e.Question)
}

// Origin says where a question comes from.
type Origin struct {
	Phase    string
	Scenario int
	Marker   string
	// DecisionsFile and QuestionsFile are absolute paths.
	DecisionsFile string
	QuestionsFile string
}

// Asker asks the developer and records the decision.
type Asker struct {
	Prompter ports.Prompter
	Files    ports.Files
	Now      func() time.Time
	Lang     string
}

// Ask asks q and returns the answer once it is recorded.
func (a *Asker) Ask(ctx context.Context, o Origin, q ports.Question) (string, error) {
	answer, err := a.Prompter.Ask(ctx, q)
	if errors.Is(err, ports.ErrNonInteractive) {
		if werr := a.Files.AppendFile(o.QuestionsFile, []byte(a.entry(o, q, ""))); werr != nil {
			return "", errors.Join(err, werr)
		}
		return "", &PendingQuestionError{Question: q.Text, File: o.QuestionsFile}
	}
	if err != nil {
		return "", err
	}
	answer = strings.TrimSpace(answer)
	if err := a.Files.AppendFile(o.DecisionsFile, []byte(a.entry(o, q, answer))); err != nil {
		return "", fmt.Errorf("recording the decision: %w", err)
	}
	return answer, nil
}

// Decisions returns the decisions log of a specification, or "".
func (a *Asker) Decisions(o Origin) string {
	data, err := a.Files.ReadFile(o.DecisionsFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

type labels struct{ scenario, question, answer, pending, options string }

var catalog = map[string]labels{
	"es": {"escenario", "Pregunta", "Respuesta", "_pendiente de respuesta_", "Opciones"},
	"en": {"scenario", "Question", "Answer", "_awaiting an answer_", "Options"},
}

func (a *Asker) entry(o Origin, q ports.Question, answer string) string {
	l, ok := catalog[a.Lang]
	if !ok {
		l = catalog["en"]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "### %s · %s", a.Now().Format("2006-01-02 15:04"), o.Phase)
	if o.Scenario > 0 {
		fmt.Fprintf(&b, " · %s %d", l.scenario, o.Scenario)
		if o.Marker != "" {
			fmt.Fprintf(&b, " (`%s`)", o.Marker)
		}
	}
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "- **%s:** %s\n", l.question, oneLine(q.Text))
	if q.Context != "" {
		fmt.Fprintf(&b, "  - %s\n", oneLine(q.Context))
	}
	if len(q.Options) > 0 {
		fmt.Fprintf(&b, "  - %s: %s\n", l.options, strings.Join(q.Options, " · "))
	}
	if answer == "" {
		fmt.Fprintf(&b, "- **%s:** %s\n\n", l.answer, l.pending)
	} else {
		fmt.Fprintf(&b, "- **%s:** %s\n\n", l.answer, oneLine(answer))
	}
	return b.String()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
