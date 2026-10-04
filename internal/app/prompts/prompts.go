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
	Red      Name = "red"
	Green    Name = "green"
	Refactor Name = "refactor"
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
	Decisions  string
	Lessons    string
	TestFiles  []File

	LastFailure  string
	SuiteFailure string
	GateReport   string
	Feedback     string
	Attempt      int
	MaxAttempts  int
}

// Languages with a full set of templates.
var Languages = []string{"es", "en"}

// Render executes template name in language lang ("es" or "en"; anything
// else falls back to English).
func Render(lang string, name Name, d Data) (string, error) {
	if !supported(lang) {
		lang = "en"
	}
	sub, err := fs.Sub(assets.PromptsFS, "prompts/"+lang)
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
