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
