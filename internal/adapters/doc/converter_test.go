package doc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocumentConverterIsSupported(t *testing.T) {
	c := NewDocumentConverter()

	supported := []string{".pdf", ".docx", ".xlsx", ".pptx", ".csv", ".html"}
	for _, ext := range supported {
		if !c.IsSupported(ext) {
			t.Errorf("la extensión %s debería estar soportada", ext)
		}
	}

	unsupported := []string{".exe", ".bin", ".dll", ".zip"}
	for _, ext := range unsupported {
		if c.IsSupported(ext) {
			t.Errorf("la extensión %s NO debería estar soportada", ext)
		}
	}
}

func TestDocumentConverterDirectoryScan(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-doc-test-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Crear archivo CSV de prueba
	csvFile := filepath.Join(tmpDir, "conciliacion.csv")
	csvContent := "Cuenta,Importe,Estado\nES123456,150.00,OK\nES987654,230.50,PENDIENTE\n"
	_ = os.WriteFile(csvFile, []byte(csvContent), 0644)

	c := NewDocumentConverter()
	targetMd := filepath.Join(tmpDir, "conciliacion.md")

	out, err := c.ConvertFile(csvFile, targetMd)
	if err != nil {
		t.Logf("Aviso: markitdown no pudo ejecutarse directamente en test: %v", err)
		return
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no se generó el markdown: %v", err)
	}

	if !strings.Contains(string(data), "ES123456") {
		t.Errorf("el markdown generado no contiene los datos esperados: %s", string(data))
	}
}
