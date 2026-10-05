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
	"specforge/internal/domain/review"
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
	// ChangeTurn is one turn of a change request to a specification.
	ChangeTurn Name = "change_turn"
	// LegacyMap maps the business capabilities of a legacy system.
	LegacyMap Name = "legacy_map"
	// FromLegacy drafts a specification from the legacy code.
	FromLegacy Name = "from_legacy"
	// Review is one read-only review lens; Refute tries to refute its
	// inferential findings; Validate checks a correction of them.
	Review   Name = "review"
	Refute   Name = "refute"
	Validate Name = "validate"
	// Correct is the one correction of the findings that block.
	Correct Name = "correct"
	// Verify is the independent verifier, in a copy of the project.
	Verify Name = "verify"
)

// File is a file shown to the agent as context.
type File struct {
	Path    string
	Content string
}

// Data is everything a prompt template can reference.
type Data struct {
	SpecTitle string
	SpecPath  string
	Stack     string
	Index     int
	Total     int
	Scenario  string
	// Previous is the scenario as it was implemented, when the
	// specification changed it since: its test must be updated.
	Previous    string
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
	// Request is the change the developer asked for (spec change).
	Request string
	// AnsweredQuestion and Answer carry the developer's answer to the
	// question the agent asked in this turn.
	AnsweredQuestion string
	Answer           string
	Attempt          int
	MaxAttempts      int
	// Known are the tests that failed before the loop began: not the
	// agent's to fix.
	Known []string
	// Surfaces are the files the approved plan allows the agent to edit
	// (a trailing slash allows a whole directory).
	Surfaces []string
	// Findings to correct, and how many lines the correction may change.
	Findings []review.Finding
	Budget   int

	// Migration: the legacy repository (read-only), its inventory, the
	// capability map, the capability a specification is drafted for, the
	// target Java release and the imports the new code may not use.
	Legacy           string
	LegacySources    string
	Inventory        string
	Capabilities     string
	Capability       string
	DocPath          string
	JavaRelease      int
	ForbiddenImports []string
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

// ReviewData is what the review, refute and validate prompts reference.
type ReviewData struct {
	Lens   review.Lens
	Prefix string
	Stack  string

	SpecTitle  string
	Marker     string
	Scenario   string
	Invariants string
	Plan       string
	Diff       string
	Findings   []review.Finding
	Feedback   string
}

// VerifyData is what the verify prompt references.
type VerifyData struct {
	Stack, SpecTitle string
	// Spec is the specification's text; Required the IDs to answer for.
	Spec     string
	Required []string
	// BaseDir is the copy of the project before the change, if any.
	BaseDir  string
	Feedback string
}

// RenderVerify renders the verifier's prompt.
func RenderVerify(lang string, d VerifyData) (string, error) {
	return renderStandalone(lang, Verify, d)
}

// RenderReview renders a review, refute or validate prompt. They end with
// their own JSON contract, validated against a schema.
func RenderReview(lang string, name Name, d ReviewData) (string, error) {
	return renderStandalone(lang, name, d)
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

// RenderChangeTurn renders one turn of a change request.
func RenderChangeTurn(lang string, d Data) (string, error) {
	return renderStandalone(lang, ChangeTurn, d)
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
