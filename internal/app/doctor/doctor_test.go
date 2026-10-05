package doctor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"specforge/internal/adapters/fsys"
	"specforge/internal/domain/stack"
	"specforge/internal/ports"
)

type proc struct{ userName string }

func (p proc) Run(_ context.Context, c ports.Command) (ports.CommandResult, error) {
	if len(c.Args) > 0 && c.Args[len(c.Args)-1] == "user.name" {
		return ports.CommandResult{Stdout: p.userName}, nil
	}
	return ports.CommandResult{Stdout: c.Name + " version 1.2.3\nmore"}, nil
}

type readier struct{ r ports.Readiness }

func (r readier) Ready(context.Context, string, stack.Profile) ports.Readiness { return r.r }

type probe struct {
	name string
	readier
}

func (p probe) Name() string               { return p.name }
func (p probe) Applies(stack.Profile) bool { return true }

var goStack = stack.Profile{Kind: stack.Go, Runner: stack.RunnerGo}

func deps(installed ...string) Deps {
	return Deps{
		Proc: proc{userName: "Ana"},
		Find: func(name string) (string, bool) {
			for _, i := range installed {
				if i == name {
					return "/usr/bin/" + name, true
				}
			}
			return "", false
		},
		Files:  fsys.OS{},
		Runner: readier{ports.Readiness{Ready: true, Detail: "/usr/bin/go"}},
		Gates: func(stack.Profile) []ports.Probe {
			return []ports.Probe{probe{"lint", readier{ports.Readiness{Ready: true, Detail: "go vet", Hint: "install golangci-lint"}}}}
		},
		Browser: func() (string, error) { return "", errors.New("none") },
	}
}

func opts() Options {
	return Options{Root: ".", Agent: "claude", AgentBinary: "claude", Profiles: []stack.Profile{goStack}, Commit: true, Version: "6.0.0"}
}

func find(r Report, name string) Check {
	for _, c := range r {
		if c.Name == name {
			return c
		}
	}
	return Check{}
}

func TestEverythingInstalledIsOKWithWarnings(t *testing.T) {
	r, err := Run(context.Background(), deps("claude", "git"), opts())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if c := find(r, "agent"); c.Status != OK || c.Detail != "claude · /usr/bin/claude version 1.2.3" {
		t.Errorf("agent = %+v", c)
	}
	if c := find(r, "gate lint"); c.Status != Warn || c.Hint == "" {
		t.Errorf("a ready gate with a better option warns: %+v", c)
	}
	if c := find(r, "browser (e2e)"); c.Status != Warn || c.Required {
		t.Errorf("a missing browser only warns: %+v", c)
	}
}

func TestMissingRequiredToolsFailWithHints(t *testing.T) {
	d := deps("git")
	d.Runner = readier{ports.Readiness{Detail: "go not found on PATH", Hint: "install Go"}}
	r, err := Run(context.Background(), d, opts())
	var missing *MissingError
	if !errors.As(err, &missing) || !errors.Is(err, ports.ErrToolNotFound) {
		t.Fatalf("want MissingError (exit 4), got %v", err)
	}
	if got := err.Error(); got != "missing: agent, tests" {
		t.Errorf("error = %q", got)
	}
	if c := find(r, "agent"); !strings.Contains(c.Hint, "npm install -g @anthropic-ai/claude-code") {
		t.Errorf("agent hint = %q", c.Hint)
	}
	if c := find(r, "tests"); c.Status != Fail || c.Hint != "install Go" {
		t.Errorf("tests = %+v", c)
	}
}

func TestGitIdentityIsRequiredOnlyToCommit(t *testing.T) {
	d := deps("claude", "git")
	d.Proc = proc{}
	_, err := Run(context.Background(), d, opts())
	if err == nil || !strings.Contains(err.Error(), "git") {
		t.Fatalf("committing without user.name must fail: %v", err)
	}
	o := opts()
	o.Commit = false
	if _, err := Run(context.Background(), d, o); err != nil {
		t.Fatalf("without commits user.name is not needed: %v", err)
	}
}

func TestConfigurationAgentStackAndLegacy(t *testing.T) {
	o := opts()
	o.Config = errors.New("models.deploy: unknown phase")
	o.Agent = ""
	o.Profiles = nil
	o.StackErr = errors.New("no go.mod at /x")
	o.Legacy = "/does/not/exist"
	r, _ := Run(context.Background(), deps("git"), o)
	if c := find(r, "configuration"); c.Status != Fail || c.Detail != "models.deploy: unknown phase" {
		t.Errorf("configuration = %+v", c)
	}
	if c := find(r, "agent"); c.Status != Fail || c.Hint != "specforge init (or --agent claude|gemini)" {
		t.Errorf("agent = %+v", c)
	}
	if c := find(r, "stack"); c.Status != Warn || c.Detail != "no go.mod at /x" {
		t.Errorf("stack = %+v", c)
	}
	if c := find(r, "legacy"); c.Status != Fail {
		t.Errorf("legacy = %+v", c)
	}
}

func TestGuardIsCheckedWhenWired(t *testing.T) {
	d := deps("claude", "git")
	d.Guard = func(agent string) ports.Readiness {
		return ports.Readiness{Detail: "no hook in .claude/settings.json", Hint: "specforge setup"}
	}
	r, err := Run(context.Background(), d, opts())
	if err != nil {
		t.Fatalf("the guard is optional: %v", err)
	}
	if c := find(r, "guard hook"); c.Status != Warn || c.Hint != "specforge setup" {
		t.Errorf("guard = %+v", c)
	}
}
