// Package doctor checks that a machine and a project have what SpecForge
// needs, before a loop finds out halfway: the agent, git, the stack's test
// runner, the quality gates' tools, a browser for E2E, the configuration
// and, when installed, the destructive-command guard. Each check says what
// it found and, when something is missing, how to install it.
package doctor

import (
	"context"
	"strings"
	"time"

	"specforge/internal/domain/stack"
	"specforge/internal/ports"
)

// Status of one check.
type Status string

const (
	OK   Status = "ok"
	Warn Status = "warn"
	Fail Status = "fail"
)

// Check is one line of the report.
type Check struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Status   Status `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

// Report is every check, in order.
type Report []Check

// Missing returns the required checks that failed.
func (r Report) Missing() []Check {
	var out []Check
	for _, c := range r {
		if c.Required && c.Status == Fail {
			out = append(out, c)
		}
	}
	return out
}

// MissingError reports required checks that failed. It is a missing tool
// or setting: the CLI exits with code 4.
type MissingError struct{ Checks []Check }

func (e *MissingError) Error() string {
	names := make([]string, len(e.Checks))
	for i, c := range e.Checks {
		names[i] = c.Name
	}
	return "missing: " + strings.Join(names, ", ")
}

// Unwrap makes a MissingError a ports.ErrToolNotFound.
func (e *MissingError) Unwrap() error { return ports.ErrToolNotFound }

// Deps are what the checks look at.
type Deps struct {
	Proc ports.CommandRunner
	// Find resolves an executable on PATH.
	Find  func(name string) (string, bool)
	Files ports.Files
	// Runner tells whether the stack's test runner is installed.
	Runner ports.Readier
	// Gates returns the quality gates of a stack.
	Gates func(p stack.Profile) []ports.Probe
	// Browser finds Chrome, Chromium or Edge.
	Browser func() (string, error)
	// Guard reports whether the destructive-command guard is installed
	// for the agent; nil skips the check.
	Guard func(agent string) ports.Readiness
}

// Options describe what was configured and detected.
type Options struct {
	Root string
	// Agent is the configured agent ("" when none) and AgentBinary its
	// executable.
	Agent, AgentBinary string
	// Config is the error of reading specforge.yaml and the user
	// configuration, nil when they are valid.
	Config error
	// Profiles are the stacks to check (the configured one, or every one
	// detected); StackErr explains why there is none.
	Profiles []stack.Profile
	StackErr error
	// Legacy is the configured legacy repository (absolute), if any.
	Legacy string
	// Commit is true when the loop records a commit per scenario.
	Commit  bool
	Version string
}

// versionTimeout bounds each `--version` call.
const versionTimeout = 10 * time.Second

// Run performs every check. The error is a *MissingError when a required
// check failed; the report is complete either way.
func Run(ctx context.Context, d Deps, o Options) (Report, error) {
	r := Report{
		{Name: "specforge", Status: OK, Detail: o.Version},
		configCheck(o),
		agentCheck(ctx, d, o),
		gitCheck(ctx, d, o),
	}
	r = append(r, stackChecks(ctx, d, o)...)
	if o.Legacy != "" {
		r = append(r, legacyCheck(d, o))
	}
	r = append(r, browserCheck(d))
	if d.Guard != nil && o.Agent != "" {
		r = append(r, fromReadiness("guard hook", false, d.Guard(o.Agent)))
	}
	if missing := r.Missing(); len(missing) > 0 {
		return r, &MissingError{Checks: missing}
	}
	return r, ctx.Err()
}

func configCheck(o Options) Check {
	c := Check{Name: "configuration", Required: true, Status: OK, Detail: "specforge.yaml and user settings are valid"}
	if o.Config != nil {
		c.Status, c.Detail, c.Hint = Fail, o.Config.Error(), "fix specforge.yaml (unknown keys and invalid values are errors)"
	}
	return c
}

func agentCheck(ctx context.Context, d Deps, o Options) Check {
	c := Check{Name: "agent", Required: true}
	switch {
	case o.Agent == "":
		c.Status, c.Detail, c.Hint = Fail, "no agent configured", "specforge init (or --agent claude|gemini)"
		return c
	case o.AgentBinary == "":
		c.Status, c.Detail = Fail, "unknown agent "+o.Agent
		return c
	}
	path, ok := d.Find(o.AgentBinary)
	if !ok {
		c.Status, c.Detail = Fail, o.AgentBinary+" not found on PATH"
		c.Hint = map[string]string{
			"claude": "npm install -g @anthropic-ai/claude-code, then run claude once to sign in",
			"gemini": "npm install -g @google/gemini-cli, then run gemini once to sign in",
		}[o.AgentBinary]
		return c
	}
	c.Status, c.Detail = OK, o.Agent+" · "+orPath(version(ctx, d, path), path)
	return c
}

func gitCheck(ctx context.Context, d Deps, o Options) Check {
	c := Check{Name: "git", Required: true}
	path, ok := d.Find("git")
	if !ok {
		c.Status, c.Detail, c.Hint = Fail, "git not found on PATH", "install git: https://git-scm.com/downloads"
		return c
	}
	c.Status, c.Detail = OK, orPath(version(ctx, d, path), path)
	if !o.Commit {
		return c
	}
	res, err := d.Proc.Run(ctx, ports.Command{Name: path, Args: []string{"-C", o.Root, "config", "user.name"}, Timeout: versionTimeout})
	if err != nil || strings.TrimSpace(res.Stdout) == "" {
		c.Status, c.Detail = Fail, c.Detail+" · no user.name: the loop cannot commit"
		c.Hint = `git config user.name "Your Name" && git config user.email you@example.com (or commit: false in specforge.yaml)`
	}
	return c
}

func stackChecks(ctx context.Context, d Deps, o Options) []Check {
	if len(o.Profiles) == 0 {
		detail := "no stack detected"
		if o.StackErr != nil {
			detail = o.StackErr.Error()
		}
		return []Check{{Name: "stack", Status: Warn, Detail: detail, Hint: "add stack: to specforge.yaml, or start one with specforge setup --new java|react|python|go"}}
	}
	var out []Check
	for _, p := range o.Profiles {
		out = append(out, Check{Name: "stack", Status: OK, Detail: p.Name()},
			fromReadiness("tests", true, d.Runner.Ready(ctx, o.Root, p)))
		for _, g := range d.Gates(p) {
			out = append(out, fromReadiness("gate "+g.Name(), false, g.Ready(ctx, o.Root, p)))
		}
	}
	return out
}

func legacyCheck(d Deps, o Options) Check {
	c := Check{Name: "legacy", Required: true, Status: OK, Detail: o.Legacy}
	if !d.Files.Exists(o.Legacy) {
		c.Status, c.Detail, c.Hint = Fail, o.Legacy+" does not exist", "fix migration.legacy in specforge.yaml"
	}
	return c
}

func browserCheck(d Deps) Check {
	c := Check{Name: "browser (e2e)", Status: OK}
	path, err := d.Browser()
	if err != nil {
		c.Status, c.Detail, c.Hint = Warn, "no Chrome, Chromium or Edge found", "only `specforge e2e` needs it: install one or set CHROME_PATH"
		return c
	}
	c.Detail = path
	return c
}

func fromReadiness(name string, required bool, r ports.Readiness) Check {
	c := Check{Name: name, Required: required, Status: OK, Detail: r.Detail, Hint: r.Hint}
	switch {
	case !r.Ready && required:
		c.Status = Fail
	case !r.Ready || r.Hint != "":
		c.Status = Warn
	}
	return c
}

// version returns the first line of `tool --version`, or "".
func version(ctx context.Context, d Deps, path string) string {
	res, err := d.Proc.Run(ctx, ports.Command{Name: path, Args: []string{"--version"}, Timeout: versionTimeout})
	if err != nil || !res.Success() {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(res.Combined()), "\n")
	return strings.TrimSpace(line)
}

func orPath(v, path string) string {
	if v == "" {
		return path
	}
	return v
}
