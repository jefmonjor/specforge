package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeSlug(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Crear endpoint de pagos", "crear-endpoint-de-pagos"},
		{"Liquidación Intereses AS400 Pasivos Nocturno", "liquidacin-intereses-as400-pasivos"},
		{"Validar Cuentas!!!", "validar-cuentas"},
	}

	for _, c := range cases {
		got := sanitizeSlug(c.input)
		if got != c.expected {
			t.Errorf("para '%s' esperado '%s', obtenido '%s'", c.input, c.expected, got)
		}
	}
}

func TestSealSpecification(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-spec-seal-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	specFile := filepath.Join(tmpDir, "spec.md")
	content := "# Feature: Transferencias Inmediatas\n\nScenario: Saldo suficiente\nGiven saldo 100\nWhen transfiere 50\nThen saldo 50"
	if err := os.WriteFile(specFile, []byte(content), 0644); err != nil {
		t.Fatalf("error escribiendo spec: %v", err)
	}

	hash1, err := sealSpecification(specFile)
	if err != nil {
		t.Fatalf("error sellando spec: %v", err)
	}

	data, _ := os.ReadFile(specFile)
	if !strings.Contains(string(data), "<!-- seal: sha256:"+hash1+" -->") {
		t.Errorf("el archivo no contiene el sello esperado: %s", string(data))
	}

	// Resellar debe ser idempotente
	hash2, err := sealSpecification(specFile)
	if err != nil {
		t.Fatalf("error re-sellando: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("los hashes deberían coincidir, hash1=%s, hash2=%s", hash1, hash2)
	}
}
