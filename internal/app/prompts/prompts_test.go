package prompts

import (
	"strings"
	"testing"
)

func TestEveryPromptRendersInEveryLanguageWithTheContract(t *testing.T) {
	d := Data{
		SpecTitle: "Password reset", SpecPath: "specs/0001-reset.md", Stack: "go (go)",
		Index: 1, Total: 2, Scenario: "Scenario: S\n  Given x\n", Marker: "SDD_0001_001",
		TestCommand: "go test -run SDD_0001_001 ./...", LastFailure: "want 70",
		Glossary: "- Balance", Decisions: "- 30 min from request", TestFiles: []File{{Path: "a_test.go", Content: "package a"}},
		Feedback: "no marker", Attempt: 2, MaxAttempts: 3, GateReport: "dup 4%",
	}
	for _, lang := range append(Languages, "fr") {
		for _, name := range []Name{Red, Green, Refactor} {
			out, err := Render(lang, name, d)
			if err != nil {
				t.Fatalf("%s/%s: %v", lang, name, err)
			}
			for _, must := range []string{`"status": "done"`, `"needs_clarification"`, `"blocked"`, "Password reset", "a_test.go", "30 min from request"} {
				if !strings.Contains(out, must) {
					t.Errorf("%s/%s lacks %q", lang, name, must)
				}
			}
			if name != Refactor && !strings.Contains(out, "SDD_0001_001") {
				t.Errorf("%s/%s lacks the marker", lang, name)
			}
			if strings.Contains(out, "<no value>") {
				t.Errorf("%s/%s has an unset field", lang, name)
			}
		}
	}
}

func TestOptionalSectionsDisappearWhenEmpty(t *testing.T) {
	out, err := Render("en", Red, Data{SpecTitle: "T", Marker: "SDD_0000_001"})
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"previous attempt", "Ubiquitous language", "Decisions already", "Existing test file"} {
		if strings.Contains(out, absent) {
			t.Errorf("empty section %q rendered", absent)
		}
	}
}
