package security

import (
	"strings"
	"testing"
)

func report() *Report {
	r := &Report{Findings: []Finding{
		{ID: "A", Severity: Critical, Status: Confirmed},
		{ID: "B", Severity: Medium, Status: Confirmed},
		{ID: "C", Severity: High, Status: NeedsValidation},
		{ID: "D", Severity: Medium, Status: NeedsValidation},
		{ID: "E", Severity: Critical, Status: Rejected},
	}}
	r.Recount()
	return r
}

func ids(fs []Finding) string {
	var s []string
	for _, f := range fs {
		s = append(s, f.ID)
	}
	return strings.Join(s, ",")
}

func TestBlockingFailsClosedOnUnresolvedSevereFindings(t *testing.T) {
	r := report()
	if got := ids(r.Blocking(Info)); got != "A,C,B" {
		t.Errorf("threshold info: %s", got)
	}
	if got := ids(r.Blocking(High)); got != "A,C" {
		t.Errorf("threshold high: %s", got)
	}
	if got := ids(r.Blocking(Critical)); got != "A" {
		t.Errorf("threshold critical: %s (a high needs_validation is below a critical threshold)", got)
	}
	r.Findings[2].Decision = "rejected by developer"
	if got := ids(r.Blocking(High)); got != "A" {
		t.Errorf("a cleared finding must not block: %s", got)
	}
}

func TestRecountIgnoresTheModelsTotals(t *testing.T) {
	r := report()
	r.TotalConfirmed = 99
	r.Recount()
	if r.TotalConfirmed != 2 || r.TotalNeedsValidation != 2 || r.TotalRejected != 1 {
		t.Fatalf("totals = %d %d %d", r.TotalConfirmed, r.TotalNeedsValidation, r.TotalRejected)
	}
}

func TestParseSeverity(t *testing.T) {
	for in, want := range map[string]Severity{"": Info, "confirmed": Info, "HIGH": High, " critical ": Critical} {
		if got, err := ParseSeverity(in); err != nil || got != want {
			t.Errorf("ParseSeverity(%q) = %q, %v", in, got, err)
		}
	}
	// Regression: a typo such as "hgih" used to match nothing, so the gate
	// always passed.
	if _, err := ParseSeverity("hgih"); err == nil {
		t.Error("typo must be rejected")
	}
}

func TestValidateAndMerge(t *testing.T) {
	r := &Report{Findings: []Finding{{ID: "X", Severity: "huge", Status: "maybe"}, {ID: "X", Severity: Low, Status: Rejected}}}
	err := r.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate") || !strings.Contains(err.Error(), "huge") {
		t.Fatalf("Validate = %v", err)
	}
	total := &Report{}
	total.Merge(report(), "c1")
	total.Merge(report(), "c2")
	if len(total.Findings) != 10 || total.Findings[5].ID != "c2-A" || total.TotalConfirmed != 4 {
		t.Fatalf("merge: %+v", total)
	}
}

func TestMarkdown(t *testing.T) {
	md := report().Markdown("es")
	for _, want := range []string{"# Informe de auditoría de seguridad", "## Vulnerabilidades confirmadas", "### [A]", "## Requieren validación"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q", want)
		}
	}
	if strings.Contains(md, "[E]") {
		t.Error("rejected findings are not listed")
	}
}
