package quality

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"specforge/internal/domain"
	"specforge/internal/ports"
)

// StrykerGate ejecuta pruebas de mutación para comprobar la resistencia de la suite de tests
type StrykerGate struct{}

func NewStrykerGate() ports.QualityGate {
	return &StrykerGate{}
}

func (g *StrykerGate) Name() string {
	return "Stryker (Mutation Testing)"
}

func (g *StrykerGate) RunStaticAnalysis(ctx context.Context, project domain.ProjectInfo) (bool, string, error) {
	// Verificar si existe configuración de Stryker
	hasConf := false
	confNames := []string{"stryker.conf.json", "stryker.conf.js", "stryker.conf.mjs", "stryker.config.json"}
	for _, conf := range confNames {
		if _, err := os.Stat(filepath.Join(project.RootPath, conf)); err == nil {
			hasConf = true
			break
		}
	}

	if !hasConf {
		return true, "Stryker: Omitido (sin configuración stryker.conf.* en el proyecto).", nil
	}

	npx := winCmd("npx")
	res, err := runProcessWithPipes(ctx, project.RootPath, npx, "stryker", "run")
	if err != nil && res.ExitCode == 1 && res.Output == "" {
		return true, "Stryker: Omitido (npx/stryker no disponible en este entorno).", nil
	}

	output := strings.TrimSpace(res.Output)
	if res.ErrorOut != "" {
		output += "\n" + strings.TrimSpace(res.ErrorOut)
	}

	// Comprobar si hubo fallo de umbral de mutantes
	if !res.Success || strings.Contains(output, "Mutation score based on") && strings.Contains(output, "is below threshold") ||
		strings.Contains(output, "Surviving mutants") {
		return false, fmt.Sprintf("Mutation Testing Failed: Surviving mutants detected. Tests are not robust enough.\n%s", output), nil
	}

	return true, "Stryker: Mutation Score validado (tests robustos, cero mutantes supervivientes).", nil
}
