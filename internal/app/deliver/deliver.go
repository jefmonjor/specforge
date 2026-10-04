// Package deliver builds the hand-over of a specification (gate R4):
// DELIVERY.md for people, trace.json for tools and PR_BODY.md for the pull
// request. Everything comes from artifacts SpecForge wrote along the way:
// the sealed specification and plan, the loop state, the decisions and
// questions logs, the lessons and the audit and E2E reports.
package deliver

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"specforge/internal/app/clarify"
	"specforge/internal/app/layout"
	"specforge/internal/app/tddloop"
	"specforge/internal/domain/delivery"
	"specforge/internal/domain/e2e"
	"specforge/internal/domain/lessons"
	"specforge/internal/domain/security"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/stack"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// Options select the specification.
type Options struct {
	Root, SpecPath, Language string
	// Profile tells test files apart; nil when the project has no stack yet.
	Profile *stack.Profile
	Now     time.Time
}

var outOfScope = regexp.MustCompile(`(?i)out of scope|fuera de alcance`)

// Build assembles the trace. The specification must be approved: a
// delivery of something nobody approved would describe nothing.
func Build(files ports.Files, o Options) (delivery.Trace, error) {
	lay := layout.Layout{Root: o.Root}
	data, err := files.ReadFile(o.SpecPath)
	if err != nil {
		return delivery.Trace{}, err
	}
	md := string(data)
	if err := spec.Verify(md); err != nil {
		return delivery.Trace{}, err
	}
	doc, err := spec.Parse(md, spec.ParseOptions{Languages: []string{o.Language}})
	if err != nil {
		return delivery.Trace{}, err
	}
	id := spec.IDFromPath(o.SpecPath)
	t := delivery.Trace{
		GeneratedAt: o.Now.UTC(),
		ID:          id,
		Title:       doc.Title,
		Spec:        approvalOf(lay.Rel(o.SpecPath), md),
		OutOfScope:  spec.Section(md, outOfScope),
	}
	if m, ok, _ := spec.ReadMeta(md); ok && m.Title != "" {
		t.Title = m.Title
	}
	if plan, err := files.ReadFile(lay.Plan(o.SpecPath)); err == nil {
		a := approvalOf(lay.Rel(lay.Plan(o.SpecPath)), string(plan))
		t.Plan = &a
	}

	st := readState(files, lay.State(o.SpecPath))
	for _, sc := range doc.Scenarios {
		t.Scenarios = append(t.Scenarios, scenario(files, lay, o, id, sc, st))
	}

	answered := map[string]bool{}
	if log, err := files.ReadFile(lay.Decisions(o.SpecPath)); err == nil {
		for _, e := range clarify.Entries(string(log)) {
			answered[e.Question] = e.Answer != ""
			// Reviews and verification checks are process, already shown
			// per scenario; the decisions are the product answers.
			if e.Answer != "" && e.Phase != tddloop.OriginReview && e.Phase != tddloop.OriginVerify {
				t.Decisions = append(t.Decisions, e.Question+" → "+e.Answer)
			}
		}
	}
	if log, err := files.ReadFile(lay.Questions(o.SpecPath)); err == nil {
		for _, e := range clarify.Entries(string(log)) {
			if e.Answer == "" && !answered[e.Question] && !slices.Contains(t.Pending, e.Question) {
				t.Pending = append(t.Pending, e.Question)
			}
		}
	}
	if o.Profile != nil {
		if text, err := files.ReadFile(lay.Lessons()); err == nil {
			for _, line := range strings.Split(lessons.For(string(text), string(o.Profile.Kind)), "\n") {
				if line = strings.TrimPrefix(line, "- "); line != "" {
					t.Lessons = append(t.Lessons, line)
				}
			}
		}
	}
	t.Checks = checks(files, o, lay)
	return t, nil
}

func approvalOf(path, content string) delivery.Approval {
	a := delivery.Approval{Path: path}
	if spec.Verify(content) != nil {
		return a // a draft or an edited document: not approved
	}
	if m, ok, _ := spec.ReadMeta(content); ok {
		a.ApprovedBy, a.ApprovedAt = m.ApprovedBy, m.ApprovedAt
	}
	if s, ok := spec.ReadSeal(content); ok {
		a.Seal = s.Algorithm + ":" + s.Hash
	}
	return a
}

func readState(files ports.Files, path string) *tdd.State {
	data, err := files.ReadFile(path)
	if err != nil {
		return nil
	}
	var st tdd.State
	if json.Unmarshal(data, &st) != nil {
		return nil
	}
	return &st
}

