package migrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"specforge/internal/adapters/fsys"
	"specforge/internal/adapters/process"
	"specforge/internal/adapters/workspace"
	"specforge/internal/app/clarify"
	"specforge/internal/app/docturn"
	"specforge/internal/ports"
)

// legacyFiles is a small Java 6 payroll application.
var legacyFiles = map[string]string{
	"pom.xml": "<project><build><plugins><plugin><configuration><source>1.6</source><target>1.6</target></configuration></plugin></plugins></build></project>\n",
	"src/main/java/com/acme/payroll/PayrollServlet.java": "package com.acme.payroll;\n\nimport javax.servlet.http.HttpServlet;\nimport java.util.Vector;\n\npublic class PayrollServlet extends HttpServlet {\n  // net = gross - 15% tax, rounded down\n  int net(int gross) { return gross - gross * 15 / 100; }\n}\n",
}

type agent struct {
	root    string
	writes  []map[string]string
	replies []string
	reqs    []ports.AgentRequest
}

func (a *agent) Name() string { return "fake" }
func (a *agent) Run(_ context.Context, r ports.AgentRequest) (string, error) {
	a.reqs = append(a.reqs, r)
	i := len(a.reqs) - 1
	if i >= len(a.replies) {
		return "", errors.New("unexpected call")
	}
	if i < len(a.writes) {
		for p, c := range a.writes[i] {
			if !filepath.IsAbs(p) {
				p = filepath.Join(a.root, p)
			}
			os.MkdirAll(filepath.Dir(p), 0o755)
			os.WriteFile(p, []byte(c), 0o644)
		}
	}
	return a.replies[i], nil
}
func (a *agent) Interactive(context.Context, ports.AgentRequest) error { return nil }

type events struct{ rejected []string }

func (e *events) Working()                {}
func (e *events) Rejected(r string)       { e.rejected = append(e.rejected, r) }
func (e *events) Answered(string, string) {}

type nopPrompter struct{}

func (nopPrompter) Ask(context.Context, ports.Question) (string, error) {
	return "", ports.ErrNonInteractive
}

const done = "```json\n{\"status\":\"done\",\"files_written\":[\"x\"]}\n```"

func setup(t *testing.T, a *agent) (Deps, Options, *events) {
	t.Helper()
	base := t.TempDir()
	root, old := filepath.Join(base, "new"), filepath.Join(base, "old")
	for rel, c := range legacyFiles {
		p := filepath.Join(old, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(c), 0o644)
	}
	os.MkdirAll(root, 0o755)
	a.root = root
	ev := &events{}
	return Deps{
		Agent: a, Workspace: workspace.New(process.NewRunner(nil)), Files: fsys.OS{},
		Asker:  &clarify.Asker{Prompter: nopPrompter{}, Files: fsys.OS{}, Lang: "en", Now: time.Now},
		Events: ev,
	}, Options{Root: root, Legacy: old, Language: "en", MaxAttempts: 2, JavaRelease: 21, ForbiddenImports: []string{"javax.servlet"}}, ev
}

func TestScanWritesTheInventory(t *testing.T) {
	d, o, _ := setup(t, &agent{})
	inv, path, err := Scan(d.Files, o)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if inv.JavaRelease != "1.6" || !strings.Contains(string(data), "javax.servlet") {
		t.Fatalf("inventory %+v\n%s", inv, data)
	}
	o.Legacy = filepath.Join(o.Root, "missing")
	if _, _, err := Scan(d.Files, o); !errors.Is(err, ErrNoLegacy) {
		t.Fatalf("err=%v", err)
	}
}

const capMap = "# Capability map\n\n## Pay an employee\n- **Rules**: net is gross minus 15% `src/main/java/com/acme/payroll/PayrollServlet.java:7-8`\n"

