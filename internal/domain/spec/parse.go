package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	gherkin "github.com/cucumber/gherkin/go/v40"
	messages "github.com/cucumber/messages/go/v33"
)

// ErrNoScenarios reports a specification without any parseable scenario.
// SpecForge never invents a scenario from free text: a spec without
// acceptance criteria cannot drive a test-first loop.
var ErrNoScenarios = errors.New("the specification contains no Gherkin scenarios")

// StepKind is the role of a step once And/But are resolved.
type StepKind string

const (
	Given StepKind = "Given"
	When  StepKind = "When"
	Then  StepKind = "Then"
)

// Step is one executable step of a scenario.
type Step struct {
	Kind StepKind
	// Keyword is the keyword as written, in the spec's language ("Dado ").
	Keyword string
	Text    string
	// Table holds a data table argument, row by row.
	Table [][]string
	// DocString holds a doc string argument.
	DocString string
}

// Scenario is one concrete scenario. A Scenario Outline yields one Scenario
// per Examples row, and Background steps are already prepended.
type Scenario struct {
	// Index is 1-based and stable for a given document.
	Index int
	Title string
	Steps []Step
	// Source renders the scenario back to Gherkin, ready for a prompt.
	Source string
}

// Steps of the given kind, in order.
func (s Scenario) StepsOf(kind StepKind) []Step {
	var out []Step
	for _, st := range s.Steps {
		if st.Kind == kind {
			out = append(out, st)
		}
	}
	return out
}

// Fingerprint identifies the scenario's content. Two scenarios with the
// same title and steps have the same fingerprint, so a resumed loop can tell
// which scenarios an amendment changed.
func (s Scenario) Fingerprint() string {
	sum := sha256.Sum256([]byte(s.Source))
	return hex.EncodeToString(sum[:])
}

// Document is a parsed specification.
type Document struct {
	Title     string
	Language  string
	Scenarios []Scenario
}

// ParseOptions tunes Parse.
type ParseOptions struct {
	// Languages lists Gherkin dialects to try first, e.g. {"es"}. English and
	// Spanish are always tried afterwards. A "# language:" header inside a
	// Gherkin block always wins.
	Languages []string
}

// Parse extracts and parses the Gherkin of a Markdown specification.
//
// Fenced code blocks tagged gherkin, feature or cucumber are parsed as they
// are. A specification without such blocks is read line by line, keeping
// only lines that are Gherkin once Markdown decoration (headings, list
// markers, quotes, bold) is removed, so "### Scenario: X" works too.
func Parse(markdown string, opts ParseOptions) (*Document, error) {
	langs := candidateLanguages(opts.Languages)
	doc := &Document{Title: markdownTitle(markdown)}

	blocks := fencedGherkin(markdown)
	if len(blocks) == 0 {
		parsed, err := parseLoose(markdown, langs)
		if err != nil {
			return nil, err
		}
		doc.merge(parsed)
	} else {
		for i, block := range blocks {
			parsed, err := parseBest(block, langs)
			if err != nil {
				return nil, fmt.Errorf("gherkin block %d: %w", i+1, err)
			}
			doc.merge(parsed)
		}
	}

	if len(doc.Scenarios) == 0 {
		return nil, ErrNoScenarios
	}
	doc.uniquifyTitles()
	return doc, nil
}

// IDFromPath returns the four-digit identifier of a spec file named
// "NNNN-slug.md", or "" when the name does not follow that convention.
func IDFromPath(path string) string {
	m := idPrefix.FindStringSubmatch(filepath.Base(path))
	if m == nil {
		return ""
	}
	return m[1]
}

var idPrefix = regexp.MustCompile(`^(\d{4})-`)

// Marker is the token a test name must contain to be traced to a scenario,
// e.g. SDD_0001_003. It is a valid identifier in every supported language
// and fixed-width, so no marker is a prefix of another.
func Marker(specID string, scenarioIndex int) string {
	if specID == "" {
		specID = "0000"
	}
	return fmt.Sprintf("SDD_%s_%03d", specID, scenarioIndex)
}

type parsed struct {
	title     string
	language  string
	scenarios []Scenario
}

func (d *Document) merge(p parsed) {
	if d.Title == "" {
		d.Title = p.title
	}
	if d.Language == "" {
		d.Language = p.language
	}
	for _, s := range p.scenarios {
		s.Index = len(d.Scenarios) + 1
		d.Scenarios = append(d.Scenarios, s)
	}
}

