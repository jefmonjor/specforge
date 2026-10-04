package spec

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Rule names a lint check. They are stable: tooling may filter on them.
type Rule string

const (
	RulePlaceholder  Rule = "placeholder"   // a TODO left from the template
	RuleOpenQuestion Rule = "open-question" // [NEEDS CLARIFICATION] still open
	RuleScenarios    Rule = "scenarios"     // no Gherkin, or Gherkin that does not parse
	RuleNoWhen       Rule = "no-when"       // a scenario without an action
	RuleNoThen       Rule = "no-then"       // a scenario without an outcome
	RuleManyWhens    Rule = "many-whens"    // more than one behaviour in a scenario
	RuleSection      Rule = "section"       // a template section is missing
	RuleInvariant    Rule = "invariant"     // an invariant no scenario mentions
)

// Issue is one lint finding. Blocking issues prevent approval; the others
// are advice shown to the developer.
type Issue struct {
	Rule     Rule
	Line     int // 1-based; 0 when the issue is not tied to a line
	Message  string
	Blocking bool
}

func (i Issue) String() string {
	if i.Line > 0 {
		return fmt.Sprintf("line %d: %s [%s]", i.Line, i.Message, i.Rule)
	}
	return fmt.Sprintf("%s [%s]", i.Message, i.Rule)
}

// Sections of the template, by number. A specification that follows the
// template has all of them; one that does not only gets advice.
const templateSections = 12

var (
	todoWord       = regexp.MustCompile(`\bTODO\b`)
	numberedHead   = regexp.MustCompile(`^#{2,3}\s+(\d{1,2})[.)]\s`)
	invariantID    = regexp.MustCompile(`\bINV-\d+\b`)
	invariantEntry = regexp.MustCompile(`^\s*[-*+]\s*\**(INV-\d+)\b`)
)

// Lint checks a specification. It never fails: a document that cannot be
// parsed yields a blocking issue.
func Lint(markdown string, opts ParseOptions) []Issue {
	var issues []Issue
	issues = append(issues, placeholders(markdown)...)
	for _, q := range OpenQuestions(markdown) {
		issues = append(issues, Issue{Rule: RuleOpenQuestion, Message: "open question: " + q, Blocking: true})
	}

	doc, err := Parse(markdown, opts)
	switch {
	case errors.Is(err, ErrNoScenarios):
		issues = append(issues, Issue{Rule: RuleScenarios, Message: "no Gherkin scenario found", Blocking: true})
	case err != nil:
		issues = append(issues, Issue{Rule: RuleScenarios, Message: err.Error(), Blocking: true})
	default:
		for _, sc := range doc.Scenarios {
			whens, thens := len(sc.StepsOf(When)), len(sc.StepsOf(Then))
			if whens == 0 {
				issues = append(issues, Issue{Rule: RuleNoWhen, Message: fmt.Sprintf("scenario %d %q has no When step", sc.Index, sc.Title), Blocking: true})
			}
			if thens == 0 {
				issues = append(issues, Issue{Rule: RuleNoThen, Message: fmt.Sprintf("scenario %d %q has no Then step", sc.Index, sc.Title), Blocking: true})
			}
			if whens > 1 {
				issues = append(issues, Issue{Rule: RuleManyWhens, Message: fmt.Sprintf("scenario %d %q has %d When steps: consider one behaviour per scenario", sc.Index, sc.Title, whens)})
			}
		}
	}

	issues = append(issues, sections(markdown)...)
	issues = append(issues, invariants(markdown)...)
	return issues
}

// Blocking filters the issues that prevent approval.
func Blocking(issues []Issue) []Issue {
	var out []Issue
	for _, i := range issues {
		if i.Blocking {
			out = append(out, i)
		}
	}
	return out
}

// placeholders reports every line with a TODO outside HTML comments.
func placeholders(markdown string) []Issue {
	var out []Issue
	inComment := false
	for n, line := range strings.Split(normalizeNewlines(markdown), "\n") {
		visible := line
		if inComment {
			end := strings.Index(visible, commentClose)
			if end < 0 {
				continue
			}
			visible = visible[end+len(commentClose):]
			inComment = false
		}
		for {
			start := strings.Index(visible, commentOpen)
			if start < 0 {
				break
			}
			end := strings.Index(visible[start:], commentClose)
			if end < 0 {
				visible = visible[:start]
				inComment = true
				break
			}
			visible = visible[:start] + visible[start+end+len(commentClose):]
		}
		if todoWord.MatchString(visible) {
			out = append(out, Issue{Rule: RulePlaceholder, Line: n + 1, Message: "template placeholder left: " + strings.TrimSpace(visible), Blocking: true})
		}
	}
	return out
}

// sections advises on missing numbered sections, but only for documents
// that follow the numbered template at all.
func sections(markdown string) []Issue {
	present := map[int]bool{}
	inFence := false
	for _, line := range strings.Split(normalizeNewlines(markdown), "\n") {
		if fenceOpen.MatchString(strings.TrimSpace(line)) {
			inFence = !inFence
			continue
		}
		if m := numberedHead.FindStringSubmatch(line); m != nil && !inFence {
			var n int
			fmt.Sscan(m[1], &n)
			present[n] = true
		}
	}
	if len(present) == 0 {
		return nil
	}
	var out []Issue
	for n := 1; n <= templateSections; n++ {
		if !present[n] {
			out = append(out, Issue{Rule: RuleSection, Message: fmt.Sprintf("section %d of the template is missing", n)})
		}
	}
	return out
}

// invariants advises on invariants that nothing else in the specification
// mentions: an invariant without a scenario is a rule nobody tests.
func invariants(markdown string) []Issue {
	section := Section(markdown, InvariantsTitle)
	if section == "" {
		return nil
	}
	defined := map[string]string{} // id → its defining line
	var order []string
	for _, line := range strings.Split(section, "\n") {
		if m := invariantEntry.FindStringSubmatch(line); m != nil {
			if _, dup := defined[m[1]]; !dup {
				order = append(order, m[1])
			}
			defined[m[1]] = strings.TrimSpace(line)
		}
	}
	mentioned := map[string]bool{}
	for _, line := range strings.Split(normalizeNewlines(markdown), "\n") {
		for _, id := range invariantID.FindAllString(line, -1) {
			if defined[id] != strings.TrimSpace(line) {
				mentioned[id] = true
			}
		}
	}
	var out []Issue
	for _, id := range order {
		if !mentioned[id] {
			out = append(out, Issue{Rule: RuleInvariant, Message: id + " is not referenced by any scenario"})
		}
	}
	return out
}

// RulePlanMarker reports a scenario the plan does not place.
const RulePlanMarker Rule = "plan-marker"

// LintPlan checks a technical plan against its specification: every
// scenario marker must appear (so each scenario has a planned test) and no
// placeholder may be left.
func LintPlan(plan string, markers []string) []Issue {
	issues := placeholders(plan)
	if strings.TrimSpace(plan) == "" {
		return append(issues, Issue{Rule: RulePlanMarker, Message: "the plan is empty", Blocking: true})
	}
	for _, m := range markers {
		if !strings.Contains(plan, m) {
			issues = append(issues, Issue{Rule: RulePlanMarker, Message: "no planned test for " + m, Blocking: true})
		}
	}
	return issues
}
