package spec

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type wantStep struct {
	kind StepKind
	text string
}

func steps(s Scenario) []wantStep {
	var out []wantStep
	for _, st := range s.Steps {
		out = append(out, wantStep{st.Kind, st.Text})
	}
	return out
}

func mustParse(t *testing.T, md string, langs ...string) *Document {
	t.Helper()
	doc, err := Parse(md, ParseOptions{Languages: langs})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return doc
}

func TestParseSpanishKeepsExactStepText(t *testing.T) {
	// Regression: the previous parser sliced line[5:] for every keyword and
	// produced "o se retira 30" and "ces el saldo es 70".
	md := "```gherkin\n# language: es\nCaracterística: Retiradas\n" +
		"  Escenario: Retirada con saldo\n" +
		"    Dado un saldo de 100\n    Cuando se retira 30\n    Entonces el saldo es 70\n```\n"
	doc := mustParse(t, md)
	want := []wantStep{{Given, "un saldo de 100"}, {When, "se retira 30"}, {Then, "el saldo es 70"}}
	if got := steps(doc.Scenarios[0]); !reflect.DeepEqual(got, want) {
		t.Fatalf("steps = %v, want %v", got, want)
	}
	if doc.Language != "es" || doc.Scenarios[0].Steps[1].Keyword != "Cuando " {
		t.Fatalf("language/keyword not preserved: %q %q", doc.Language, doc.Scenarios[0].Steps[1].Keyword)
	}
}

func TestParseSpanishWithoutLanguageHeaderUsesPreferredDialect(t *testing.T) {
	md := "```gherkin\nCaracterística: X\n  Escenario: Y\n    Dado a\n    Cuando b\n    Entonces c\n```\n"
	doc := mustParse(t, md, "es")
	if len(doc.Scenarios) != 1 || doc.Scenarios[0].Title != "Y" {
		t.Fatalf("unexpected scenarios: %+v", doc.Scenarios)
	}
}

