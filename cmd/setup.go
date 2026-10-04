package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"specforge/assets"
	"specforge/internal/adapters/storage"
	"specforge/internal/domain"
)

var (
	flagSetupForce bool
	flagSetupAgent string
	flagSetupStack string
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Prepara el proyecto con el baseline SDD (soporta proyectos nuevos y existentes)",
	Long: `Despliega el baseline corporativo en .specify/ y configura el proyecto:
- Proyectos NUEVOS (Greenfield): Inicializa arquetipos completos con --stack (react, java, go, python).
- Proyectos EXISTENTES (Brownfield): Detecta la tecnología automáticamente y preserva al 100% el código previo.`,
	RunE: runSetup,
}

func init() {
	setupCmd.Flags().BoolVarP(&flagSetupForce, "force", "f", false, "sobrescribir archivos preexistentes en .specify/")
	setupCmd.Flags().StringVar(&flagSetupAgent, "agent", "", "forzar agente específico ('gemini' o 'claude')")
	setupCmd.Flags().StringVar(&flagSetupStack, "stack", "", "inicializar arquetipo para proyectos nuevos ('react', 'java', 'go', 'python')")

	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	logger := storage.GetLogger()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error obteniendo directorio actual: %w", err)
	}

	// Protección de rama: prohibido ejecutar setup en master
	if isGitMasterBranch(cwd) {
		fmt.Println("ERROR: Estas en la rama 'master'. El framework SDD obliga a trabajar en una rama de feature para habilitar Pull Requests.")
		fmt.Println("Crea y muevete a una rama de feature antes de ejecutar comandos destructivos.")
		os.Exit(1)
	}

	// 1. Cargar configuración o usar defaults
	store := storage.NewConfigStorage(cfgFile)
	cfg, err := store.Load()
	if err != nil {
		logger.Warn("No se encontró ~/.specforge/config.json (%v). Usando configuración por defecto.", err)
		cfg = domain.NewDefaultConfig()
	}

	if flagSetupAgent != "" {
		cfg.Agent = strings.ToLower(flagSetupAgent)
	}

	baselineVer := assets.GetBaselineVersion()
	fmt.Printf("==================================================================\n")
	fmt.Printf("🚀 SpecForge — Setup de Proyecto (Baseline: %s)\n", baselineVer)
	fmt.Printf("==================================================================\n")

	// 2. Crear estructura en .specify/ y extraer assets embebidos
	specifyDir := filepath.Join(cwd, ".specify")
	fmt.Println("\n[1/4] Extrayendo baseline embebido a .specify/...")

	subdirs := []string{"memory", "standards", "templates", "scaffold", "openspec", "security-audit"}
	for _, sub := range subdirs {
		target := filepath.Join(specifyDir, sub)
		if err := assets.ExtractDir(sub, target, flagSetupForce); err != nil {
			return fmt.Errorf("error extrayendo componente %s: %w", sub, err)
		}
	}
	fmt.Println("  ✓ Estándares, constitución v2.0/v3.0 y plantillas extraídos.")

	// 3. Inyectar sello de versión en el spec-template
	specTemplatePath := filepath.Join(specifyDir, "templates", "spec-template.md")
	if data, err := os.ReadFile(specTemplatePath); err == nil {
		content := string(data)
		sealHeader := fmt.Sprintf("<!-- baseline: %s -->\n", baselineVer)
		if !strings.Contains(content, "baseline: sdd-baseline") {
			content = sealHeader + content
			_ = os.WriteFile(specTemplatePath, []byte(content), 0644)
		}
	}

	// 4. Instalar comandos del agente de IA
	fmt.Printf("\n[2/4] Configurando comandos de agente (%s)...\n", cfg.Agent)
	if cfg.Agent == "gemini" {
		geminiCommandsDir := filepath.Join(cwd, ".gemini")
		_ = os.MkdirAll(geminiCommandsDir, 0755)
		if err := assets.ExtractDir("commands/gemini/.gemini", geminiCommandsDir, flagSetupForce); err != nil {
			logger.Warn("Aviso extrayendo comandos de gemini: %v", err)
		}
		fmt.Println("  ✓ Comandos /speckit instalados en .gemini/commands/")
	}

	// 4.1. Desplegar Cerebro Contextual del Agente (.sdd/agent/)
	agentContextDir := filepath.Join(cwd, ".sdd", "agent")
	_ = os.MkdirAll(agentContextDir, 0755)
	memoryFiles := []struct {
		name     string
		preserve bool
	}{
		{"agente.md", false},
		{"persona.md", false},
		{"ng-rules.md", false},
		{"glossary.md", true},
		{"references.md", true},
		{"lessons.md", true},
	}
	for _, mf := range memoryFiles {
		targetFile := filepath.Join(agentContextDir, mf.name)
		if mf.preserve && fileExists(targetFile) && !flagSetupForce {
			continue // Preservar aprendizaje histórico y glosario del equipo
		}
		data, err := assets.ReadEmbeddedFile("agent-context/" + mf.name)
		if err == nil {
			_ = os.WriteFile(targetFile, data, 0644)
		}
	}
	fmt.Println("  ✓ Cerebro Contextual (6 archivos de memoria explícita) desplegado en .sdd/agent/")

	// 5. Blindaje de Git (.gitignore y .geminiignore)
	fmt.Println("\n[3/4] Blindando control de versiones (.gitignore)...")
	ensureGitIgnoreEntry(cwd, ".gitignore", []string{".specify/", ".sdd-cache/", ".gemini/", ".sdd/", "values/"})
	ensureGitIgnoreEntry(cwd, ".geminiignore", []string{"!.specify/"})
	fmt.Println("  ✓ .specify/, .sdd-cache/, .gemini/, .sdd/ y values/ excluidos de Git.")

	// Generar plantilla de values local para Tekton (GitOps)
	valuesDir := filepath.Join(cwd, "values")
	_ = os.MkdirAll(valuesDir, 0755)
	devValuesFile := filepath.Join(valuesDir, "dev-app-values.yaml")
	if _, err := os.Stat(devValuesFile); os.IsNotExist(err) || flagSetupForce {
		appName := filepath.Base(cwd)
		valuesContent := fmt.Sprintf(`# ==============================================================================
# Tekton / OpenShift GitOps Values Template
# NOTA: Este archivo es una referencia local para sincronizar con el repo de values.
# La carpeta 'values/' está excluida en .gitignore conforme al estándar GitOps.
# ==============================================================================
app:
  name: "%s"
  environment: "dev"
  replicas: 2

image:
  repository: "image-registry.openshift-image-registry.svc:5000/app-dev/%s"
  tag: "latest"
  pullPolicy: "Always"

probes:
  liveness:
    path: "/health"
    port: 8080
    initialDelaySeconds: 30
    periodSeconds: 10
  readiness:
    path: "/health"
    port: 8080
    initialDelaySeconds: 15
    periodSeconds: 5

resources:
  limits:
    cpu: "1000m"
    memory: "1Gi"
  requests:
    cpu: "250m"
    memory: "512Mi"

service:
  type: "ClusterIP"
  port: 8080
  targetPort: 8080
`, appName, appName)
		_ = os.WriteFile(devValuesFile, []byte(valuesContent), 0644)
		fmt.Printf("  ✓ Plantilla Tekton GitOps generada en: %s\n", devValuesFile)
	}

	// 6. Clasificación de Proyecto: Greenfield (Nuevo) vs Brownfield (Existente)
	fmt.Println("\n[4/4] Clasificación del Proyecto (Greenfield vs Brownfield)...")
	projInfo := domain.DetectProject(cwd)

	stack := strings.ToLower(flagSetupStack)
	if stack == "" && projInfo.Type == domain.ProjectUnknown && isDirectoryEmpty(cwd) {
		fmt.Println("  ℹ️ Directorio vacío detectado (Proyecto Nuevo / Greenfield).")
		fmt.Println("  Selecciona un arquetipo inicial:")
		fmt.Println("    1) React + TypeScript (Vite + Vitest + Knip + Stryker)")
		fmt.Println("    2) Java (Spring Boot + ArchUnit + Maven)")
		fmt.Println("    3) Go (Clean Architecture)")
		fmt.Println("    4) Python (FastAPI + pytest)")
		fmt.Println("    5) Ninguno (solo desplegar estándares SDD)")
		fmt.Print("  Opción [5]: ")
		reader := bufio.NewReader(os.Stdin)
		opt, _ := reader.ReadString('\n')
		switch strings.TrimSpace(opt) {
		case "1":
			stack = "react"
		case "2":
			stack = "java"
		case "3":
			stack = "go"
		case "4":
			stack = "python"
		}
	}

	if stack != "" {
		fmt.Printf("  [Greenfield] Desplegando arquetipo base '%s'...\n", stack)
		appName := filepath.Base(cwd)
		if err := deployScaffold(cwd, stack, appName); err != nil {
			logger.Warn("Aviso desplegando scaffold: %v", err)
		} else {
			fmt.Printf("  ✓ Arquetipo '%s' inicializado con éxito en la raíz.\n", stack)
			projInfo = domain.DetectProject(cwd)
		}
	} else if projInfo.Type != domain.ProjectUnknown {
		fmt.Printf("  ✓ [Brownfield] Proyecto existente detectado: %s\n", projInfo.Type)
		fmt.Println("  ✓ Código fuente y configuración existente preservados al 100%.")
	} else {
		fmt.Println("  ✓ Estándares SDD desplegados sin arquetipo de código.")
	}

	if projInfo.Type == domain.ProjectReact || projInfo.Type == domain.ProjectAngular || projInfo.Type == domain.ProjectNode {
		qualityConfigs := []struct {
			src  string
			dest string
		}{
			{"scaffold/react/knip.json", "knip.json"},
			{"scaffold/react/.jscpd.json", ".jscpd.json"},
			{"scaffold/react/stryker.conf.json", "stryker.conf.json"},
		}
		for _, qc := range qualityConfigs {
			destFile := filepath.Join(cwd, qc.dest)
			if _, err := os.Stat(destFile); os.IsNotExist(err) || flagSetupForce {
				if data, err := assets.ReadEmbeddedFile(qc.src); err == nil {
					_ = os.WriteFile(destFile, data, 0644)
				}
			}
		}
	}

	fmt.Println("\n==================================================================")
	fmt.Println("✓ Proyecto listo para desarrollo asistido por IA.")
	fmt.Println("  Ejecuta 'specforge loop' para iniciar el ciclo TDD.")
	fmt.Println("==================================================================")
	logger.Info("setup ejecutado con éxito en %s (Stack: %s)", cwd, projInfo.Type)

	return nil
}

