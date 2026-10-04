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

func TestAnswerWrittenInTheQuestionsFileIsUsed(t *testing.T) {
	dir := t.TempDir()
	o := origin(dir)
	q := ports.Question{Text: "Which   channel\nsends the link?", Options: []string{"email", "sms"}}

	// First run, nobody at the terminal: written once, even if asked twice.
	ci := asker(&fakePrompter{err: ports.ErrNonInteractive})
	for range 2 {
		if _, err := ci.Ask(context.Background(), o, q); err == nil {
			t.Fatal("want a pending question")
		}
	}
	data, _ := fsys.OS{}.ReadFile(o.QuestionsFile)
	if strings.Count(string(data), "Which channel sends the link?") != 1 {
		t.Fatalf("questions.md:\n%s", data)
	}

	// The developer answers in the file with an option number.
	answered := strings.Replace(string(data), "_pendiente de respuesta_", "2", 1)
	if err := (fsys.OS{}).WriteFile(o.QuestionsFile, []byte(answered)); err != nil {
		t.Fatal(err)
	}
	prompter := &fakePrompter{err: errors.New("the terminal must not be used")}
	got, err := asker(prompter).Ask(context.Background(), o, q)
	if err != nil || got != "sms" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
	if !strings.Contains(asker(prompter).Decisions(o), "**Respuesta:** sms") {
		t.Fatal("the written answer must be recorded as a decision")
	}
}

func TestEnglishQuestionsFileIsReadToo(t *testing.T) {
	dir := t.TempDir()
	o := origin(dir)
	q := ports.Question{Text: "Free text?"}
	en := &Asker{Prompter: &fakePrompter{err: ports.ErrNonInteractive}, Files: fsys.OS{}, Lang: "en", Now: time.Now}
	en.Ask(context.Background(), o, q)
	data, _ := fsys.OS{}.ReadFile(o.QuestionsFile)
	fsys.OS{}.WriteFile(o.QuestionsFile, []byte(strings.Replace(string(data), "_awaiting an answer_", "from the request", 1)))
	if got, err := asker(&fakePrompter{err: ports.ErrNonInteractive}).Ask(context.Background(), o, q); err != nil || got != "from the request" {
		t.Fatalf("Ask = %q, %v", got, err)
	}
}

func TestStrictQuestionIgnoresAnInvalidWrittenAnswer(t *testing.T) {
	dir := t.TempDir()
	o := origin(dir)
	q := ports.Question{Text: "Already implemented?", Options: []string{"Yes", "No"}, Strict: true}
	ci := asker(&fakePrompter{err: ports.ErrNonInteractive})
	ci.Ask(context.Background(), o, q)
	data, _ := fsys.OS{}.ReadFile(o.QuestionsFile)
	fsys.OS{}.WriteFile(o.QuestionsFile, []byte(strings.Replace(string(data), "_pendiente de respuesta_", "Accept", 1)))
	var pq *PendingQuestionError
	if _, err := ci.Ask(context.Background(), o, q); !errors.As(err, &pq) {
		t.Fatalf("an invalid answer must leave the question pending, got %v", err)
	}
	data, _ = fsys.OS{}.ReadFile(o.QuestionsFile)
	if strings.Count(string(data), "Already implemented?") != 1 {
		t.Fatalf("questions.md:\n%s", data)
	}
}
