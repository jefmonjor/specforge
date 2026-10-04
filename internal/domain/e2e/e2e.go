// Package e2e models a browser-driven verification of a scenario.
//
// The agent proposes one action at a time; this package decides whether the
// action is allowed (only selectors that exist on the page, navigation only
// within the application under test) and whether an assertion's evidence is
// really on the page. A scenario passes when every Then step has evidence
// verified here, never because the agent says so.
package e2e

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode"
)

// ActionType is what the agent asks the browser to do.
type ActionType string

const (
	Click    ActionType = "click"
	Type     ActionType = "type"
	Select   ActionType = "select"
	Press    ActionType = "press"
	Scroll   ActionType = "scroll"
	Wait     ActionType = "wait"
	Navigate ActionType = "navigate"
	// Assert claims that a Then step holds, with evidence to verify.
	Assert ActionType = "assert"
	// Fail claims that a Then step cannot hold; it ends the scenario as
	// failed, which needs no verification because it is never a pass.
	Fail ActionType = "fail"
)

var browserActions = []ActionType{Click, Type, Select, Press, Scroll, Wait, Navigate}

// EvidenceKind is how an assertion is checked against the page.
type EvidenceKind string

const (
	EvidenceText     EvidenceKind = "text"
	EvidenceSelector EvidenceKind = "selector"
	EvidenceURL      EvidenceKind = "url"
)

// Evidence is what must be on the page for an assertion to count.
type Evidence struct {
	Kind  EvidenceKind `json:"kind"`
	Value string       `json:"value"`
}

// Action is one decision of the agent.
type Action struct {
	Type        ActionType `json:"action"`
	Selector    string     `json:"selector,omitempty"`
	Value       string     `json:"value,omitempty"`
	ThenIndex   int        `json:"then_index,omitempty"`
	Evidence    *Evidence  `json:"evidence,omitempty"`
	Explanation string     `json:"explanation,omitempty"`
}

// Element is an interactive or informative element of the page.
type Element struct {
	Tag         string `json:"tag"`
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	AriaLabel   string `json:"aria_label,omitempty"`
	Selector    string `json:"selector"`
}

// Snapshot is the page as the agent sees it.
type Snapshot struct {
	URL      string    `json:"url"`
	Title    string    `json:"title"`
	Elements []Element `json:"elements"`
	// Text is the visible text of the page, truncated.
	Text string `json:"text"`
}

// HasSelector reports whether sel is one of the snapshot's selectors.
func (s Snapshot) HasSelector(sel string) bool {
	for _, e := range s.Elements {
		if e.Selector == sel {
			return true
		}
	}
	return false
}

// MaxValueLength bounds text typed into the page.
const MaxValueLength = 2000

// Validate checks a proposed action against the current page. thens is the
// number of Then steps in the scenario; base is the application URL.
func Validate(a Action, s Snapshot, base string, thens int) error {
	switch {
	case slices.Contains(browserActions, a.Type):
	case a.Type == Assert || a.Type == Fail:
		if a.ThenIndex < 1 || a.ThenIndex > thens {
			return fmt.Errorf("then_index %d is not a Then step (1..%d)", a.ThenIndex, thens)
		}
		if a.Type == Assert && (a.Evidence == nil || strings.TrimSpace(a.Evidence.Value) == "") {
			return fmt.Errorf("an assert needs evidence")
		}
		if a.Type == Assert {
			switch a.Evidence.Kind {
			case EvidenceText, EvidenceSelector, EvidenceURL:
			default:
				return fmt.Errorf("unknown evidence kind %q", a.Evidence.Kind)
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown action %q", a.Type)
	}

	if len(a.Value) > MaxValueLength {
		return fmt.Errorf("value longer than %d characters", MaxValueLength)
	}
	switch a.Type {
	case Click, Type, Select:
		if !s.HasSelector(a.Selector) {
			return fmt.Errorf("selector %q is not on the page; use one of the listed selectors", a.Selector)
		}
	case Navigate:
		if !SameOrigin(base, a.Value) {
			return fmt.Errorf("navigation to %q leaves the application under test", a.Value)
		}
	}
	return nil
}

// SameOrigin reports whether target (absolute or relative) stays on base's
// scheme, host and port.
func SameOrigin(base, target string) bool {
	b, err := url.Parse(base)
	if err != nil {
		return false
	}
	t, err := url.Parse(target)
	if err != nil {
		return false
	}
	t = b.ResolveReference(t)
	return t.Scheme == b.Scheme && t.Host == b.Host
}

// Verify checks evidence against the page.
func Verify(e Evidence, s Snapshot) bool {
	switch e.Kind {
	case EvidenceText:
		return strings.Contains(fold(s.Text+" "+s.Title), fold(e.Value))
	case EvidenceSelector:
		return s.HasSelector(e.Value)
	case EvidenceURL:
		return strings.Contains(s.URL, e.Value)
	}
	return false
}

// fold lowercases and collapses whitespace so formatting differences do not
// fail a verification.
func fold(s string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(s), unicode.IsSpace), " ")
}
