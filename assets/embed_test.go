package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedFiles(t *testing.T) {
	ver := GetBaselineVersion()
	if ver != "specforge-3.0.0" {
		t.Errorf("GetBaselineVersion esperada 'specforge-3.0.0', obtenida: %s", ver)
	}

	goDoc, err := ReadEmbeddedFile("standards/go.md")
	if err != nil {
		t.Fatalf("error leyendo standards/go.md embebido: %v", err)
	}
	if !strings.Contains(string(goDoc), "Go Standard") {
		t.Errorf("el documento embebido de go no contiene el estándar esperado")
	}

	constDoc, err := ReadEmbeddedFile("memory/constitution.md")
	if err != nil {
		t.Fatalf("error leyendo memory/constitution.md embebido: %v", err)
	}
	if !strings.Contains(string(constDoc), "6.6") {
		t.Errorf("la constitución embebida no contiene la cláusula 6.6")
	}
}

func TestExtractDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-free-embed-extract-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := ExtractDir("standards", tmpDir, true); err != nil {
		t.Fatalf("error extrayendo directorio standards: %v", err)
	}

	extractedGo := filepath.Join(tmpDir, "go.md")
	if _, err := os.Stat(extractedGo); err != nil {
		t.Errorf("no se encontró go.md tras la extracción: %v", err)
	}
}
