package spec

import (
	"fmt"
	"strings"
	"testing"
)

// specWith builds a specification with one invariant and the scenarios
// given as "Title|Then step".
func specWith(inv string, scenarios ...string) (string, *Document) {
	var b strings.Builder
	fmt.Fprintf(&b, "# Pay\n\n## 4. Invariants\n\n- **INV-01**: %s\n\n## 6. Scenarios\n\n```gherkin\nFeature: Pay\n", inv)
	for _, s := range scenarios {
		title, then, _ := strings.Cut(s, "|")
		fmt.Fprintf(&b, "\n  Scenario: %s\n    Given an employee\n    When the payroll runs\n    Then %s\n", title, then)
	}
	b.WriteString("```\n")
	doc, err := Parse(b.String(), ParseOptions{})
	if err != nil {
		panic(err)
	}
	return b.String(), doc
}

func approve(t *testing.T, l Ledger, md string, doc *Document) (Ledger, map[string]ScenarioChange) {
	t.Helper()
	l, changes := Assign(l, "0001", md, doc)
	byTitle := map[string]ScenarioChange{}
	for _, c := range changes {
		byTitle[c.Title] = c
	}
	return l, byTitle
}

func TestTheFirstApprovalNumbersScenariosInOrder(t *testing.T) {
	md, doc := specWith("net ≥ 0", "A|a", "B|b", "C|c")
	_, got := approve(t, Ledger{}, md, doc)
	if got["A"].Marker != "SDD_0001_001" || got["C"].Marker != "SDD_0001_003" || got["B"].Change != Added {
		t.Fatalf("%+v", got)
	}
}

func TestMarkersStayWhenScenariosMove(t *testing.T) {
	md, doc := specWith("net ≥ 0", "A|a", "B|b", "C|c")
	l, _ := approve(t, Ledger{}, md, doc)

	// X inserted after A, C moved before B.
	md2, doc2 := specWith("net ≥ 0", "A|a", "X|x", "C|c", "B|b")
	l2, got := approve(t, l, md2, doc2)
	want := map[string]string{"A": "SDD_0001_001", "B": "SDD_0001_002", "C": "SDD_0001_003", "X": "SDD_0001_004"}
	for title, marker := range want {
		if got[title].Marker != marker {
			t.Errorf("%s: marker %s, want %s", title, got[title].Marker, marker)
		}
	}
	if got["X"].Change != Added || got["C"].Change != Unchanged || got["C"].Index != 3 {
		t.Fatalf("%+v", got)
	}

	// B removed; a new scenario never gets a removed scenario's number.
	md3, doc3 := specWith("net ≥ 0", "A|a", "X|x", "C|c", "Y|y")
	l3, got := approve(t, l2, md3, doc3)
	if got["B"].Change != Removed || got["B"].Marker != "SDD_0001_002" || got["Y"].Marker != "SDD_0001_005" {
		t.Fatalf("%+v", got)
	}
	if removed := l3.RemovedMarkers(); len(removed) != 1 || removed[0] != "SDD_0001_002" {
		t.Fatalf("removed = %v", removed)
	}
	// And a scenario added back with B's title is a new scenario.
	md4, doc4 := specWith("net ≥ 0", "A|a", "X|x", "C|c", "Y|y", "B|b")
	_, got = approve(t, l3, md4, doc4)
	if got["B"].Marker != "SDD_0001_006" {
		t.Fatalf("B came back as %s", got["B"].Marker)
	}
}

func TestAnEditIsModifiedAndARenameKeepsItsMarker(t *testing.T) {
	md, doc := specWith("net ≥ 0", "A|a", "B|b")
	l, _ := approve(t, Ledger{}, md, doc)

	md2, doc2 := specWith("net ≥ 0", "A|a changed", "Renamed B|b")
	l2, got := approve(t, l, md2, doc2)
	if got["A"].Change != Modified || got["A"].Marker != "SDD_0001_001" {
		t.Fatalf("edited steps: %+v", got["A"])
	}
	if got["Renamed B"].Change != Renamed || got["Renamed B"].Marker != "SDD_0001_002" || got["Renamed B"].Was != "B" {
		t.Fatalf("renamed: %+v", got["Renamed B"])
	}
	e, _ := l2.Entry("SDD_0001_001")
	if !strings.Contains(e.Previous, "Then a\n") || !strings.Contains(e.Source, "Then a changed") {
		t.Fatalf("the previous version is kept: %+v", e)
	}
}

func TestChangingAnInvariantChangesTheScenariosThatNameIt(t *testing.T) {
	md, doc := specWith("net ≥ 0", "INV-01 net never negative|the net is 0", "Other|x")
	l, _ := approve(t, Ledger{}, md, doc)
	md2, doc2 := specWith("net ≥ 1", "INV-01 net never negative|the net is 0", "Other|x")
	_, got := approve(t, l, md2, doc2)
	if got["INV-01 net never negative"].Change != Modified || got["Other"].Change != Unchanged {
		t.Fatalf("%+v", got)
	}
}

func TestMarkersForADocument(t *testing.T) {
	md, doc := specWith("net ≥ 0", "A|a", "B|b")
	l, _ := approve(t, Ledger{}, md, doc)
	md2, doc2 := specWith("net ≥ 0", "B|b", "A|a")
	if m := Markers(l, "0001", md2, doc2); m[1] != "SDD_0001_002" || m[2] != "SDD_0001_001" {
		t.Fatalf("markers by index: %v", m)
	}
}
