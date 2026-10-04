package e2e

import (
	"fmt"
	"strings"
)

// Status of a scenario run.
type Status string

const (
	Passed Status = "passed"
	Failed Status = "failed"
	Error  Status = "error"
)

// ThenResult records whether a Then step was verified and how.
type ThenResult struct {
	Text     string    `json:"text"`
	Verified bool      `json:"verified"`
	Evidence *Evidence `json:"evidence,omitempty"`
}

// StepLog is one action taken.
type StepLog struct {
	N          int    `json:"n"`
	Action     Action `json:"action"`
	Outcome    string `json:"outcome"`
	Screenshot string `json:"screenshot,omitempty"`
}

// ScenarioResult is the result of one scenario.
type ScenarioResult struct {
	Index  int          `json:"index"`
	Title  string       `json:"title"`
	Status Status       `json:"status"`
	Reason string       `json:"reason,omitempty"`
	Thens  []ThenResult `json:"thens"`
	Steps  []StepLog    `json:"steps"`
}

// Done reports whether every Then step is verified.
func (r ScenarioResult) Done() bool {
	for _, t := range r.Thens {
		if !t.Verified {
			return false
		}
	}
	return len(r.Thens) > 0
}

// Report is the result of an E2E run.
type Report struct {
	BaseURL   string           `json:"base_url"`
	Spec      string           `json:"spec"`
	Scenarios []ScenarioResult `json:"scenarios"`
}

// PassRate is the share of scenarios that passed, between 0 and 1.
func (r Report) PassRate() float64 {
	if len(r.Scenarios) == 0 {
		return 0
	}
	n := 0
	for _, s := range r.Scenarios {
		if s.Status == Passed {
			n++
		}
	}
	return float64(n) / float64(len(r.Scenarios))
}

// Markdown renders a short human report.
func (r Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# E2E · %s\n\n- URL: %s\n- Pass rate: %.0f%%\n\n| # | Scenario | Status | Then verified |\n|---|---|---|---|\n",
		r.Spec, r.BaseURL, r.PassRate()*100)
	for _, s := range r.Scenarios {
		v := 0
		for _, t := range s.Thens {
			if t.Verified {
				v++
			}
		}
		fmt.Fprintf(&b, "| %d | %s | %s | %d/%d |\n", s.Index, s.Title, s.Status, v, len(s.Thens))
	}
	for _, s := range r.Scenarios {
		if s.Reason != "" {
			fmt.Fprintf(&b, "\n**%d. %s** — %s\n", s.Index, s.Title, s.Reason)
		}
	}
	return b.String()
}
