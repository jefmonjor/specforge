package delivery

import (
	"reflect"
	"strings"
	"testing"
)

func scenarios(lines ...int) []Scenario {
	out := make([]Scenario, len(lines))
	for i, n := range lines {
		out[i] = Scenario{Index: i + 1, Lines: n, Commit: string(rune('a'+i)) + "000000000", Status: Done}
	}
	return out
}

func TestSlices(t *testing.T) {
	cases := []struct {
		name  string
		lines []int
		want  [][]int
		over  []bool
	}{
		{"fits in one", []int{100, 150, 140}, [][]int{{1, 2, 3}}, []bool{false}},
		{"three of 250", []int{250, 250, 250}, [][]int{{1}, {2}, {3}}, []bool{false, false, false}},
		{"greedy and in order", []int{200, 150, 100, 300}, [][]int{{1, 2}, {3, 4}}, []bool{false, false}},
		{"an oversized scenario stands alone", []int{100, 900, 50}, [][]int{{1}, {2}, {3}}, []bool{false, true, false}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Slices(scenarios(c.lines...), 400)
			var ids [][]int
			var over []bool
			for i, s := range got {
				ids = append(ids, s.Scenarios)
				over = append(over, s.Oversized)
				if s.N != i+1 {
					t.Fatalf("slices are numbered in order: %+v", got)
				}
			}
			if !reflect.DeepEqual(ids, c.want) || !reflect.DeepEqual(over, c.over) {
				t.Fatalf("Slices = %v %v, want %v %v", ids, over, c.want, c.over)
			}
		})
	}
	uncommitted := scenarios(100, 100)
	uncommitted[1].Commit = ""
	if got := Slices(uncommitted, 400); len(got) != 1 || len(got[0].Scenarios) != 1 {
		t.Fatalf("a scenario without a commit has nothing to slice: %+v", got)
	}
}

func TestOverBudgetDeliveryProposesSlices(t *testing.T) {
	tr := sample()
	tr.Scenarios = scenarios(250, 250, 250)
	tr.Budget = 400
	tr.Slices = Slices(tr.Scenarios, tr.Budget)
	md := tr.Markdown("en")
	for _, want := range []string{
		"750 authored line(s), over the budget of 400: see the suggested slices",
		"## Suggested slices (stacked pull requests, in order)",
		"- Slice 2 · scenario(s) 2 · 250 line(s) · `git branch specforge/slice-2 b000000`",
		"| 250 line(s)",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	sub := tr.Sub(tr.Slices[1])
	if len(sub.Scenarios) != 1 || sub.Scenarios[0].Index != 2 || len(sub.Slices) != 0 {
		t.Fatalf("Sub = %+v", sub)
	}
	tr.Scenarios, tr.Slices = scenarios(100), nil
	if md := tr.Markdown("en"); !strings.Contains(md, "100 authored line(s) · budget 400") || strings.Contains(md, "Suggested slices") {
		t.Errorf("within the budget: one line, no slices:\n%s", md)
	}
}
