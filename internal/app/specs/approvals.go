package specs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"specforge/internal/app/layout"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// ReadLedger reads the scenario ledger of a specification: empty before
// its first approval with one.
func ReadLedger(files ports.Files, lay layout.Layout, specPath string) (spec.Ledger, error) {
	data, err := files.ReadFile(lay.Scenarios(specPath))
	if errors.Is(err, os.ErrNotExist) {
		return spec.Ledger{}, nil
	}
	if err != nil {
		return spec.Ledger{}, err
	}
	var l spec.Ledger
	if err := json.Unmarshal(data, &l); err != nil {
		return spec.Ledger{}, fmt.Errorf("reading %s: %w", lay.Rel(lay.Scenarios(specPath)), err)
	}
	return l, nil
}

// MarkersOf lists the markers of the scenarios of doc, in order.
func MarkersOf(files ports.Files, lay layout.Layout, specPath, markdown string, doc *spec.Document) ([]string, error) {
	l, err := ReadLedger(files, lay, specPath)
	if err != nil {
		return nil, err
	}
	byIndex := spec.Markers(l, spec.IDFromPath(specPath), markdown, doc)
	out := make([]string, len(doc.Scenarios))
	for i, sc := range doc.Scenarios {
		out[i] = byIndex[sc.Index]
	}
	return out, nil
}

// recordApproval gives the approved scenarios their markers, keeps them in
// the ledger, and appends the approval and what changed to
// specs/NNNN-slug/approvals.md, the history of what was approved when.
func (s Service) recordApproval(e Entry, content, by, hash string, at time.Time) ([]spec.ScenarioChange, error) {
	doc, err := spec.Parse(content, spec.ParseOptions{Languages: []string{s.Language}})
	if err != nil {
		return nil, err
	}
	ledger, err := s.ledger(e)
	if err != nil {
		return nil, err
	}
	ledger, changes := spec.Assign(ledger, e.ID, content, doc)
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := s.Files.WriteFile(s.Layout.Scenarios(e.Path), append(data, '\n')); err != nil {
		return nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "### %s · %s · %s:%s\n\n", at.UTC().Format(time.RFC3339), by, spec.SealV1, hash)
	for _, c := range changes {
		switch c.Change {
		case spec.Removed:
			fmt.Fprintf(&b, "- %s · %s · %s\n", c.Change, c.Marker, c.Title)
		case spec.Renamed:
			fmt.Fprintf(&b, "- %s · %s · %d · %s (was %s)\n", c.Change, c.Marker, c.Index, c.Title, c.Was)
		default:
			fmt.Fprintf(&b, "- %s · %s · %d · %s\n", c.Change, c.Marker, c.Index, c.Title)
		}
	}
	b.WriteString("\n")
	return changes, s.Files.AppendFile(s.Layout.Approvals(e.Path), []byte(b.String()))
}

// ledger reads the ledger. A specification approved before the ledger
// existed starts from its loop's state, whose markers its tests already
// carry, matched by title; without one, from the scenarios' positions.
func (s Service) ledger(e Entry) (spec.Ledger, error) {
	l, err := ReadLedger(s.Files, s.Layout, e.Path)
	if err != nil || len(l.Scenarios) > 0 {
		return l, err
	}
	data, err := s.Files.ReadFile(s.Layout.State(e.Path))
	if err != nil {
		return spec.Ledger{}, nil
	}
	var st tdd.State
	if json.Unmarshal(data, &st) != nil {
		return spec.Ledger{}, nil
	}
	for _, sc := range st.Scenarios {
		if sc.Marker != "" && !slices.ContainsFunc(l.Scenarios, func(x spec.LedgerEntry) bool { return x.Title == sc.Title }) {
			l.Scenarios = append(l.Scenarios, spec.LedgerEntry{Marker: sc.Marker, Title: sc.Title, Fingerprint: spec.LegacyPrefix + sc.Fingerprint})
		}
	}
	return l, nil
}

// Positional reports a specification approved before the ledger existed
// whose loop state is not here (another clone, or no loop yet): its next
// approval numbers the scenarios by position, which may not be the
// markers its tests carry.
func (s Service) Positional(e Entry) bool {
	if l, err := ReadLedger(s.Files, s.Layout, e.Path); err != nil || len(l.Scenarios) > 0 {
		return false
	}
	if !s.Files.Exists(s.Layout.Approvals(e.Path)) {
		return false
	}
	l, err := s.ledger(e)
	return err == nil && len(l.Scenarios) == 0
}

// Preview is what approving the specification as it is now would record:
// each scenario's marker and how it changed. Nothing is written.
func (s Service) Preview(e Entry) ([]spec.ScenarioChange, error) {
	data, err := s.Files.ReadFile(e.Path)
	if err != nil {
		return nil, err
	}
	doc, err := spec.Parse(string(data), spec.ParseOptions{Languages: []string{s.Language}})
	if err != nil {
		return nil, err
	}
	ledger, err := s.ledger(e)
	if err != nil {
		return nil, err
	}
	_, changes := spec.Assign(ledger, e.ID, string(data), doc)
	return changes, nil
}
