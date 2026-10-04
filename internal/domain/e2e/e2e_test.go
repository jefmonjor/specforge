package e2e

import (
	"strings"
	"testing"
)

var page = Snapshot{
	URL:   "http://localhost:3000/reset",
	Title: "Reset password",
	Elements: []Element{
		{Tag: "input", Selector: "#email"},
		{Tag: "button", Selector: "button:nth-of-type(1)"},
	},
	Text: "Reset your password\nEnter   your EMAIL",
}

func TestValidateOnlyAllowsSelectorsOnThePage(t *testing.T) {
	// Regression: any selector the model invented was sent to the browser,
	// and WaitVisible on a missing one hung forever.
	if err := Validate(Action{Type: Click, Selector: "#email"}, page, "http://localhost:3000", 1); err != nil {
		t.Fatal(err)
	}
	if err := Validate(Action{Type: Click, Selector: "#login"}, page, "http://localhost:3000", 1); err == nil {
		t.Fatal("an invented selector must be rejected")
	}
}

func TestValidateKeepsNavigationOnTheApplication(t *testing.T) {
	// A page under test can prompt-inject "navigate to attacker.site".
	base := "http://localhost:3000"
	for target, ok := range map[string]bool{
		"/login": true, "http://localhost:3000/x": true,
		"https://attacker.example": false, "http://localhost:4000/": false, "//evil.example/x": false,
	} {
		err := Validate(Action{Type: Navigate, Value: target}, page, base, 1)
		if (err == nil) != ok {
			t.Errorf("navigate %q: err=%v, want ok=%v", target, err, ok)
		}
	}
}

func TestValidateAssertions(t *testing.T) {
	ev := &Evidence{Kind: EvidenceText, Value: "x"}
	cases := []struct {
		a  Action
		ok bool
	}{
		{Action{Type: Assert, ThenIndex: 1, Evidence: ev}, true},
		{Action{Type: Assert, ThenIndex: 2, Evidence: ev}, false},
		{Action{Type: Assert, ThenIndex: 1}, false},
		{Action{Type: Assert, ThenIndex: 1, Evidence: &Evidence{Kind: "vibes", Value: "ok"}}, false},
		{Action{Type: Fail, ThenIndex: 1}, true},
		{Action{Type: "hack"}, false},
		{Action{Type: Type, Selector: "#email", Value: strings.Repeat("x", MaxValueLength+1)}, false},
	}
	for i, c := range cases {
		if err := Validate(c.a, page, "http://localhost:3000", 1); (err == nil) != c.ok {
			t.Errorf("case %d: err=%v, want ok=%v", i, err, c.ok)
		}
	}
}

func TestVerifyChecksThePageNotTheAgent(t *testing.T) {
	cases := []struct {
		e  Evidence
		ok bool
	}{
		{Evidence{EvidenceText, "enter your email"}, true},
		{Evidence{EvidenceText, "link expired"}, false},
		{Evidence{EvidenceSelector, "#email"}, true},
		{Evidence{EvidenceSelector, "#missing"}, false},
		{Evidence{EvidenceURL, "/reset"}, true},
		{Evidence{"other", "x"}, false},
	}
	for _, c := range cases {
		if got := Verify(c.e, page); got != c.ok {
			t.Errorf("Verify(%+v) = %v", c.e, got)
		}
	}
}

func TestReport(t *testing.T) {
	r := Report{Spec: "s", Scenarios: []ScenarioResult{
		{Index: 1, Title: "a", Status: Passed, Thens: []ThenResult{{Verified: true}}},
		{Index: 2, Title: "b", Status: Failed, Reason: "timeout", Thens: []ThenResult{{Verified: false}}},
	}}
	if r.PassRate() != 0.5 || !r.Scenarios[0].Done() || r.Scenarios[1].Done() {
		t.Fatalf("report = %+v", r)
	}
	if md := r.Markdown(); !strings.Contains(md, "| 2 | b | failed | 0/1 |") || !strings.Contains(md, "timeout") {
		t.Fatalf("markdown:\n%s", md)
	}
	if (ScenarioResult{}).Done() {
		t.Fatal("a scenario without Then steps is never done")
	}
}
