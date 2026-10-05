package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jefmonjor/specforge/v6/internal/app/doctor"
)

func TestDoctorSaysWhatIsMissingAndExits4(t *testing.T) {
	h := newHarness(t)
	h.write("go.mod", "module example.com/m\n\ngo 1.22\n")
	h.expect(4, "doctor")
	for _, want := range []string{"✗ agent · no agent configured", "→ specforge init", "✓ stack · go", "Something required is missing"} {
		if !strings.Contains(h.err.String(), want) {
			t.Errorf("missing %q in:\n%s", want, h.err)
		}
	}
}

func TestDoctorJSON(t *testing.T) {
	h := newHarness(t)
	h.write("specforge.yaml", "models:\n  deploy: x\n")
	h.expect(4, "doctor", "--json")
	var r doctor.Report
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatalf("stdout is not a report: %v\n%s", err, h.out)
	}
	var cfg doctor.Check
	for _, c := range r {
		if c.Name == "configuration" {
			cfg = c
		}
	}
	if cfg.Status != doctor.Fail || !strings.Contains(cfg.Detail, "models.deploy") {
		t.Fatalf("an invalid specforge.yaml is reported, not fatal: %+v", cfg)
	}
}
