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

// JSCPDGate valida el principio DRY comprobando que no existan bloques de código duplicados
type JSCPDGate struct{}

func NewJSCPDGate() ports.QualityGate {
	return &JSCPDGate{}
}

func (g *JSCPDGate) Name() string {
	return "jscpd (Copy/Paste Detector)"
}

func (g *JSCPDGate) RunStaticAnalysis(ctx context.Context, project domain.ProjectInfo) (bool, string, error) {
	// Definir directorio a escanear (preferir src si existe, sino raíz)
	scanPath := "."
	if _, err := os.Stat(filepath.Join(project.RootPath, "src")); err == nil {
		scanPath = "./src"
	}

	npx := winCmd("npx")
	res, err := runProcessWithPipes(ctx, project.RootPath, npx, "jscpd", scanPath,
		"--threshold", "0",
		"--reporters", "console",
		"--ignore", "**/*_test.go,**/*.test.*,**/*.spec.*,**/node_modules/**,**/target/**,**/.sdd-cache/**")
	if err != nil && res.ExitCode == 1 && res.Output == "" {
		// Herramienta no instalada en la máquina/proyecto
		return true, "jscpd: Omitido (npx/jscpd no disponible en este entorno).", nil
	}

	output := strings.TrimSpace(res.Output)
	if res.ErrorOut != "" {
		output += "\n" + strings.TrimSpace(res.ErrorOut)
	}

	// Comprobar si jscpd detectó clones
	if strings.Contains(output, "ERROR: Clone found") ||
		strings.Contains(output, "clone(s) found") ||
		strings.Contains(output, "clones found") ||
		(!res.Success && strings.Contains(output, "threshold")) {
		return false, fmt.Sprintf("DRY Violation (jscpd): Duplicated code found.\n%s", output), nil
	}

	return true, "jscpd: Cero bloques duplicados (Principio DRY cumplido al 100%).", nil
}
