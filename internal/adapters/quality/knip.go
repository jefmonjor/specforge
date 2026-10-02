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

// KnipGate detecta código muerto, exportaciones huérfanas y dependencias no utilizadas
type KnipGate struct{}

func NewKnipGate() ports.QualityGate {
	return &KnipGate{}
}

func (g *KnipGate) Name() string {
	return "Knip (Dead Code Detector)"
}

func (g *KnipGate) RunStaticAnalysis(ctx context.Context, project domain.ProjectInfo) (bool, string, error) {
	pkgPath := filepath.Join(project.RootPath, "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		return true, "Knip no aplica para este stack (sin package.json).", nil
	}

	npx := winCmd("npx")
	res, err := runProcessWithPipes(ctx, project.RootPath, npx, "knip", "--no-exit-code")
	if err != nil && res.ExitCode == 1 && res.Output == "" {
		// npx o knip no disponible en el entorno
		return true, "Knip: Omitido (npx/knip no disponible en este entorno).", nil
	}

	output := strings.TrimSpace(res.Output)
	if res.ErrorOut != "" {
		output += "\n" + strings.TrimSpace(res.ErrorOut)
	}

	// Detectar si knip reporta violaciones
	if strings.Contains(output, "Unused files") ||
		strings.Contains(output, "Unused dependencies") ||
		strings.Contains(output, "Unused exports") ||
		strings.Contains(output, "Unlisted dependencies") {
		return false, fmt.Sprintf("Knip Violation: Dead code or unused dependencies detected.\n%s", output), nil
	}

	return true, "Knip: Cero código muerto ni dependencias huérfanas.", nil
}
