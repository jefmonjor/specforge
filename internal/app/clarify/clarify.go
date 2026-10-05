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
	"slices"
	"strconv"
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
	return "a question awaits the developer's answer: " + e.Question
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

// Ask returns the developer's answer to q and records it in the decisions
// log. An answer the developer already wrote in the questions file is used
// first; otherwise the question is asked at the terminal. Without a
// terminal the question is written to the questions file (once) and Ask
// returns a *PendingQuestionError.
func (a *Asker) Ask(ctx context.Context, o Origin, q ports.Question) (string, error) {
	written, pending := a.lookup(o, q)
	answer := written
	if answer == "" {
		var err error
		answer, err = a.Prompter.Ask(ctx, q)
		if errors.Is(err, ports.ErrNonInteractive) {
			if !pending {
				if werr := a.Files.AppendFile(o.QuestionsFile, []byte(a.entry(o, q, ""))); werr != nil {
					return "", errors.Join(err, werr)
				}
			}
			return "", &PendingQuestionError{Question: q.Text, File: o.QuestionsFile}
		}
		if err != nil {
			return "", err
		}
	}
	answer = strings.TrimSpace(answer)
	if err := a.Files.AppendFile(o.DecisionsFile, []byte(a.entry(o, q, answer))); err != nil {
		return "", fmt.Errorf("recording the decision: %w", err)
	}
	return answer, nil
}

// Record writes a decision nobody had to be asked for, such as the agent
// raising the risk of a change, so the log keeps every decision in one
// place.
func (a *Asker) Record(o Origin, q ports.Question, answer string) error {
	if err := a.Files.AppendFile(o.DecisionsFile, []byte(a.entry(o, q, strings.TrimSpace(answer)))); err != nil {
		return fmt.Errorf("recording the decision: %w", err)
	}
	return nil
}

// lookup searches the questions file for q. answer is what the developer
// wrote in place of the placeholder (an option number becomes the option);
// pending is true when q is there still unanswered.
func (a *Asker) lookup(o Origin, q ports.Question) (answer string, pending bool) {
	data, err := a.Files.ReadFile(o.QuestionsFile)
	if err != nil {
		return "", false
	}
	want := oneLine(q.Text)
	var question string
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "### ") {
			question = ""
			continue
		}
		if v, ok := field(line, func(l labels) string { return l.question }); ok {
			question = v
			continue
		}
		v, ok := field(line, func(l labels) string { return l.answer })
		if !ok || question != want {
			continue
		}
		if v == "" || isPlaceholder(v) {
			pending = true
			continue
		}
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= len(q.Options) {
			v = q.Options[n-1]
		}
		if q.Strict && !slices.ContainsFunc(q.Options, func(o string) bool { return strings.EqualFold(o, v) }) {
			pending = true // not one of the options: still unanswered
			continue
		}
		answer, pending = v, false
	}
	return answer, pending
}

// field reads "- **Label:** value" for the label of any language.
func field(line string, label func(labels) string) (string, bool) {
	for _, l := range catalog {
		prefix := "- **" + label(l) + ":**"
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), true
		}
	}
	return "", false
}

func isPlaceholder(v string) bool {
	for _, l := range catalog {
		if v == l.pending {
			return true
		}
	}
	return false
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

// Entry is one question of a decisions or questions log.
type Entry struct {
	Heading  string // "2026-10-04 15:30 · GREEN · scenario 3 (`SDD_0001_003`)"
	Phase    string // "GREEN"
	Question string
	Answer   string // "" while unanswered
}

// heading parses an entry's "### <time> · <phase> · …" line.
func heading(line string) (text, phase string, ok bool) {
	h, ok := strings.CutPrefix(line, "### ")
	if !ok {
		return "", "", false
	}
	text = strings.TrimSpace(h)
	if parts := strings.Split(text, " · "); len(parts) > 1 {
		phase = parts[1]
	}
	return text, phase, true
}

// Without drops from a log written by Asker the entries of phases.
func Without(log string, phases ...string) string {
	var kept strings.Builder
	drop := false
	for _, line := range strings.SplitAfter(log, "\n") {
		if _, phase, ok := heading(strings.TrimRight(line, "\r\n")); ok {
			drop = slices.Contains(phases, phase)
		}
		if !drop {
			kept.WriteString(line)
		}
	}
	return kept.String()
}

// Entries parses a decisions or questions log written by Asker, in any
// language.
func Entries(log string) []Entry {
	var out []Entry
	var cur *Entry
	for _, line := range strings.Split(strings.ReplaceAll(log, "\r\n", "\n"), "\n") {
		if h, phase, ok := heading(line); ok {
			out = append(out, Entry{Heading: h, Phase: phase})
			cur = &out[len(out)-1]
			continue
		}
		if cur == nil {
			continue
		}
		if v, ok := field(line, func(l labels) string { return l.question }); ok {
			cur.Question = v
		} else if v, ok := field(line, func(l labels) string { return l.answer }); ok && !isPlaceholder(v) {
			cur.Answer = v
		}
	}
	return out
}
