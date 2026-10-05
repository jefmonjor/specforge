package verification

import (
	"strings"
	"testing"
)

func TestProblems(t *testing.T) {
	required := []string{"INV-01", "INV-02", "SDD_0001_001"}
	good := Report{
		Verdicts: []Verdict{{ID: "INV-01", Status: Met}, {ID: "INV-02", Status: Unmet}, {ID: "SDD_0001_001", Status: Unverified, Reason: "needs a database"}},
		Blockers: []Blocker{{ID: "INV-02", Command: "go run . pay --gross -5", Observed: "net: -5.00", Expected: "error"}},
	}
	if p := good.Problems(required); len(p) != 0 {
		t.Fatalf("a complete report has no problems: %v", p)
	}
	bad := Report{
		Verdicts: []Verdict{{ID: "INV-01", Status: Unmet}, {ID: "INV-02", Status: Unmet}},
		Blockers: []Blocker{{ID: "INV-02", Command: "go run .", Observed: ""}, {ID: "INV-99", Command: "x", Observed: "y"}},
	}
	got := strings.Join(bad.Problems(required), "\n")
	for _, want := range []string{
		"no verdict for SDD_0001_001",
		"blocker INV-02 has no command or no observed output",
		"blocker INV-99 is not a requirement that was asked",
		"INV-01 is unmet but has no blocker",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestOnly(t *testing.T) {
	r := Report{
		Verdicts: []Verdict{{ID: "INV-01", Status: Met}, {ID: "INV-02", Status: Unmet}},
		Blockers: []Blocker{{ID: "INV-02", Command: "c", Observed: "o"}},
	}
	o := r.Only([]string{"INV-02"})
	if len(o.Verdicts) != 1 || o.Verdicts[0].ID != "INV-02" || len(o.Blockers) != 1 || r.Count(Met) != 1 || strings.Join(r.BlockerIDs(), ",") != "INV-02" {
		t.Fatalf("Only = %+v", o)
	}
}
