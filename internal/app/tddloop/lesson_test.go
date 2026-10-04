package tddloop

import (
	"strings"
	"testing"

	"specforge/internal/domain/tdd"
)

func TestALessonAfterARejectionIsKeptAndShown(t *testing.T) {
	h := newHarness(t, specBody)
	withLesson := func(files map[string]string, lesson string) reply {
		return func(p *project, prompt string) string {
			out := writes(files)(p, prompt)
			return strings.Replace(out, `{"status":"done",`, `{"status":"done","lesson":"`+lesson+`",`, 1)
		}
	}
	h.agent.turns = []reply{
		writes(map[string]string{test1: testFor("SDD_0001_001")}),
		withLesson(map[string]string{test1: testFor("SDD_0001_001") + "// ok\n", "reset.go": "package m\n"}, "Add a compiling stub for every function a new test calls."),
		withLesson(map[string]string{"reset.go": "package m\n// v1\n"}, "Not kept: GREEN passed at once."),
		writes(map[string]string{test2: testFor("SDD_0001_002"), "expiry.go": "package m\n"}),
	}
	h.tests.outcomes = []tdd.Outcome{notCompiled(), red(1), green(), green(), red(1)}

	_, err := h.run()
	if err == nil || !strings.Contains(err.Error(), "unexpected agent call") {
		t.Fatalf("want the script to end at scenario 2 GREEN, got %v", err)
	}
	got := h.p.read("specs/LESSONS.md")
	if !strings.Contains(got, "- [go] Add a compiling stub for every function a new test calls.") || strings.Contains(got, "Not kept") {
		t.Fatalf("LESSONS.md:\n%s", got)
	}
	if !strings.Contains(h.agent.prompts[3], "## Lessons from earlier scenarios\n- Add a compiling stub") {
		t.Fatalf("scenario 2 RED prompt lacks the lesson:\n%s", h.agent.prompts[3])
	}
}