// scenario traces one scenario. The loop state is matched by content
// fingerprint, so progress recorded for an older version of a changed
// scenario never counts.
func scenario(files ports.Files, lay layout.Layout, o Options, id string, sc spec.Scenario, st *tdd.State) delivery.Scenario {
	out := delivery.Scenario{Index: sc.Index, Marker: spec.Marker(id, sc.Index), Title: sc.Title, Status: delivery.Pending}
	if st == nil {
		return out
	}
	var ref *tdd.ScenarioRef
	for i := range st.Scenarios {
		if st.Scenarios[i].Fingerprint == sc.Fingerprint() {
			ref = &st.Scenarios[i]
		}
	}
	if ref == nil {
		return out
	}
	switch {
	case ref.Satisfied:
		out.Status = delivery.Satisfied
	case ref.Done:
		out.Status = delivery.Done
	}
	out.Files, out.Commit = ref.Files, ref.Commit
	for _, f := range ref.Files {
		if o.Profile != nil && !o.Profile.IsTestFile(f) {
			continue
		}
		if names := testNames(files, lay.Abs(f), ref.Marker); len(names) > 0 {
			out.Tests = append(out.Tests, delivery.Test{File: f, Names: names})
		}
	}
	for _, c := range st.Checkpoints {
		if c.Scenario != ref.Index {
			continue
		}
		switch {
		case c.Status == "rejected":
			out.Rejections++
		case c.Step == "review" && c.Status == "change requested":
			out.ReviewNotes++
		case c.Step == "refactor" && c.Status == "accepted":
			out.Gates = c.Details
		}
	}
	return out
}

// testNames finds the test names that carry marker: a quoted title (JS)
// or an identifier (Go, Java, Python).
func testNames(files ports.Files, path, marker string) []string {
	data, err := files.ReadFile(path)
	if err != nil || marker == "" {
		return nil
	}
	quoted := regexp.MustCompile("[\"'`]([^\"'`\\n]*" + regexp.QuoteMeta(marker) + "[^\"'`\\n]*)[\"'`]")
	ident := regexp.MustCompile(`[A-Za-z0-9_]*` + regexp.QuoteMeta(marker) + `[A-Za-z0-9_]*`)
	var names []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(line, marker) || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "*") {
			continue // comments mention markers without naming a test
		}
		name := ""
		if m := quoted.FindStringSubmatch(line); m != nil {
			name = m[1]
		} else {
			name = ident.FindString(line)
		}
		if name != "" && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	return names
}

func checks(files ports.Files, o Options, lay layout.Layout) delivery.Checks {
	var c delivery.Checks
	if data, err := files.ReadFile(filepath.Join(o.Root, "docs", "security", "findings.json")); err == nil {
		var r security.Report
		if json.Unmarshal(data, &r) == nil {
			r.Recount()
			c.Security = &delivery.Security{Confirmed: r.TotalConfirmed, NeedsValidation: r.TotalNeedsValidation, Rejected: r.TotalRejected}
		}
	}
	e2ePath := filepath.Join(o.Root, "docs", "e2e", filepath.Base(lay.SpecDir(o.SpecPath)), "report.json")
	if data, err := files.ReadFile(e2ePath); err == nil {
		var r e2e.Report
		if json.Unmarshal(data, &r) == nil && len(r.Scenarios) > 0 {
			passed := 0
			for _, s := range r.Scenarios {
				if s.Status == e2e.Passed {
					passed++
				}
			}
			c.E2E = &delivery.E2E{Scenarios: len(r.Scenarios), Passed: passed, PassRate: r.PassRate()}
		}
	}
	return c
}

// PRTemplates are where GitHub looks for a pull request template.
var PRTemplates = []string{
	".github/pull_request_template.md", ".github/PULL_REQUEST_TEMPLATE.md",
	"pull_request_template.md", "PULL_REQUEST_TEMPLATE.md",
	"docs/pull_request_template.md", "docs/PULL_REQUEST_TEMPLATE.md",
}

// Write renders the three files next to the specification and returns
// their paths.
func Write(files ports.Files, o Options, t delivery.Trace) ([]string, error) {
	lay := layout.Layout{Root: o.Root}
	dir := lay.SpecDir(o.SpecPath)
	template := ""
	for _, p := range PRTemplates {
		if data, err := files.ReadFile(lay.Abs(p)); err == nil {
			template = string(data)
			break
		}
	}
	trace, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{
		filepath.Join(dir, "DELIVERY.md"): []byte(t.Markdown(o.Language)),
		filepath.Join(dir, "trace.json"):  append(trace, '\n'),
		filepath.Join(dir, "PR_BODY.md"):  []byte(t.PRBody(o.Language, template)),
	}
	var paths []string
	for _, name := range []string{"DELIVERY.md", "trace.json", "PR_BODY.md"} {
		p := filepath.Join(dir, name)
		if err := files.WriteFile(p, out[p]); err != nil {
			return paths, fmt.Errorf("writing %s: %w", name, err)
		}
		paths = append(paths, p)
	}
	return paths, nil
}
