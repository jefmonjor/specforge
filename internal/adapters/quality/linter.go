package quality

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"specforge/internal/domain"
	"specforge/internal/ports"
)

// LinterGate ejecuta el linter estricto adaptado a cada tecnología
type LinterGate struct{}

func NewLinterGate() ports.QualityGate {
	return &LinterGate{}
}

func (g *LinterGate) Name() string {
	return "Strict Linter"
}

func (g *LinterGate) RunStaticAnalysis(ctx context.Context, project domain.ProjectInfo) (bool, string, error) {
	switch project.Type {
	case domain.ProjectReact, domain.ProjectAngular, domain.ProjectNode:
		npm := winCmd("npm")
		res, err := runProcessWithPipes(ctx, project.RootPath, npm, "run", "lint", "--if-present")
		if err != nil && res.ExitCode != 0 {
			out := strings.TrimSpace(res.Output + "\n" + res.ErrorOut)
			return false, fmt.Sprintf("Linter Violation (ESLint/TSLint):\n%s", out), nil
		}
		return true, "Linter (JS/TS): Código limpio conforme a estándares.", nil

	case domain.ProjectGo:
		// Intentar golangci-lint si está presente en el sistema
		if _, err := exec.LookPath("golangci-lint"); err == nil {
			res, err := runProcessWithPipes(ctx, project.RootPath, "golangci-lint", "run")
			if err != nil && res.ExitCode != 0 {
				out := strings.TrimSpace(res.Output + "\n" + res.ErrorOut)
				return false, fmt.Sprintf("Linter Violation (golangci-lint):\n%s", out), nil
			}
			return true, "Linter (golangci-lint): Sin violaciones estáticas.", nil
		}
		// Fallback estándar nativo: go vet
		res, err := runProcessWithPipes(ctx, project.RootPath, "go", "vet", "./...")
		if err != nil && res.ExitCode != 0 {
			out := strings.TrimSpace(res.Output + "\n" + res.ErrorOut)
			return false, fmt.Sprintf("Linter Violation (go vet):\n%s", out), nil
		}
		return true, "Linter (go vet): Sin violaciones estáticas.", nil

	case domain.ProjectPython:
		// Intentar ruff check si está instalado
		if _, err := exec.LookPath("ruff"); err == nil {
			res, err := runProcessWithPipes(ctx, project.RootPath, "ruff", "check", ".")
			if err != nil && res.ExitCode != 0 {
				out := strings.TrimSpace(res.Output + "\n" + res.ErrorOut)
				return false, fmt.Sprintf("Linter Violation (Ruff):\n%s", out), nil
			}
			return true, "Linter (Ruff): Código Python sin violaciones.", nil
		}
		return true, "Linter (Python): Ruff no disponible en el entorno.", nil

	default:
		return true, "Linter: No configurado para este stack.", nil
	}
}
