// Package stack describes the technology stacks SpecForge can drive and
// detects them from a project's files.
//
// Detection is pure: it reads through an fs.FS, so callers pass
// os.DirFS(root) and tests pass an fstest.MapFS.
package stack

import (
	"encoding/json"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

// Kind is the build system family.
type Kind string

const (
	Go     Kind = "go"
	Maven  Kind = "maven"
	Gradle Kind = "gradle"
	Node   Kind = "node"
	Python Kind = "python"
)

// Runner is the test runner SpecForge drives for a stack.
type Runner string

const (
	RunnerGo     Runner = "go"
	RunnerMaven  Runner = "maven"
	RunnerGradle Runner = "gradle"
	RunnerVitest Runner = "vitest"
	RunnerJest   Runner = "jest"
	RunnerNPM    Runner = "npm"
	RunnerPytest Runner = "pytest"
)

// Profile is a detected stack.
type Profile struct {
	Kind   Kind
	Runner Runner
	// Framework is a notable UI framework ("react", "angular"), if any.
	Framework string
	// StandardDoc is the baseline standards file for the stack.
	StandardDoc string
}

// Name is a short human label, e.g. "node (vitest, react)".
func (p Profile) Name() string {
	parts := []string{string(p.Runner)}
	if p.Framework != "" {
		parts = append(parts, p.Framework)
	}
	return string(p.Kind) + " (" + strings.Join(parts, ", ") + ")"
}

// IgnoredDirs are never walked: dependencies, build output and SpecForge's
// own working directories.
var IgnoredDirs = map[string]bool{
	".git": true, "node_modules": true, "target": true, "build": true, "dist": true,
	"vendor": true, ".venv": true, "venv": true, "__pycache__": true, ".gradle": true,
	".idea": true, ".vscode": true, ".specify": true, ".specforge": true, ".sdd": true,
	".sdd-cache": true, "coverage": true, ".next": true, ".pytest_cache": true,
}

var (
	jsTestFile   = regexp.MustCompile(`(^|/)__tests__/|\.(test|spec)\.[cm]?[jt]sx?$`)
	pyTestFile   = regexp.MustCompile(`(^|/)(test_[^/]*|[^/]*_test|conftest)\.py$|(^|/)tests?/.*\.py$`)
	jvmTestFile  = regexp.MustCompile(`(^|/)src/test/.*\.(java|kt|groovy|scala)$`)
	goTestSuffix = "_test.go"
)

// IsTestFile reports whether rel (slash-separated, relative to the project
// root) is a test source for this stack. These are the files GREEN and
// REFACTOR must not touch.
func (p Profile) IsTestFile(rel string) bool {
	rel = path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	switch p.Kind {
	case Go:
		return strings.HasSuffix(rel, goTestSuffix)
	case Maven, Gradle:
		return jvmTestFile.MatchString(rel)
	case Node:
		return jsTestFile.MatchString(rel)
	case Python:
		return pyTestFile.MatchString(rel)
	}
	return false
}

// Detect returns the stacks found at the root of fsys, most specific first.
// More than one result means the project is ambiguous and the caller must
// ask which one to drive instead of guessing.
func Detect(fsys fs.FS) []Profile {
	var out []Profile
	if exists(fsys, "go.mod") {
		out = append(out, Profile{Kind: Go, Runner: RunnerGo, StandardDoc: "standards/go.md"})
	}
	if exists(fsys, "pom.xml") {
		out = append(out, Profile{Kind: Maven, Runner: RunnerMaven, StandardDoc: "standards/java.md"})
	}
	if exists(fsys, "build.gradle") || exists(fsys, "build.gradle.kts") {
		out = append(out, Profile{Kind: Gradle, Runner: RunnerGradle, StandardDoc: "standards/java.md"})
	}
	if data, err := fs.ReadFile(fsys, "package.json"); err == nil {
		out = append(out, nodeProfile(data))
	}
	if exists(fsys, "pyproject.toml") || exists(fsys, "requirements.txt") || exists(fsys, "setup.py") {
		out = append(out, Profile{Kind: Python, Runner: RunnerPytest, StandardDoc: "standards/python.md"})
	}
	return out
}

// ByName finds the candidate whose Kind matches name ("go", "node", ...).
func ByName(candidates []Profile, name string) (Profile, bool) {
	for _, p := range candidates {
		if string(p.Kind) == strings.ToLower(strings.TrimSpace(name)) {
			return p, true
		}
	}
	return Profile{}, false
}

type packageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func nodeProfile(data []byte) Profile {
	p := Profile{Kind: Node, Runner: RunnerNPM, StandardDoc: "standards/react.md"}
	var pkg packageJSON
	if json.Unmarshal(data, &pkg) != nil {
		return p
	}
	has := func(dep string) bool {
		_, a := pkg.Dependencies[dep]
		_, b := pkg.DevDependencies[dep]
		return a || b
	}
	testScript := pkg.Scripts["test"]
	switch {
	case has("vitest") || strings.Contains(testScript, "vitest"):
		p.Runner = RunnerVitest
	case has("jest") || strings.Contains(testScript, "jest"):
		p.Runner = RunnerJest
	}
	switch {
	case has("@angular/core"):
		p.Framework = "angular"
	case has("react"):
		p.Framework = "react"
	}
	return p
}

func exists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
