package spec

import (
	"reflect"
	"strings"
	"testing"
)

const planDoc = "# Plan\n\n## Approach\nUse `internal/pay` as it is.\n\n" +
	"## Components\n" +
	"- `internal/pay/net.go`: computes the net salary (changed)\n" +
	"- `internal/pay/rounding/down.go`: rounds down to cents (new)\n" +
	"- cmd/pay/main.go — wires the command, changed\n" +
	"- `internal/report/`: the whole package, new\n" +
	"- The domain stays pure.\n\n" +
	"## Tests per scenario\n" +
	"| Marker | Scenario | Test file | Test name |\n| :--- | :--- | :--- | :--- |\n" +
	"| SDD_0001_001 | Net | `internal/pay/net_test.go` | `TestSDD_0001_001_Net` |\n" +
	"| SDD_0001_002 | Bonus | internal/pay/bonus_test.go | TestSDD_0001_002_Bonus |\n\n" +
	"## Risks\n- `docs/notes.md` may be outdated\n"

func TestPlanSurfaces(t *testing.T) {
	s := PlanSurfaces(planDoc)
	wantFiles := []string{"cmd/pay/main.go", "internal/pay/bonus_test.go", "internal/pay/net.go", "internal/pay/net_test.go", "internal/pay/rounding/down.go"}
	if !reflect.DeepEqual(s.Files, wantFiles) {
		t.Errorf("files = %v, want %v", s.Files, wantFiles)
	}
	if want := []string{"internal/pay/rounding", "internal/report"}; !reflect.DeepEqual(s.Dirs, want) {
		t.Errorf("dirs = %v, want %v", s.Dirs, want)
	}
	for p, want := range map[string]bool{
		"internal/pay/net.go":                true,
		"./internal/pay/net_test.go":         true,
		"internal/pay/rounding/half.go":      true, // under a new component's directory
		"internal/report/pdf/render.go":      true,
		"internal/pay/tax.go":                false,
		"docs/notes.md":                      false, // only mentioned in the risks
		"internal/pay/rounding_test.go":      false,
		"internal/reporting/x.go":            false,
		"specs/0001-net/decisions.md":        false,
		"internal/pay/rounding/../tax.go":    false,
		"TestSDD_0001_001_Net":               false,
		"internal/pay/bonus_test.go":         true,
		"cmd/pay/main.go":                    true,
		"internal/pay/rounding/down.go":      true,
		"internal/pay/rounding/down_test.go": true,
	} {
		if got := s.Allows(p); got != want {
			t.Errorf("Allows(%q) = %v, want %v", p, got, want)
		}
	}
	if !strings.Contains(strings.Join(s.List(), ","), "internal/report/") {
		t.Errorf("List shows directories with a slash: %v", s.List())
	}
}

func TestPlanWithoutComponentsHasNoSurfaces(t *testing.T) {
	if s := PlanSurfaces("# Plan\n\n## Approach\nSmall.\n"); !s.Empty() {
		t.Fatalf("surfaces = %+v", s)
	}
}

func TestLintPlanWarnsAboutComponentsWithoutPaths(t *testing.T) {
	issues := LintPlan(planDoc, []string{"SDD_0001_001", "SDD_0001_002"})
	var warned []string
	for _, i := range issues {
		if i.Rule == RulePlanComponent {
			if i.Blocking {
				t.Fatal("a component without a path warns, it does not block")
			}
			warned = append(warned, i.Message)
		}
	}
	if len(warned) != 1 || !strings.Contains(warned[0], "The domain stays pure.") {
		t.Fatalf("warnings = %v", warned)
	}
}

func TestSpanishPlanSections(t *testing.T) {
	plan := "## Componentes\n- `src/pago.py`: nuevo\n\n## Tests por escenario\n| SDD_0001_001 | x | `tests/test_pago.py` | test_x |\n"
	s := PlanSurfaces(plan)
	if !s.Allows("src/pago.py") || !s.Allows("tests/test_pago.py") || !s.Allows("src/otro.py") {
		t.Fatalf("surfaces = %+v", s)
	}
}