func TestParseResolvesAndButToPreviousKind(t *testing.T) {
	md := "```gherkin\nFeature: F\n  Scenario: S\n    Given a\n    And b\n    When c\n    Then d\n    But e\n```\n"
	got := steps(mustParse(t, md).Scenarios[0])
	want := []wantStep{{Given, "a"}, {Given, "b"}, {When, "c"}, {Then, "d"}, {Then, "e"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("steps = %v, want %v", got, want)
	}
}

func TestParseExpandsOutlinesAndPrependsBackground(t *testing.T) {
	md := "```gherkin\nFeature: Interest\n" +
		"  Background:\n    Given an open account\n" +
		"  Scenario Outline: Interest at <rate>\n" +
		"    When interest of <rate> is applied to <balance>\n" +
		"    Then the balance is <result>\n" +
		"    Examples:\n      | rate | balance | result |\n      | 2%   | 100     | 102    |\n      | 5%   | 100     | 105    |\n```\n"
	doc := mustParse(t, md)
	if len(doc.Scenarios) != 2 {
		t.Fatalf("an outline with two rows must yield two scenarios, got %d", len(doc.Scenarios))
	}
	s := doc.Scenarios[1]
	if s.Index != 2 || s.Title != "Interest at 5%" {
		t.Fatalf("unexpected second scenario: %d %q", s.Index, s.Title)
	}
	want := []wantStep{{Given, "an open account"}, {When, "interest of 5% is applied to 100"}, {Then, "the balance is 105"}}
	if got := steps(s); !reflect.DeepEqual(got, want) {
		t.Fatalf("steps = %v, want %v", got, want)
	}
	if strings.Contains(s.Source, "<rate>") {
		t.Fatalf("placeholders must be substituted in the prompt source:\n%s", s.Source)
	}
}

func TestParseOutlineWithoutPlaceholderInNameGetsUniqueTitles(t *testing.T) {
	md := "```gherkin\nFeature: F\n  Scenario Outline: Same\n    Given <x>\n    Examples:\n      | x |\n      | 1 |\n      | 2 |\n```\n"
	doc := mustParse(t, md)
	if doc.Scenarios[0].Title == doc.Scenarios[1].Title {
		t.Fatalf("titles must be unique: %q", doc.Scenarios[0].Title)
	}
}

func TestParseMarkdownHeadingsWithoutFence(t *testing.T) {
	// Regression: "### Scenario:" collapsed the whole spec into a single
	// invented "Requerimiento General" scenario.
	md := "# Password reset\n\nSome prose that is not Gherkin.\n\n" +
		"### Scenario: Request a link\n- Given a registered user\n- When she asks for a reset\n- Then she gets a link\n\n" +
		"### **Scenario:** Expired link\nGiven an expired link\nWhen she opens it\nThen she sees an error\n\n" +
		"## Out of scope\n- SMS reset\n"
	doc := mustParse(t, md)
	if len(doc.Scenarios) != 2 {
		t.Fatalf("want 2 scenarios, got %d: %+v", len(doc.Scenarios), doc.Scenarios)
	}
	if doc.Scenarios[1].Title != "Expired link" || doc.Title != "Password reset" {
		t.Fatalf("titles: %q / %q", doc.Scenarios[1].Title, doc.Title)
	}
	if got := steps(doc.Scenarios[0]); got[2] != (wantStep{Then, "she gets a link"}) {
		t.Fatalf("steps = %v", got)
	}
}

func TestParseKeepsTablesAndDocStrings(t *testing.T) {
	md := "```gherkin\nFeature: F\n  Scenario: S\n    Given users\n      | name | role |\n      | ana  | admin |\n" +
		"    When the payload is\n      \"\"\"\n      {\"a\": 1}\n      \"\"\"\n    Then ok\n```\n"
	s := mustParse(t, md).Scenarios[0]
	if !reflect.DeepEqual(s.Steps[0].Table, [][]string{{"name", "role"}, {"ana", "admin"}}) {
		t.Fatalf("table = %v", s.Steps[0].Table)
	}
	if s.Steps[1].DocString != `{"a": 1}` {
		t.Fatalf("docstring = %q", s.Steps[1].DocString)
	}
}

func TestParseMultipleBlocksNumbersScenariosContinuously(t *testing.T) {
	block := func(name string) string {
		return "```gherkin\nFeature: " + name + "\n  Scenario: " + name + "\n    Given x\n```\n"
	}
	doc := mustParse(t, block("A")+"text\n"+block("B"))
	if len(doc.Scenarios) != 2 || doc.Scenarios[1].Index != 2 || doc.Scenarios[1].Title != "B" {
		t.Fatalf("unexpected: %+v", doc.Scenarios)
	}
}

func TestParseWithoutScenariosIsAnError(t *testing.T) {
	for _, md := range []string{"# Title\n\nJust prose.\n", "```gherkin\nFeature: F\n```\n"} {
		if _, err := Parse(md, ParseOptions{}); !errors.Is(err, ErrNoScenarios) {
			t.Errorf("Parse(%q) error = %v, want ErrNoScenarios", md, err)
		}
	}
}

func TestParseInvalidGherkinReportsTheParserError(t *testing.T) {
	md := "```gherkin\nFeature: F\n  Scenario: S\n    Given x\n  this line is not gherkin\n```\n"
	_, err := Parse(md, ParseOptions{})
	if err == nil || !strings.Contains(err.Error(), "gherkin block 1") {
		t.Fatalf("want a located parser error, got %v", err)
	}
}

func TestParseOfficialTemplateExample(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "password-reset.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := mustParse(t, string(data), "es")
	var titles []string
	for _, s := range doc.Scenarios {
		titles = append(titles, s.Title)
	}
	want := []string{"Solicitud de enlace", "Enlace caducado", "Validez según canal (example 1)", "Validez según canal (example 2)"}
	if !reflect.DeepEqual(titles, want) {
		t.Fatalf("titles = %q, want %q", titles, want)
	}
}

func TestFingerprintChangesWithContent(t *testing.T) {
	a := mustParse(t, "```gherkin\nFeature: F\n  Scenario: S\n    Given x\n```\n").Scenarios[0]
	b := mustParse(t, "```gherkin\nFeature: F\n  Scenario: S\n    Given y\n```\n").Scenarios[0]
	if a.Fingerprint() == b.Fingerprint() {
		t.Fatal("different steps must produce different fingerprints")
	}
}

func TestIDFromPathAndMarker(t *testing.T) {
	if got := IDFromPath("specs/0042-password-reset.md"); got != "0042" {
		t.Errorf("IDFromPath = %q", got)
	}
	if got := IDFromPath("spec.md"); got != "" {
		t.Errorf("IDFromPath(spec.md) = %q", got)
	}
	if got := Marker("0042", 3); got != "SDD_0042_003" {
		t.Errorf("Marker = %q", got)
	}
	if got := Marker("", 12); got != "SDD_0000_012" {
		t.Errorf("Marker without id = %q", got)
	}
	if strings.HasPrefix(Marker("0001", 10), Marker("0001", 1)) {
		t.Error("a marker must never be a prefix of another")
	}
}

func FuzzParse(f *testing.F) {
	f.Add("```gherkin\nFeature: F\n  Scenario: S\n    Given x\n```\n")
	f.Add("### Scenario: X\nGiven a\nWhen b\nThen c\n")
	f.Add("# language: es\nEscenario: X\nDado a\n")
	f.Fuzz(func(t *testing.T, md string) {
		doc, err := Parse(md, ParseOptions{Languages: []string{"es"}})
		if err != nil {
			return
		}
		if len(doc.Scenarios) == 0 {
			t.Fatal("a successful parse must return at least one scenario")
		}
		for i, s := range doc.Scenarios {
			if s.Index != i+1 {
				t.Fatalf("scenario %d has index %d", i, s.Index)
			}
		}
	})
}