func deployScaffold(targetDir, stack, appName string) error {
	switch stack {
	case "react":
		if err := assets.ExtractDir("scaffold/react", targetDir, false); err != nil {
			return err
		}
		replaceInFile(filepath.Join(targetDir, "package.json"), "{{APP}}", appName)
		replaceInFile(filepath.Join(targetDir, "package.json"), "{{DESCRIPTION}}", "Aplicación "+appName)
	case "java":
		if err := assets.ExtractDir("scaffold/java", targetDir, false); err != nil {
			return err
		}
		replaceInFile(filepath.Join(targetDir, "pom.xml"), "{{APP}}", appName)
		replaceInFile(filepath.Join(targetDir, "pom.xml"), "{{DESCRIPTION}}", "Servicio "+appName)
	case "go":
		_ = os.WriteFile(filepath.Join(targetDir, "go.mod"), []byte(fmt.Sprintf("module %s\n\ngo 1.24.0\n", appName)), 0644)
		_ = os.WriteFile(filepath.Join(targetDir, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello "+appName+"\")\n}\n"), 0644)
	case "python":
		_ = os.WriteFile(filepath.Join(targetDir, "pyproject.toml"), []byte(fmt.Sprintf("[project]\nname = \"%s\"\nversion = \"0.1.0\"\n", appName)), 0644)
		_ = os.MkdirAll(filepath.Join(targetDir, "src"), 0755)
		_ = os.WriteFile(filepath.Join(targetDir, "src", "main.py"), []byte("def main():\n    print('Hello "+appName+"')\n\nif __name__ == '__main__':\n    main()\n"), 0644)
	}
	return nil
}

func replaceInFile(path, oldStr, newStr string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	content := strings.ReplaceAll(string(data), oldStr, newStr)
	_ = os.WriteFile(path, []byte(content), 0644)
}

func isDirectoryEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if name != ".git" && name != ".specify" && name != ".sdd-cache" && name != ".sdd" {
			return false
		}
	}
	return true
}

func ensureGitIgnoreEntry(rootDir, fileName string, entries []string) {
	filePath := filepath.Join(rootDir, fileName)
	existingContent := ""
	if data, err := os.ReadFile(filePath); err == nil {
		existingContent = string(data)
	}

	modified := false
	for _, entry := range entries {
		if !strings.Contains(existingContent, entry) {
			if existingContent != "" && !strings.HasSuffix(existingContent, "\n") {
				existingContent += "\n"
			}
			existingContent += entry + "\n"
			modified = true
		}
	}

	if modified {
		_ = os.WriteFile(filePath, []byte(existingContent), 0644)
	}
}

func isGitMasterBranch(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	branch := strings.TrimSpace(string(out))
	return branch == "master"
}
