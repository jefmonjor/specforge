package domain

import (
	"strings"
	"testing"
)

func TestExtractJSONFromMarkdown(t *testing.T) {
	raw := "Aquí tienes el análisis:\n```json\n{\"generated_at\":\"2026-09-30\",\"findings\":[]}\n```\nEspero que sirva."
	extracted := ExtractJSONFromMarkdown(raw)
	if !strings.HasPrefix(string(extracted), "{") || !strings.HasSuffix(string(extracted), "}") {
		t.Errorf("fallo extrayendo JSON de Markdown: %s", string(extracted))
	}
}

func TestValidateFindingsAgainstSchema(t *testing.T) {
	validJSON := `{
		"generated_at": "2026-09-30T10:00:00Z",
		"findings": [
			{
				"id": "SEC-001",
				"title": "SQL Injection en login",
				"severity": "critical",
				"status": "confirmed",
				"file": "src/auth.ts",
				"line": 42,
				"attack_class": "sql_injection",
				"description": "Concatenación en query",
				"proof_of_impact": "Bypass con ' OR 1=1--"
			}
		]
	}`

	report, err := ValidateFindingsAgainstSchema([]byte(validJSON), nil)
	if err != nil {
		t.Fatalf("error validando JSON válido: %v", err)
	}

	if report.TotalConfirmed != 1 {
		t.Errorf("total_confirmed esperado 1, obtenido: %d", report.TotalConfirmed)
	}

	hasFailures, violating := report.HasFailures("critical")
	if !hasFailures || len(violating) != 1 {
		t.Errorf("debería haber detectado fallo crítico")
	}

	md := report.RenderMarkdown()
	if !strings.Contains(md, "SEC-001") || !strings.Contains(md, "CRITICAL") {
		t.Errorf("el reporte Markdown no contiene la información esperada: %s", md)
	}
}

func TestValidateFindingsInvalidSchema(t *testing.T) {
	invalidJSON := `{
		"findings": [
			{
				"id": "SEC-001",
				"title": "Incompleto"
			}
		]
	}`

	_, err := ValidateFindingsAgainstSchema([]byte(invalidJSON), nil)
	if err == nil {
		t.Errorf("se esperaba error de validación por campos requeridos faltantes")
	}
}
