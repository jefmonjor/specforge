package specs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jefmonjor/specforge/v6/internal/app/clarify"
	"github.com/jefmonjor/specforge/v6/internal/domain/spec"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// Clarify asks every open question of the specification, one at a time,
// and writes each decision in place of its question, so the specification
// itself says what was decided. Answers are also recorded in the decisions
// log. Without a terminal the questions go to questions.md and Clarify
// returns the pending error after writing the decisions it already has.
func (s Service) Clarify(ctx context.Context, e Entry, asker *clarify.Asker, by string) (int, error) {
	data, err := s.Files.ReadFile(e.Path)
	if err != nil {
		return 0, err
	}
	content := string(data)
	origin := clarify.Origin{Phase: "CLARIFY", DecisionsFile: s.Layout.Decisions(e.Path), QuestionsFile: s.Layout.Questions(e.Path)}
	resolved := 0
	var pending error
	for _, q := range spec.OpenQuestions(content) {
		answer, err := asker.Ask(ctx, origin, ports.Question{Text: q})
		if err != nil {
			var p *clarify.PendingQuestionError
			if errors.As(err, &p) {
				pending = err // keep asking: every question lands in questions.md
				continue
			}
			return resolved, s.write(e, content, resolved, err)
		}
		note := s.Now().Format(time.DateOnly)
		if by != "" {
			note += ", " + by
		}
		var ok bool
		if content, ok = spec.ResolveQuestion(content, q, answer, note); !ok {
			return resolved, fmt.Errorf("could not find the question %q in %s", q, e.Rel)
		}
		resolved++
	}
	return resolved, s.write(e, content, resolved, pending)
}

func (s Service) write(e Entry, content string, resolved int, err error) error {
	if resolved == 0 {
		return err
	}
	if werr := s.Files.WriteFile(e.Path, []byte(content)); werr != nil {
		return werr
	}
	return err
}
