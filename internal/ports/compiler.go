package ports

import (
	"context"
	"specforge/internal/domain"
)

// BuildResult encapsula la salida de la compilación o ejecución de tests
type BuildResult struct {
	Success  bool
	ExitCode int
	Output   string
	ErrorOut string
}

// Compiler define el contrato de validación y compilación según el stack
type Compiler interface {
	Build(ctx context.Context, project domain.ProjectInfo) (BuildResult, error)
	Test(ctx context.Context, project domain.ProjectInfo) (BuildResult, error)
	GenerateTestStubs(ctx context.Context, project domain.ProjectInfo, scenario domain.Scenario, runner AgentRunner, opts AgentOptions) error
	RunTests(ctx context.Context, project domain.ProjectInfo, scenario string) (passed bool, output string, err error)
}
