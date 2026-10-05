package review

import (
	"fmt"
	"slices"
	"strings"

	"specforge/internal/domain/risk"
)

// Lens is one angle of review. Each runs as its own read-only agent turn.
type Lens string

const (
	// LensRisk: security, data, money, permissions, process execution.
	LensRisk Lens = "risk"
	// LensReliability: correctness against the specification and its
	// invariants, edge cases, error handling.
	LensReliability Lens = "reliability"
	// LensReadability: names from the ubiquitous language, structure,
	// duplication, dead code.
	LensReadability Lens = "readability"
	// LensResilience: failure modes: timeouts, retries, partial writes,
	// concurrency, resource leaks.
	LensResilience Lens = "resilience"
)

// AllLenses in the order they run.
var AllLenses = []Lens{LensRisk, LensReliability, LensReadability, LensResilience}

// ParseLenses reads review.lenses: "auto" (nil: by risk), "off" (none) or
// a list of lens names.
func ParseLenses(values []string) (lenses []Lens, auto bool, err error) {
	if len(values) == 0 || (len(values) == 1 && strings.EqualFold(values[0], "auto")) {
		return nil, true, nil
	}
	if len(values) == 1 && strings.EqualFold(values[0], "off") {
		return []Lens{}, false, nil
	}
	for _, v := range values {
		l := Lens(strings.ToLower(strings.TrimSpace(v)))
		if !slices.Contains(AllLenses, l) {
			return nil, false, fmt.Errorf("unknown review lens %q (use auto, off, or any of risk, reliability, readability, resilience)", v)
		}
		if !slices.Contains(lenses, l) {
			lenses = append(lenses, l)
		}
	}
	return lenses, false, nil
}

// Select picks the lenses for a change by its tier: none when passive,
// every lens when high, and for a medium change the one that matters most:
// risk when a path is sensitive, reliability when code changed, else
// readability. Code is whatever the rules do not call documentation.
func Select(tier risk.Tier, files []string, rules risk.Rules) []Lens {
	switch tier {
	case risk.High:
		return slices.Clone(AllLenses)
	case risk.Medium:
		switch {
		case slices.ContainsFunc(files, rules.Sensitive):
			return []Lens{LensRisk}
		case slices.ContainsFunc(files, func(p string) bool { return !rules.Documentation(p) }):
			return []Lens{LensReliability}
		}
		return []Lens{LensReadability}
	}
	return nil
}
