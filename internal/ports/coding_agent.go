package ports

import (
	"context"
	"time"
)

// AgentRequest is one invocation of a coding agent CLI.
type AgentRequest struct {
	// Prompt is sent through stdin (headless) or a private temp file
	// (interactive), never as a command-line argument.
	Prompt string
	// Dir is the project root the agent works in.
	Dir string
	// Model optionally overrides the agent's default model.
	Model string
	// ReadDirs are directories outside Dir the agent may read (a legacy
	// repository). SpecForge verifies they are left unchanged.
	ReadDirs []string
	// Commands lets the agent run shell commands without approval. Only a
	// disposable copy of the project may be given to such an agent.
	Commands bool
	// Env holds extra KEY=VALUE pairs for the agent process.
	Env []string
	// Timeout bounds the invocation; zero means only ctx bounds it.
	Timeout time.Duration
}

// Agent drives a coding agent CLI such as Claude Code or Gemini CLI.
type Agent interface {
	// Name is the agent identifier ("claude", "gemini").
	Name() string
	// Run executes the prompt headlessly and returns the agent's stdout.
	Run(ctx context.Context, req AgentRequest) (string, error)
	// Interactive hands the terminal to the agent, seeded with the prompt.
	Interactive(ctx context.Context, req AgentRequest) error
}

// ModelFor picks the model for one phase of work ("plan", "red",
// "review"…). Nil, or "" for a phase, lets the agent use its default.
type ModelFor func(phase string) string

// For returns the model for phase; it is safe on a nil ModelFor.
func (m ModelFor) For(phase string) string {
	if m == nil {
		return ""
	}
	return m(phase)
}
