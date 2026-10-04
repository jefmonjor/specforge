// Package scaffold starts a new project with the tooling SpecForge drives
// already in place: the test runner, the linter and, where the stack has
// them, the architecture, duplication, dead-code and mutation tools. It
// only creates files: a project that already has a build file is refused,
// and any other file that exists is kept as it is.
package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"specforge/assets"
	"specforge/internal/ports"
)

// Kinds are the scaffolds available.
var Kinds = []string{"java", "react", "python", "go"}

// buildFiles mark a project that already exists.
var buildFiles = []string{"pom.xml", "build.gradle", "build.gradle.kts", "package.json", "pyproject.toml", "setup.py", "go.mod"}

// ErrExistingProject reports a directory that already holds a project.
var ErrExistingProject = errors.New("this directory already holds a project")

// Action is what happened to one file.
type Action string

const (
	Created Action = "created"
	Kept    Action = "kept"
)

// Change records one file.
type Change struct {
	Path   string
	Action Action
}

// Data names the project in the templates.
type Data struct {
	Name    string // artifact, package or module name: "payroll-service"
	Title   string // "Payroll service"
	Group   string // Java group id: "com.example"
	Package string // Java package: "com.example.payrollservice"
	Module  string // Python module: "payroll_service"
}

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

// NewData derives every name from a project name.
func NewData(name string) (Data, error) {
	slug := strings.Trim(nonWord.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		return Data{}, fmt.Errorf("the project name %q has no letters or digits", name)
	}
	if slug[0] >= '0' && slug[0] <= '9' {
		slug = "app-" + slug
	}
	words := strings.Split(slug, "-")
	title := strings.ToUpper(words[0][:1]) + words[0][1:]
	if len(words) > 1 {
		title += " " + strings.Join(words[1:], " ")
	}
	return Data{
		Name: slug, Title: title, Group: "com.example",
		Package: "com.example." + strings.Join(words, ""),
		Module:  strings.Join(words, "_"),
	}, nil
}

// Create writes the scaffold of kind into root.
func Create(files ports.Files, root, kind string, d Data) ([]Change, error) {
	base := "scaffolds/" + kind
	if _, err := fs.Stat(assets.FS, base); err != nil {
		return nil, fmt.Errorf("unknown scaffold %q (available: %s)", kind, strings.Join(Kinds, ", "))
	}
	for _, f := range buildFiles {
		if files.Exists(filepath.Join(root, f)) {
			return nil, fmt.Errorf("%w (%s exists): run `specforge setup` without --new", ErrExistingProject, f)
		}
	}
	var sources []string
	err := fs.WalkDir(assets.FS, base, func(p string, e fs.DirEntry, err error) error {
		if err == nil && !e.IsDir() {
			sources = append(sources, p)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(sources)
	var changes []Change
	for _, src := range sources {
		rel := target(strings.TrimPrefix(src, base+"/"), d)
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if files.Exists(abs) {
			changes = append(changes, Change{Path: rel, Action: Kept})
			continue
		}
		content, err := render(src, d)
		if err != nil {
			return changes, err
		}
		if err := files.WriteFile(abs, content); err != nil {
			return changes, fmt.Errorf("%s: %w", rel, err)
		}
		changes = append(changes, Change{Path: rel, Action: Created})
	}
	return changes, nil
}

// target maps a template path to the project path: __PKG__ becomes the
// Java package directory, __MODULE__ the Python module, and a .tmpl suffix
// goes (go.mod is stored as go.mod.tmpl: Go does not embed a directory
// holding a go.mod, which marks another module).
func target(rel string, d Data) string {
	rel = strings.TrimSuffix(rel, ".tmpl")
	rel = strings.ReplaceAll(rel, "__PKG__", strings.ReplaceAll(d.Package, ".", "/"))
	rel = strings.ReplaceAll(rel, "__MODULE__", d.Module)
	return path.Clean(rel)
}

func render(src string, d Data) ([]byte, error) {
	raw, err := fs.ReadFile(assets.FS, src)
	if err != nil {
		return nil, err
	}
	// [[ ]] delimiters leave JSX braces and GitHub expressions alone.
	t, err := template.New(path.Base(src)).Delims("[[", "]]").Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("scaffold template %s: %w", src, err)
	}
	var b bytes.Buffer
	if err := t.Execute(&b, d); err != nil {
		return nil, fmt.Errorf("scaffold template %s: %w", src, err)
	}
	return b.Bytes(), nil
}
