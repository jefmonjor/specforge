package delivery

import (
	"strings"
	"testing"
)

func sample() Trace {
	return Trace{
		ID: "0001", Title: "Discount codes",
		Spec: Approval{Path: "specs/0001-discount-codes.md", ApprovedBy: "Ana", ApprovedAt: "2026-10-04T10:00:00Z", Seal: "sha256-v1:9394d6452b3a0000000000"},
		Scenarios: []Scenario{
			{Index: 1, Marker: "SDD_0001_001", Title: "A valid code | reduces", Status: Done, Commit: "1673c15abc",
				Tests: []Test{{File: "discount_test.go", Names: []string{"TestSDD_0001_001_Valid"}}}, Gates: "lint=passed duplication=skipped", ReviewNotes: 1},
			{Index: 2, Marker: "SDD_0001_002", Title: "Unknown code", Status: Satisfied, Commit: "684724b"},
		},
		Decisions:  []string{"Which channel? → email"},
		OutOfScope: "- Stacking codes",
	}
}

func TestDeliveryMarkdown(t *testing.T) {
	md := sample().Markdown("en")
	for _, want := range []string{
		"# Delivery · 0001 Discount codes",
		"approved by Ana on 2026-10-04 · `sha256-v1:9394d6452b3a…`",
		"**Plan** no plan",
		"2/2 finished · 1 through RED → GREEN → REFACTOR · 1 already satisfied",
		"| 1 | A valid code \\| reduces | `discount_test.go` · `TestSDD_0001_001_Valid` | `1673c15` | lint ✓ · duplication ⚠ | 1 change(s) requested in review |",
		"| 2 | Unknown code | — | `684724b` | — | already satisfied |",
		"- Which channel? → email",
		"Security audit: not run.",
		"## Out of scope\n\n- Stacking codes",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	if strings.Contains(md, "incomplete") {
		t.Error("a complete delivery must not say it is incomplete")
	}
}

func TestIncompleteDeliveryIsSaidUpFront(t *testing.T) {
	tr := sample()
	tr.Scenarios[1].Status = Pending
	tr.Pending = []string{"Rounding of 10%?"}
	tr.Checks = Checks{Security: &Security{Confirmed: 0, NeedsValidation: 1}, E2E: &E2E{Scenarios: 2, Passed: 1, PassRate: 0.5}}
	md := tr.Markdown("es")
	for _, want := range []string{"⚠ Esta entrega está incompleta", "1 pendientes", "⏳ sin terminar", "## ⚠ Preguntas aún abiertas", "1 por validar", "1/2 escenarios verificados en navegador (50%)"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	body := tr.PRBody("en", "")
	if !strings.Contains(body, "## Not done\n\n- SDD_0001_002 · Unknown code\n- Rounding of 10%?") {
		t.Fatalf("PR body:\n%s", body)
	}
}

func TestPRBodyKeepsTheRepositoryTemplate(t *testing.T) {
	tpl := "## What\n<!-- describe -->\n\n## Checklist\n- [ ] docs\n"
	body := sample().PRBody("en", tpl)
	if !strings.HasPrefix(body, "## What\n\nImplements specification 0001 · Discount codes") ||
		!strings.Contains(body, "<!-- describe -->\n\n## Checklist\n- [ ] docs") || !strings.Contains(body, "## Scenarios") {
		t.Fatalf("PR body:\n%s", body)
	}
	if plain := sample().PRBody("en", ""); !strings.HasPrefix(plain, "## Summary\n\nImplements") {
		t.Fatalf("plain body:\n%s", plain)
	}
}

func TestKnownFailuresAreListedApart(t *testing.T) {
	tr := sample()
	tr.Baseline = &Baseline{Command: "go test ./...", Failures: []string{"pay › TestLegacyRounding"}}
	md := tr.Markdown("en")
	for _, want := range []string{
		"## Known failures (already failing before the loop; not counted against it)\n\n- `pay › TestLegacyRounding`",
		"Baseline: 1 test(s) already failed before the loop (`go test ./...`)",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	if !strings.Contains(tr.PRBody("es", ""), "Línea base: 1 test(s) ya fallaban") {
		t.Error("the PR body states the baseline too")
	}
	tr.Baseline.Failures = nil
	if md := tr.Markdown("en"); strings.Contains(md, "Known failures") || !strings.Contains(md, "the whole suite passed before the loop") {
		t.Errorf("a clean baseline is one line, not a section:\n%s", md)
	}
}

func TestRiskIsShownPerScenario(t *testing.T) {
	tr := sample()
	tr.Scenarios[0].Risk = &Risk{Tier: "high", Lines: 40, Reasons: []string{"`auth/x.go` is a sensitive path"}}
	if md := tr.Markdown("en"); !strings.Contains(md, "risk high: `auth/x.go` is a sensitive path") {
		t.Errorf("missing the risk note:\n%s", md)
	}
}
