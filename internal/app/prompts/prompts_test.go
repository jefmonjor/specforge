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

func TestMigrationPrompts(t *testing.T) {
	d := Data{Legacy: "/src/old", Inventory: "## Build\nmaven", Capability: "Pay an employee", SpecPath: "specs/0001-pay.md",
		DocPath: "docs/legacy/CAPABILITIES.md", JavaRelease: 21, ForbiddenImports: []string{"javax.servlet"}, MaxAttempts: 3}
	for _, lang := range Languages {
		for _, name := range []Name{LegacyMap, FromLegacy} {
			out, err := Render(lang, name, d)
			if err != nil {
				t.Fatalf("%s/%s: %v", lang, name, err)
			}
			if !strings.Contains(out, "/src/old") || strings.Contains(out, "<no value>") || strings.Count(out, "/src/old") > 3 {
				t.Errorf("%s/%s:\n%s", lang, name, out)
			}
		}
		// The loop's prompts carry the legacy code as a reference.
		out, err := Render(lang, Green, Data{SpecTitle: "Pay", Marker: "SDD_0001_001", Legacy: "/src/old", LegacySources: "- INV-01: `Pay.java:3`", JavaRelease: 21, ForbiddenImports: []string{"javax.servlet", "org.apache.log4j"}})
		if err != nil || !strings.Contains(out, "`javax.servlet`, `org.apache.log4j`") || !strings.Contains(out, "Java 21") || !strings.Contains(out, "`Pay.java:3`") {
			t.Errorf("%s green: %v\n%s", lang, err, out)
		}
	}
}
