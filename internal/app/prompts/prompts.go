// Package prompts renders the agent prompts from the embedded templates.
// Every prompt ends with the response contract of package protocol.
package prompts

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"specforge/assets"
)

// Name of a prompt template.
type Name string

const (
	Red       Name = "red"
	Green     Name = "green"
	Refactor  Name = "refactor"
	E2E       Name = "e2e"
	Interview Name = "interview"
	Plan      Name = "plan"
	// InterviewTurn is one turn of the interview SpecForge runs.
	InterviewTurn Name = "interview_turn"
)

// File is a file shown to the agent as context.
type File struct {
	Path    string
	Content string
}

// Data is everything a prompt template can reference.
type Data struct {
	SpecTitle   string
	SpecPath    string
	Stack       string
	Index       int
	Total       int
	Scenario    string
	Marker      string
	MarkerHint  string
	TestCommand string

	Glossary   string
	Invariants string
	// Plan is the approved technical plan, when there is one.
	Plan string
	// For the plan prompt: the specification, the project's files and
	// where the plan goes.
	Spec     string
	Tree     string
	PlanPath string
	Markers  []string
	// Draft is the current plan, revised instead of rewritten.
	Draft     string
	Decisions string
	Lessons   string
	TestFiles []File

	LastFailure  string
	SuiteFailure string
	GateReport   string
	Feedback     string
	// AnsweredQuestion and Answer carry the developer's answer to the
	// question the agent asked in this turn.
	AnsweredQuestion string
	Answer           string
	Attempt          int
	MaxAttempts      int
}

// E2EData is what the E2E prompt references.
type E2EData struct {
	App       string
	Index     int
	Title     string
	Scenario  string
	Thens     []E2EThen
	URL       string
	PageTitle string
	Elements  []E2EElement
	Text      string
	History   []string
	Decisions string
}

// E2EThen is a Then step and whether it was verified.
type E2EThen struct {
	N        int
	Text     string
	Verified bool
}

// E2EElement is an element of the page.
type E2EElement struct {
	Selector, Tag, Type, Text, Placeholder, AriaLabel string
}

// InterviewData is what the interview prompt references.
type InterviewData struct {
	ID, Title, SpecPath string
	// Context lists the other specifications, one per line.
	Context string
}

// RenderE2E renders the E2E prompt.
func RenderE2E(lang string, d E2EData) (string, error) { return renderStandalone(lang, E2E, d) }

// RenderInterview renders the prompt of an interactive interview.
func RenderInterview(lang string, d InterviewData) (string, error) {
	return renderStandalone(lang, Interview, d)
}

// RenderInterviewTurn renders one turn of the interview.
func RenderInterviewTurn(lang string, d Data) (string, error) {
	return renderStandalone(lang, InterviewTurn, d)
}

// renderStandalone renders a template that does not end with the response
// contract (the agent talks to a person, or answers with actions).
func renderStandalone(lang string, name Name, d any) (string, error) {
	if !supported(lang) {
		lang = "en"
	}
	t, err := template.ParseFS(assets.FS, "prompts/"+lang+"/"+string(name)+".md")
	if err != nil {
		return "", fmt.Errorf("parsing the %s prompt: %w", name, err)
	}
	var b bytes.Buffer
	if err := t.Execute(&b, d); err != nil {
		return "", fmt.Errorf("rendering the %s prompt: %w", name, err)
	}
	return b.String(), nil
}

// Languages with a full set of templates.
var Languages = []string{"es", "en"}

// Render executes template name in language lang ("es" or "en"; anything
// else falls back to English).
func Render(lang string, name Name, d Data) (string, error) {
	if !supported(lang) {
		lang = "en"
	}
	sub, err := fs.Sub(assets.FS, "prompts/"+lang)
	if err != nil {
		return "", err
	}
	t, err := template.New(string(name)).ParseFS(sub, "contract.md", "context.md", string(name)+".md")
	if err != nil {
		return "", fmt.Errorf("parsing prompt %s/%s: %w", lang, name, err)
	}
	var b bytes.Buffer
	if err := t.ExecuteTemplate(&b, string(name)+".md", d); err != nil {
		return "", fmt.Errorf("rendering prompt %s/%s: %w", lang, name, err)
	}
	return strings.TrimSpace(b.String()) + "\n", nil
}

func supported(lang string) bool {
	for _, l := range Languages {
		if l == lang {
			return true
		}
	}
	return false
}
