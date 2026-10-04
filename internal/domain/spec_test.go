package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseScenarios(t *testing.T) {
	spec := `# Feature: Depósito de Intereses
Como cliente bancario
Quiero recibir intereses periódicos

Scenario: Cálculo de intereses base 360
  Given una cuenta con 10000 EUR
  When se liquidan intereses al 2%
  Then el saldo resultante es 10200 EUR

Escenario: Penalización por cancelación anticipada
  Dado un depósito cancelado antes de vencimiento
  Cuando se calcula la liquidación
  Entonces se aplica retención del 1%
`
	scenarios, err := ParseScenarios(spec)
	if err != nil {
		t.Fatalf("error parseando escenarios: %v", err)
	}

	if len(scenarios) != 2 {
		t.Fatalf("se esperaban 2 escenarios, obtenidos: %d", len(scenarios))
	}

	if scenarios[0].Title != "Cálculo de intereses base 360" {
		t.Errorf("título incorrecto en escenario 1: %s", scenarios[0].Title)
	}
	if len(scenarios[0].Given) == 0 || len(scenarios[0].When) == 0 || len(scenarios[0].Then) == 0 {
		t.Errorf("fallo extrayendo Given/When/Then en escenario 1: %+v", scenarios[0])
	}

	if scenarios[1].Title != "Penalización por cancelación anticipada" {
		t.Errorf("título incorrecto en escenario 2: %s", scenarios[1].Title)
	}
}

func TestVerifySpecIntegrity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-spec-integ-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	content := "# Feature: Test\nScenario: Demo\nGiven test\nWhen run\nThen pass"
	cleanHash := CalculateCleanSpecHash(content)
	sealed := content + "\n\n<!-- seal: sha256:" + cleanHash + " -->\n"

	specFile := filepath.Join(tmpDir, "spec.md")
	if err := os.WriteFile(specFile, []byte(sealed), 0644); err != nil {
		t.Fatalf("error escribiendo: %v", err)
	}

	// 1. Debe pasar cuando el archivo está intacto
	if err := VerifySpecIntegrity(specFile, cleanHash); err != nil {
		t.Errorf("integridad debería ser válida: %v", err)
	}

	// 2. Modificación manual no autorizada (tampering)
	tampered := sealed + "\nCambio no autorizado por el usuario"
	_ = os.WriteFile(specFile, []byte(tampered), 0644)

	// Debe fallar fatalmente
	if err := VerifySpecIntegrity(specFile, cleanHash); err == nil {
		t.Errorf("debería haber fallado por modificación manual")
	} else if !strings.Contains(strings.ToLower(err.Error()), "la especificación ha sido modificada manualmente") {
		t.Errorf("mensaje de error inesperado: %v", err)
	}
}

func TestExtractNeedsClarification(t *testing.T) {
	// 1. Caso sin aclaraciones
	cleanContent := "# Feature: Reserva\nScenario: OK\nGiven libre\nWhen reservar\nThen confirmada"
	if items := ExtractNeedsClarification(cleanContent); len(items) != 0 {
		t.Errorf("se esperaban 0 aclaraciones, obtenidas %d", len(items))
	}

	// 2. Caso con cuestiones abiertas
	contentWithClarifications := `# Feature: Reserva
Scenario: OK
Given libre
When reservar
Then confirmada

## Cuestiones Abiertas
- [NEEDS CLARIFICATION]: ¿El tiempo de cortesía de 15 minutos aplica también en Nochevieja?
* [NEEDS CLARIFICATION] Confirmar si se cobra comisión por cancelación tardía.
`
	items := ExtractNeedsClarification(contentWithClarifications)
	if len(items) != 2 {
		t.Fatalf("se esperaban 2 aclaraciones, obtenidas: %d", len(items))
	}
	if !strings.Contains(items[0], "Nochevieja") {
		t.Errorf("cuestión 1 no esperada: %s", items[0])
	}
	if !strings.Contains(items[1], "comisión") {
		t.Errorf("cuestión 2 no esperada: %s", items[1])
	}
}
