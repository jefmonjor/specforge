package spec

import (
	"slices"
	"strings"
	"testing"
)

func rules(issues []Issue) []Rule {
	var out []Rule
	for _, i := range issues {
		out = append(out, i.Rule)
	}
	return out
}

const cleanSpec = "# Reset\n\n" +
	"## 4. Invariants\n\n- **INV-01**: a link is used once.\n\n" +
	"## 6. Scenarios\n\n```gherkin\nFeature: Reset\n\n" +
	"  Scenario: Link reused (INV-01)\n    Given a used link\n    When it is opened\n    Then the error \"used\" is shown\n```\n"

func TestLintCleanSpecHasNoBlockingIssue(t *testing.T) {
	issues := Lint(cleanSpec, ParseOptions{})
	if b := Blocking(issues); len(b) != 0 {
		t.Fatalf("unexpected blocking issues: %v", b)
	}
	// Numbered sections are present, so the missing ones are advice.
	if !slices.Contains(rules(issues), RuleSection) {
		t.Fatalf("want section advice, got %v", issues)
	}
}

func TestLintPlaceholdersIgnoreComments(t *testing.T) {
	md := "# T\n<!-- TODO in a comment -->\n<!--\nTODO multi\n-->\nTODO: intent\nok <!-- TODO --> TODO tail\n" +
		"```gherkin\nFeature: F\n  Scenario: S\n    When x\n    Then y\n```\n"
	var lines []int
	for _, i := range Lint(md, ParseOptions{}) {
		if i.Rule == RulePlaceholder {
			lines = append(lines, i.Line)
		}
	}
	if !slices.Equal(lines, []int{6, 7}) {
		t.Fatalf("placeholder lines = %v, want [6 7]", lines)
	}
}

func TestLintScenarioStructure(t *testing.T) {
	md := "```gherkin\nFeature: F\n" +
		"  Scenario: no action\n    Given x\n    Then y\n" +
		"  Scenario: no outcome\n    When x\n" +
		"  Scenario: two actions\n    When a\n    When b\n    Then c\n```\n"
	got := rules(Lint(md, ParseOptions{}))
	for _, want := range []Rule{RuleNoWhen, RuleNoThen, RuleManyWhens} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %s in %v", want, got)
		}
	}
	for _, i := range Lint(md, ParseOptions{}) {
		if i.Rule == RuleManyWhens && i.Blocking {
			t.Error("many-whens must be advice, not blocking")
		}
	}
}

func TestLintNoScenariosAndOpenQuestions(t *testing.T) {
	got := Lint("# T\n\n- [NEEDS CLARIFICATION]: which channel?\n", ParseOptions{})
	if !slices.Contains(rules(got), RuleScenarios) || !slices.Contains(rules(got), RuleOpenQuestion) {
		t.Fatalf("got %v", got)
	}
}

func TestLintUnreferencedInvariant(t *testing.T) {
	md := "## 4. Invariants\n\n- **INV-01**: once.\n- **INV-02**: never negative.\n\n" +
		"```gherkin\nFeature: F\n  Scenario: S INV-01\n    When x\n    Then y\n```\n"
	var msgs []string
	for _, i := range Lint(md, ParseOptions{}) {
		if i.Rule == RuleInvariant {
			msgs = append(msgs, i.Message)
		}
	}
	if len(msgs) != 1 || msgs[0] != "INV-02 is not referenced by any scenario" {
		t.Fatalf("got %v", msgs)
	}
}

func TestLintPlan(t *testing.T) {
	plan := "| SDD_0001_001 | reset_test.go |\nTODO decide the port\n"
	got := LintPlan(plan, []string{"SDD_0001_001", "SDD_0001_002"})
	if len(got) != 2 || got[0].Rule != RulePlaceholder || got[1].Message != "no planned test for SDD_0001_002" {
		t.Fatalf("got %v", got)
	}
	if got := LintPlan("  ", nil); len(got) != 1 || !got[0].Blocking {
		t.Fatalf("empty plan: %v", got)
	}
}

func TestInvariantIDs(t *testing.T) {
	md := "## 4. Invariants\n\n- **INV-01**: once.\n- **INV-02**: never negative, unlike INV-01.\n- **INV-01**: duplicate.\n\n## 5. Other\n- INV-09 is not defined here\n"
	if got := InvariantIDs(md); strings.Join(got, ",") != "INV-01,INV-02" {
		t.Fatalf("InvariantIDs = %v", got)
	}
}
