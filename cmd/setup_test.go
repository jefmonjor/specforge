package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupInCleanDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-setup-test-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("error obteniendo wd: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("error cambiando a temp: %v", err)
	}

	cmd := rootCmd
	cmd.SetArgs([]string{"setup", "--agent", "gemini"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error ejecutando sdd setup: %v", err)
	}

	// 1. Verificar .specify/
	constFile := filepath.Join(tmpDir, ".specify", "memory", "constitution.md")
	data, err := os.ReadFile(constFile)
	if err != nil {
		t.Fatalf("no se extrajo constitution.md: %v", err)
	}
	if !strings.Contains(string(data), "6.6") {
		t.Errorf("constitution.md extraída no contiene la cláusula 6.6")
	}

	// 2. Verificar .gitignore
	giData, err := os.ReadFile(filepath.Join(tmpDir, ".gitignore"))
	if err != nil {
		t.Fatalf("no se creó .gitignore: %v", err)
	}
	if !strings.Contains(string(giData), ".specify/") || !strings.Contains(string(giData), ".sdd-cache/") || !strings.Contains(string(giData), "values/") {
		t.Errorf(".gitignore no contiene las entradas esperadas: %s", string(giData))
	}

	// 3. Verificar values/dev-app-values.yaml para Tekton
	valData, err := os.ReadFile(filepath.Join(tmpDir, "values", "dev-app-values.yaml"))
	if err != nil {
		t.Fatalf("no se creó values/dev-app-values.yaml: %v", err)
	}
	if !strings.Contains(string(valData), "/health") {
		t.Errorf("dev-app-values.yaml no contiene probe /health")
	}
}

func TestSetupGreenfieldScaffoldReact(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-greenfield-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	cmd := rootCmd
	cmd.SetArgs([]string{"setup", "--stack", "react"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error ejecutando setup --stack react: %v", err)
	}

	// Verificar archivos creados en raíz
	pkgData, err := os.ReadFile(filepath.Join(tmpDir, "package.json"))
	if err != nil {
		t.Fatalf("no se creó package.json: %v", err)
	}
	if strings.Contains(string(pkgData), "{{APP}}") {
		t.Errorf("{{APP}} no fue reemplazado en package.json")
	}

	// Verificar configs de calidad
	if _, err := os.Stat(filepath.Join(tmpDir, "knip.json")); err != nil {
		t.Errorf("knip.json no fue creado en la raíz")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".jscpd.json")); err != nil {
		t.Errorf(".jscpd.json no fue creado en la raíz")
	}
}

func TestSetupBrownfieldPreservation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-brownfield-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	// Crear código preexistente
	customPom := "<project><name>mi-servicio-antiguo</name></project>"
	_ = os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte(customPom), 0644)

	cmd := rootCmd
	cmd.SetArgs([]string{"setup"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("error ejecutando setup en brownfield: %v", err)
	}

	// Verificar que pom.xml se preservó exactamente igual
	afterData, _ := os.ReadFile(filepath.Join(tmpDir, "pom.xml"))
	if string(afterData) != customPom {
		t.Errorf("pom.xml fue alterado en proyecto brownfield: %s", string(afterData))
	}

	// Verificar que .specify se desplegó
	if _, err := os.Stat(filepath.Join(tmpDir, ".specify", "standards", "java.md")); err != nil {
		t.Errorf("no se desplegó el estándar java en brownfield")
	}
}
