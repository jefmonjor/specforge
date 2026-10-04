package clarify

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"specforge/internal/adapters/fsys"
	"specforge/internal/ports"
)

type fakePrompter struct {
	answer string
	err    error
	got    ports.Question
}

func (f *fakePrompter) Ask(_ context.Context, q ports.Question) (string, error) {
	f.got = q
	return f.answer, f.err
}

func origin(dir string) Origin {
	return Origin{Phase: "GREEN", Scenario: 3, Marker: "SDD_0001_003",
		DecisionsFile: filepath.Join(dir, "decisions.md"), QuestionsFile: filepath.Join(dir, "questions.md")}
}

func asker(p ports.Prompter) *Asker {
	return &Asker{Prompter: p, Files: fsys.OS{}, Lang: "es",
		Now: func() time.Time { return time.Date(2026, 10, 4, 15, 30, 0, 0, time.UTC) }}
}

func TestAnswerIsRecordedAsADecision(t *testing.T) {
	dir := t.TempDir()
	p := &fakePrompter{answer: "  desde la petición \n"}
	a := asker(p)
	q := ports.Question{Text: "¿30 min desde la petición o desde la entrega?", Options: []string{"petición", "entrega"}}

	got, err := a.Ask(context.Background(), origin(dir), q)
	if err != nil || got != "desde la petición" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
	log := a.Decisions(origin(dir))
	for _, want := range []string{"2026-10-04 15:30 · GREEN · escenario 3 (`SDD_0001_003`)", "**Pregunta:** ¿30 min", "Opciones: petición · entrega", "**Respuesta:** desde la petición"} {
		if !strings.Contains(log, want) {
			t.Errorf("decisions log lacks %q:\n%s", want, log)
		}
	}
}

func TestWithoutTerminalTheQuestionIsSavedAndTheRunStops(t *testing.T) {
	dir := t.TempDir()
	a := asker(&fakePrompter{err: ports.ErrNonInteractive})
	_, err := a.Ask(context.Background(), origin(dir), ports.Question{Text: "¿Quién aprueba?"})
	var pq *PendingQuestionError
	if !errors.As(err, &pq) || pq.Question != "¿Quién aprueba?" {
		t.Fatalf("want PendingQuestionError, got %v", err)
	}
	data, _ := fsys.OS{}.ReadFile(origin(dir).QuestionsFile)
	if !strings.Contains(string(data), "_pendiente de respuesta_") {
		t.Fatalf("questions.md = %s", data)
	}
	if a.Decisions(origin(dir)) != "" {
		t.Fatal("an unanswered question is not a decision")
	}
}

func TestOtherPrompterErrorsPropagate(t *testing.T) {
	boom := errors.New("boom")
	_, err := asker(&fakePrompter{err: boom}).Ask(context.Background(), origin(t.TempDir()), ports.Question{Text: "x"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}
