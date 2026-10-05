package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Ledger gives each scenario of a specification its marker for good. A
// marker names the scenario's tests, its review and verification records
// and its row in the plan, so it must not move when another scenario is
// inserted, removed or reordered. The ledger keeps a scenario's marker
// while its title stays (its steps may change), and a renamed scenario's
// marker while its steps stay. A removed scenario's marker is never given
// to another.
type Ledger struct {
	// Next is the number the next new scenario gets.
	Next      int           `json:"next"`
	Scenarios []LedgerEntry `json:"scenarios"`
}

// LedgerEntry is one scenario as it was last approved.
type LedgerEntry struct {
	Marker string `json:"marker"`
	Title  string `json:"title"`
	// Fingerprint covers the scenario's steps and the invariants it
	// names. A ledger started from an older loop holds that loop's
	// fingerprint instead, as LegacyPrefix + the scenario's text hash.
	Fingerprint string `json:"fingerprint"`
	// Steps covers the steps alone: same steps under a new title is a
	// rename.
	Steps  string `json:"steps"`
	Source string `json:"source"`
	// Previous is the scenario as it was before its last modification.
	Previous string `json:"previous,omitempty"`
	Removed  bool   `json:"removed,omitempty"`
}

// LegacyPrefix marks a fingerprint an older SpecForge recorded: a hash of
// the scenario's text (Scenario.Fingerprint).
const LegacyPrefix = "legacy:"

// Change is how a scenario changed between two approvals.
type Change string

const (
	Added     Change = "ADDED"
	Modified  Change = "MODIFIED"
	Renamed   Change = "RENAMED"
	Unchanged Change = "UNCHANGED"
	Removed   Change = "REMOVED"
)

// ScenarioChange is one scenario of an approval and how it changed.
type ScenarioChange struct {
	Change Change
	Marker string
	// Index is its position in the specification (0 when removed).
	Index int
	Title string
	// Was is the former title of a renamed scenario.
	Was string
}

// Assign gives every scenario of doc its marker, keeping those of the
// ledger, and returns the ledger as it is now with what changed. An empty
// ledger numbers the scenarios in order, which is how markers were given
// before the ledger existed.
func Assign(l Ledger, specID, markdown string, doc *Document) (Ledger, []ScenarioChange) {
	next := max(l.Next, 1)
	var active []LedgerEntry
	for _, e := range l.Scenarios {
		if n := markerNumber(e.Marker); n >= next {
			next = n + 1
		}
		if !e.Removed {
			active = append(active, e)
		}
	}
	used := make([]bool, len(active))
	claim := func(match func(LedgerEntry) bool) (LedgerEntry, bool) {
		for i, e := range active {
			if !used[i] && match(e) {
				used[i] = true
				return e, true
			}
		}
		return LedgerEntry{}, false
	}

	entries := make([]LedgerEntry, len(doc.Scenarios))
	changes := make([]ScenarioChange, len(doc.Scenarios))
	var pending []int
	for i, sc := range doc.Scenarios {
		entries[i] = LedgerEntry{Title: sc.Title, Fingerprint: Fingerprint(markdown, sc), Steps: stepsFingerprint(sc), Source: sc.Source}
		changes[i] = ScenarioChange{Index: sc.Index, Title: sc.Title}
		old, ok := claim(func(e LedgerEntry) bool { return e.Title == sc.Title })
		if !ok {
			pending = append(pending, i)
			continue
		}
		entries[i].Marker = old.Marker
		changes[i].Change = Unchanged
		entries[i].Previous = old.Previous
		if old.Fingerprint != entries[i].Fingerprint && old.Fingerprint != LegacyPrefix+sc.Fingerprint() {
			changes[i].Change, entries[i].Previous = Modified, old.Source
		}
	}
	for _, i := range pending {
		if old, ok := claim(func(e LedgerEntry) bool { return e.Steps == entries[i].Steps }); ok {
			entries[i].Marker, entries[i].Previous = old.Marker, old.Previous
			changes[i].Change, changes[i].Was = Renamed, old.Title
			continue
		}
		entries[i].Marker = Marker(specID, next)
		next++
		changes[i].Change = Added
	}
	for i := range changes {
		changes[i].Marker = entries[i].Marker
	}

	out := Ledger{Next: next, Scenarios: entries}
	for i, e := range active {
		if !used[i] {
			e.Removed = true
			out.Scenarios = append(out.Scenarios, e)
			changes = append(changes, ScenarioChange{Change: Removed, Marker: e.Marker, Title: e.Title})
		}
	}
	for _, e := range l.Scenarios {
		if e.Removed {
			out.Scenarios = append(out.Scenarios, e)
		}
	}
	return out, changes
}

// Markers gives the marker of each scenario of doc, by its index.
func Markers(l Ledger, specID, markdown string, doc *Document) map[int]string {
	_, changes := Assign(l, specID, markdown, doc)
	out := map[int]string{}
	for _, c := range changes {
		if c.Index > 0 {
			out[c.Index] = c.Marker
		}
	}
	return out
}

// Entry returns the ledger entry of marker.
func (l Ledger) Entry(marker string) (LedgerEntry, bool) {
	for _, e := range l.Scenarios {
		if e.Marker == marker {
			return e, true
		}
	}
	return LedgerEntry{}, false
}

// RemovedMarkers lists the markers of scenarios no longer in the
// specification.
func (l Ledger) RemovedMarkers() []string {
	var out []string
	for _, e := range l.Scenarios {
		if e.Removed {
			out = append(out, e.Marker)
		}
	}
	return out
}

// Fingerprint identifies what a scenario asks for: its steps, and the
// definition of every invariant it names, so changing what an invariant
// means changes the scenarios that test it. The title is left out: a
// renamed scenario asks for the same thing.
func Fingerprint(markdown string, sc Scenario) string {
	h := sha256.New()
	h.Write([]byte(stepsFingerprint(sc)))
	defs := invariantDefinitions(markdown)
	for _, id := range InvariantRefs(sc.Title + "\n" + sc.Source) {
		fmt.Fprintf(h, "\n%s", defs[id])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// stepsFingerprint identifies the steps alone, title aside.
func stepsFingerprint(sc Scenario) string {
	h := sha256.New()
	for _, st := range sc.Steps {
		fmt.Fprintf(h, "%v|%s|%s|%q\n", st.Kind, st.Text, st.DocString, st.Table)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// invariantDefinitions maps each invariant to its defining line.
func invariantDefinitions(markdown string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(Section(markdown, InvariantsTitle), "\n") {
		if m := invariantEntry.FindStringSubmatch(line); m != nil {
			out[m[1]] = strings.TrimSpace(line)
		}
	}
	return out
}

var markerSuffix = regexp.MustCompile(`_(\d+)$`)

func markerNumber(marker string) int {
	m := markerSuffix.FindStringSubmatch(marker)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// Active lists the markers of the scenarios still in the specification.
func (l Ledger) Active() []string {
	var out []string
	for _, e := range l.Scenarios {
		if !e.Removed {
			out = append(out, e.Marker)
		}
	}
	return slices.Clip(out)
}
