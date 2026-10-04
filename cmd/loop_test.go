package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"specforge/internal/domain"
)

func TestLoopIntegrityValidationOnResume(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-loop-test-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(oldWd)
		flagLoopResume = false
	}()
	_ = os.Chdir(tmpDir)

	// Crear especificación inicial sellada
	specContent := "# Feature: Test Resiliencia\n\nScenario: Escenario 1\nGiven precondicion\nWhen accion\nThen resultado"
	cleanHash := domain.CalculateCleanSpecHash(specContent)
	sealedContent := specContent + "\n\n<!-- seal: sha256:" + cleanHash + " -->\n"

	specPath := filepath.Join(tmpDir, "spec.md")
	_ = os.WriteFile(specPath, []byte(sealedContent), 0644)

	// Crear estado guardado
	scenarios, _ := domain.ParseScenarios(specContent)
	state := domain.NewTDDState(tmpDir, specPath, cleanHash, scenarios)
	state.CurrentPhase = domain.TDDPhaseGreen
	_ = state.Save(tmpDir)

	// 1. Modificar spec.md manualmente sin generar nuevo sello (tampering)
	tamperedContent := sealedContent + "\nModificación manual maliciosa o accidental"
	_ = os.WriteFile(specPath, []byte(tamperedContent), 0644)

	// 2. Ejecutar sdd loop --resume -> DEBE FALLAR FATALMENTE
	cmd := rootCmd
	cmd.SetArgs([]string{"loop", "--resume"})
	err = cmd.Execute()
	if err == nil {
		t.Errorf("se esperaba fallo fatal por alteración de la especificación")
	} else if !strings.Contains(strings.ToLower(err.Error()), "la especificación ha sido modificada manualmente") {
		t.Errorf("error inesperado al verificar integridad: %v", err)
	}
}

func TestLoopAbortsOnNeedsClarification(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-clarification-test-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(oldWd)
		flagLoopResume = false
	}()
	_ = os.Chdir(tmpDir)

	flagLoopResume = false

	// Crear especificación con [NEEDS CLARIFICATION]
	rawSpec := `# Feature: Test Con Dudas
Scenario: Escenario 1
Given estado
When accion
Then resultado

## Cuestiones Abiertas
- [NEEDS CLARIFICATION]: ¿El tiempo de cortesía de 15 minutos aplica también en Nochevieja?
`
	cleanHash := domain.CalculateCleanSpecHash(rawSpec)
	sealedSpec := rawSpec + "\n\n<!-- seal: sha256:" + cleanHash + " -->\n"

	specPath := filepath.Join(tmpDir, "spec.md")
	_ = os.WriteFile(specPath, []byte(sealedSpec), 0644)

	cmd := rootCmd
	cmd.SetArgs([]string{"loop", "--spec", specPath})
	err = cmd.Execute()
	if err == nil {
		t.Errorf("se esperaba bloqueo duro por [NEEDS CLARIFICATION]")
	} else if !strings.Contains(err.Error(), "[NEEDS CLARIFICATION]") {
		t.Errorf("mensaje de error inesperado: %v", err)
	}
}
