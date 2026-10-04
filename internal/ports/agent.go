package ports

import "context"

// AgentOptions define los parámetros de ejecución del agente de IA
type AgentOptions struct {
	Model       string
	Temperature float64
	Project     string
	Location    string
	WorkingDir  string
	Debug       bool
}

// AgentRunner abstrae la ejecución de un agente (Gemini CLI o Claude Code)
type AgentRunner interface {
	Name() string
	RunHeadless(ctx context.Context, prompt string, opts AgentOptions) (string, error)
	RunInteractive(ctx context.Context, initialPrompt string, opts AgentOptions) error
}