// uniquifyTitles suffixes repeated titles (typical of Outline rows whose
// name has no placeholder) so a title identifies one scenario.
func (d *Document) uniquifyTitles() {
	count := map[string]int{}
	for _, s := range d.Scenarios {
		count[s.Title]++
	}
	seen := map[string]int{}
	for i, s := range d.Scenarios {
		if count[s.Title] > 1 {
			seen[s.Title]++
			d.Scenarios[i].Title = fmt.Sprintf("%s (example %d)", s.Title, seen[s.Title])
		}
	}
}

func candidateLanguages(preferred []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range append(append([]string{}, preferred...), "en", "es") {
		l = strings.ToLower(strings.TrimSpace(l))
		if l == "" || seen[l] || gherkin.DialectsBuiltin().GetDialect(l) == nil {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	return out
}

// parseBest parses src with each language and keeps the reading that yields
// the most scenarios. A "# language:" header overrides the language anyway.
func parseBest(src string, langs []string) (parsed, error) {
	var best parsed
	var firstErr error
	found := false
	for _, lang := range langs {
		p, err := parseGherkin(src, lang)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if !found || len(p.scenarios) > len(best.scenarios) {
			best, found = p, true
		}
	}
	if !found {
		return parsed{}, firstErr
	}
	return best, nil
}

func parseGherkin(src, lang string) (parsed, error) {
	newID := (&messages.Incrementing{}).NewId
	doc, err := gherkin.ParseGherkinDocumentForLanguage(strings.NewReader(src), lang, newID)
	if err != nil {
		return parsed{}, err
	}
	if doc.Feature == nil {
		return parsed{}, nil
	}

	steps := indexSteps(doc.Feature)
	pickles := gherkin.Pickles(*doc, "spec.md", newID)

	out := parsed{title: strings.TrimSpace(doc.Feature.Name), language: doc.Feature.Language}
	for _, p := range pickles {
		out.scenarios = append(out.scenarios, toScenario(p, steps))
	}
	return out, nil
}

func indexSteps(f *messages.Feature) map[string]*messages.Step {
	idx := map[string]*messages.Step{}
	add := func(steps []*messages.Step) {
		for _, s := range steps {
			idx[s.Id] = s
		}
	}
	walk := func(children []*messages.FeatureChild) {
		for _, c := range children {
			switch {
			case c.Background != nil:
				add(c.Background.Steps)
			case c.Scenario != nil:
				add(c.Scenario.Steps)
			case c.Rule != nil:
				for _, rc := range c.Rule.Children {
					if rc.Background != nil {
						add(rc.Background.Steps)
					}
					if rc.Scenario != nil {
						add(rc.Scenario.Steps)
					}
				}
			}
		}
	}
	walk(f.Children)
	return idx
}

func toScenario(p *messages.Pickle, astSteps map[string]*messages.Step) Scenario {
	sc := Scenario{Title: strings.TrimSpace(p.Name)}
	for _, ps := range p.Steps {
		st := Step{Kind: kindOf(ps.Type), Text: strings.TrimSpace(ps.Text)}
		if len(ps.AstNodeIds) > 0 {
			if ast, ok := astSteps[ps.AstNodeIds[0]]; ok {
				st.Keyword = ast.Keyword
			}
		}
		if st.Keyword == "" {
			st.Keyword = string(st.Kind) + " "
		}
		if ps.Argument != nil {
			if ps.Argument.DataTable != nil {
				for _, row := range ps.Argument.DataTable.Rows {
					var cells []string
					for _, c := range row.Cells {
						cells = append(cells, c.Value)
					}
					st.Table = append(st.Table, cells)
				}
			}
			if ps.Argument.DocString != nil {
				st.DocString = ps.Argument.DocString.Content
			}
		}
		sc.Steps = append(sc.Steps, st)
	}
	sc.Source = render(sc)
	return sc
}

func kindOf(t messages.PickleStepType) StepKind {
	switch t {
	case messages.PickleStepType_ACTION:
		return When
	case messages.PickleStepType_OUTCOME:
		return Then
	default:
		return Given
	}
}

func render(s Scenario) string {
	var b strings.Builder
	b.WriteString("Scenario: " + s.Title + "\n")
	for _, st := range s.Steps {
		b.WriteString("  " + st.Keyword + st.Text + "\n")
		for _, row := range st.Table {
			b.WriteString("    | " + strings.Join(row, " | ") + " |\n")
		}
		if st.DocString != "" {
			b.WriteString("    \"\"\"\n")
			for _, line := range strings.Split(st.DocString, "\n") {
				b.WriteString("    " + line + "\n")
			}
			b.WriteString("    \"\"\"\n")
		}
	}
	return b.String()
}

func markdownTitle(md string) string {
	for _, line := range strings.Split(md, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(t, "# "))
		}
	}
	return ""
}