func TestMapVerifiesEveryCitationAndGivesTheLegacyReadOnly(t *testing.T) {
	a := &agent{
		writes:  []map[string]string{{CapabilitiesRel: "## Pay\n- `src/Invented.java:3`\n"}, {CapabilitiesRel: capMap}},
		replies: []string{done, done},
	}
	d, o, ev := setup(t, a)
	path, err := Map(context.Background(), d, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.rejected) != 1 || !strings.Contains(ev.rejected[0], "src/Invented.java:3") {
		t.Fatalf("rejected %v", ev.rejected)
	}
	if got := a.reqs[0].ReadDirs; len(got) != 1 || got[0] != o.Legacy {
		t.Fatalf("read dirs %v", got)
	}
	if !strings.Contains(a.reqs[0].Prompt, "javax.servlet") || !strings.Contains(a.reqs[1].Prompt, "src/Invented.java:3") {
		t.Fatalf("prompts:\n%s\n---\n%s", a.reqs[0].Prompt, a.reqs[1].Prompt)
	}
	if data, _ := os.ReadFile(path); string(data) != capMap {
		t.Fatalf("map %q", data)
	}
}

func TestMapRefusesAChangedLegacyRepository(t *testing.T) {
	a := &agent{replies: []string{done}}
	d, o, _ := setup(t, a)
	a.writes = []map[string]string{{CapabilitiesRel: capMap, filepath.Join(o.Legacy, "pom.xml"): "<project/>"}}
	_, err := Map(context.Background(), d, o)
	var scope *docturn.ScopeError
	if !errors.As(err, &scope) || !strings.HasSuffix(scope.Files[0], "/pom.xml") {
		t.Fatalf("err=%v", err)
	}
}

const legacySpec = `---
id: "0001"
title: "Pay an employee"
status: draft
---

# 0001 · Pay an employee

## 1. Intent

Pay employees their net salary.

## 4. Invariants

- **INV-01**: the tax withheld is 15% of the gross, rounded down.

## 6. Scenarios

` + "```gherkin" + `
Feature: Pay an employee

  Scenario: INV-01 net pay withholds 15%
    Given an employee with a gross of 1000
    When the employee is paid
    Then the net is 850
` + "```" + `

## 12. Open questions

- [NEEDS CLARIFICATION]: is rounding down on purpose? ` + "`src/main/java/com/acme/payroll/PayrollServlet.java:8`" + `

## 13. Legacy sources

- INV-01: ` + "`src/main/java/com/acme/payroll/PayrollServlet.java:7-8`" + `
`

func TestDraftAllowsOpenQuestionsButNotInventedSources(t *testing.T) {
	invented := strings.Replace(legacySpec, "PayrollServlet.java:7-8", "PayrollServlet.java:70", 1)
	a := &agent{replies: []string{done, done}}
	d, o, ev := setup(t, a)
	specPath := filepath.Join(o.Root, "specs", "0001-pay.md")
	a.writes = []map[string]string{{specPath: invented}, {specPath: legacySpec}}
	if err := Draft(context.Background(), d, o, specPath, "Pay an employee"); err != nil {
		t.Fatal(err)
	}
	if len(ev.rejected) != 1 || !strings.Contains(ev.rejected[0], "PayrollServlet.java:70") {
		t.Fatalf("rejected %v", ev.rejected)
	}
	if !strings.Contains(a.reqs[0].Prompt, "Pay an employee") || !strings.Contains(a.reqs[0].Prompt, "Java 21") {
		t.Fatalf("prompt:\n%s", a.reqs[0].Prompt)
	}
}

func TestCheckSpec(t *testing.T) {
	fsys := os.DirFS(t.TempDir())
	problems := CheckSpec(fsys, "# T\n\n## 1. Intent\nTODO\n", "en")
	joined := strings.Join(problems, "\n")
	for _, want := range []string{"placeholder", "no Gherkin", "Legacy sources"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %s", want, joined)
		}
	}
}

func TestReadDirsSkipsALegacyInsideTheProject(t *testing.T) {
	o := Options{Root: "/p", Legacy: "/p/legacy"}
	if o.ReadDirs() != nil {
		t.Fatal("legacy inside the project needs no extra directory")
	}
	o.Legacy = "/p-old"
	if got := o.ReadDirs(); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}
