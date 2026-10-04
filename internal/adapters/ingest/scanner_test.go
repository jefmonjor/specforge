package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectScanner(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-scan-test-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Crear archivos de prueba normales y carpetas a ignorar
	_ = os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte("<project><name>test-app</name></project>"), 0644)
	srcDir := filepath.Join(tmpDir, "src", "main", "java")
	_ = os.MkdirAll(srcDir, 0755)
	_ = os.WriteFile(filepath.Join(srcDir, "MainService.java"), []byte("public class MainService {}"), 0644)

	// Carpetas que deben ser ignoradas
	nodeModules := filepath.Join(tmpDir, "node_modules", "some-pkg")
	_ = os.MkdirAll(nodeModules, 0755)
	_ = os.WriteFile(filepath.Join(nodeModules, "index.js"), []byte("console.log('ignored')"), 0644)

	targetDir := filepath.Join(tmpDir, "target", "classes")
	_ = os.MkdirAll(targetDir, 0755)
	_ = os.WriteFile(filepath.Join(targetDir, "App.class"), []byte("binary data"), 0644)

	scanner := NewProjectScanner()
	ctx, err := scanner.ScanProject(tmpDir)
	if err != nil {
		t.Fatalf("error escaneando: %v", err)
	}

	if ctx.TotalFiles != 2 {
		t.Errorf("se esperaban 2 archivos relevantes, encontrados: %d", ctx.TotalFiles)
	}

	md := ctx.RenderMarkdown()
	if strings.Contains(md, "node_modules") || strings.Contains(md, "target") {
		t.Errorf("el volcado de contexto contiene rutas ignoradas: %s", md)
	}
	if !strings.Contains(md, "pom.xml") || !strings.Contains(md, "MainService.java") {
		t.Errorf("el volcado no contiene los archivos clave del proyecto")
	}
}
