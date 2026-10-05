// Package guardhook connects the destructive-command guard to the coding
// agents: it decides what to do with a command an agent is about to run,
// speaks each agent's hook protocol, and installs the hook in the agent's
// project settings. The guard is defence in depth: SpecForge verifies the
// loop's work with or without it.
package guardhook

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"specforge/internal/domain/guard"
)

// Modes of the guard (guard.mode in specforge.yaml).
const (
	// Block refuses every destructive command.
	Block = "block"
	// Confirm asks the developer, when the agent can ask (Claude Code's
	// permission prompt); inside the loop it blocks.
	Confirm = "confirm"
	// Off lets everything through but what is never allowed.
	Off = "off"
)

// LoopEnv is set for every agent the loop runs: nobody is there to
// confirm, so the guard blocks.
const LoopEnv = "SPECFORGE_LOOP"

// Verdict is what happens to a command.
type Verdict string

const (
	Allow Verdict = "allow"
	Ask   Verdict = "ask"
	Deny  Verdict = "deny"
)

// Decision is the verdict on a command and why.
type Decision struct {
	Verdict Verdict
	Matches []guard.Match
}

// Reason renders the matches for the agent and the developer.
func (d Decision) Reason() string {
	var parts []string
	for _, m := range d.Matches {
		parts = append(parts, fmt.Sprintf("%s (%s: `%s`)", m.Reason, m.Kind, m.Segment))
	}
	return "SpecForge guard: " + strings.Join(parts, "; ")
}

// Decide judges a command. A hard deny is refused whatever the mode; an
// allowed pattern lets a match through; confirm asks only when someone can
// answer.
func Decide(command, mode string, allow []string, canAsk bool) Decision {
	var hard, soft []guard.Match
	for _, m := range guard.Recognize(command) {
		switch {
		case m.HardDeny:
			hard = append(hard, m)
		case !guard.Allowed(m, allow):
			soft = append(soft, m)
		}
	}
	switch {
	case len(hard) > 0:
		return Decision{Verdict: Deny, Matches: hard}
	case len(soft) == 0 || mode == Off:
		return Decision{Verdict: Allow}
	case mode == Confirm && canAsk:
		return Decision{Verdict: Ask, Matches: soft}
	}
	return Decision{Verdict: Deny, Matches: soft}
}

// Agents whose hooks SpecForge speaks.
var Agents = []string{"claude", "gemini"}

// hookInput is the JSON both agents send to a pre-tool hook.
type hookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// shellTools are the tools that run a shell command.
var shellTools = map[string]string{"claude": "Bash", "gemini": "run_shell_command"}

// Command reads the shell command out of a hook's input; "" when the tool
// is not the agent's shell tool.
func Command(agent string, in io.Reader) (string, error) {
	var h hookInput
	if err := json.NewDecoder(in).Decode(&h); err != nil {
		return "", fmt.Errorf("reading the %s hook input: %w", agent, err)
	}
	if h.ToolName != "" && h.ToolName != shellTools[agent] {
		return "", nil
	}
	return h.ToolInput.Command, nil
}

// Respond writes the decision in the agent's hook protocol and returns the
// exit code: 2 blocks (the agent sees the reason on stderr); 0 lets the
// command run, or, for Claude Code, asks the developer.
func Respond(agent string, d Decision, stdout, stderr io.Writer) int {
	switch d.Verdict {
	case Allow:
		return 0
	case Ask:
		if agent == "claude" {
			out := map[string]any{"hookSpecificOutput": map[string]any{
				"hookEventName":            "PreToolUse",
				"permissionDecision":       "ask",
				"permissionDecisionReason": d.Reason(),
			}}
			_ = json.NewEncoder(stdout).Encode(out)
			return 0
		}
	}
	fmt.Fprintln(stderr, d.Reason()+". Do it another way, or ask the developer to run it.")
	return 2
}

// SelfTest runs a known destructive command through the guard as a hook
// would, and reports whether it is blocked.
func SelfTest(mode string, allow []string) error {
	d := Decide("git reset --hard HEAD", mode, allow, false)
	if mode != Off && d.Verdict != Deny {
		return fmt.Errorf("the guard let `git reset --hard HEAD` through (mode %s)", mode)
	}
	if hard := Decide("rm -rf /", mode, allow, false); hard.Verdict != Deny {
		return fmt.Errorf("the guard let `rm -rf /` through")
	}
	return nil
}
