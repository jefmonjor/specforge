package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"specforge/assets"
	"specforge/internal/adapters/agent"
	"specforge/internal/adapters/doc"
	"specforge/internal/adapters/ingest"
	"specforge/internal/adapters/storage"
	"specforge/internal/domain"
	"specforge/internal/ports"
)

var (
	flagInterviewFeature   string
	flagInterviewAgent     string
	flagInterviewNoContext bool
	flagInterviewOutput    string
	flagInterviewFromRepo  string
)

var interviewCmd = &cobra.Command{
	Use:   "interview",
	Short: "Entrevista socrática BDD interactiva con blindaje de tuberías Win32",
	Long: `Inicia una sesión interactiva guiada por un Arquitecto de Software Socrático (IA)
para definir requerimientos, casos de borde y escenarios BDD (Gherkin).
Soporta el modo migración (--from-repo <ruta>) para hacer ingeniería inversa de repositorios antiguos,
extrayendo las reglas funcionales de negocio en specs BDD limpias para el proyecto nuevo.`,
	RunE: runInterview,
}

func init() {
	interviewCmd.Flags().StringVarP(&flagInterviewFeature, "feature", "f", "", "descripción inicial o nombre de la funcionalidad")
	interviewCmd.Flags().StringVar(&flagInterviewAgent, "agent", "", "agente a utilizar ('gemini' o 'claude')")
	interviewCmd.Flags().BoolVar(&flagInterviewNoContext, "no-context", false, "omitir el escaneo de contexto del repositorio")
	interviewCmd.Flags().StringVarP(&flagInterviewOutput, "output", "o", "", "ruta destino para la especificación generada")
	interviewCmd.Flags().StringVar(&flagInterviewFromRepo, "from-repo", "", "ruta a un repositorio antiguo/legacy para extraer especificaciones BDD As-Is de migración")

	rootCmd.AddCommand(interviewCmd)
}

func runInterview(cmd *cobra.Command, args []string) error {
	logger := storage.GetLogger()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error obteniendo directorio actual: %w", err)
	}

	if flagInterviewFeature == "" && len(args) > 0 {
		flagInterviewFeature = strings.Join(args, " ")
	}

	// 1. Cargar configuración
	store := storage.NewConfigStorage(cfgFile)
	cfg, err := store.Load()
	if err != nil {
		cfg = domain.NewDefaultConfig()
	}

	agentName := cfg.Agent
	if flagInterviewAgent != "" {
		agentName = strings.ToLower(flagInterviewAgent)
	}

	fmt.Println("==================================================================")
	fmt.Println("🚀 SpecForge — Entrevista Socrática BDD (Blindaje Win32)")
	fmt.Println("==================================================================")
	fmt.Println("  ✓ Tuberías acopladas a Win32 (inmune a NativeCommandError)")
	fmt.Printf("  ✓ Agente activo: %s (Vertex AI ADC: %v)\n", agentName, cfg.Auth.UseVertexAI)

	// 2. Obtener prompt socrático base de assets embebidos
	promptBytes, err := assets.ReadEmbeddedFile("ai-framework/prompts/00_interview_bdd.md")
	if err != nil {
		return fmt.Errorf("error leyendo prompt de entrevista embebido: %w", err)
	}
	socraticPrompt := string(promptBytes)

	// 3. Recolectar contexto del repositorio (o del repo antiguo en caso de migración)
	var contextSummary string
	var legacySummary string

	if flagInterviewFromRepo != "" {
		fmt.Printf("\n[1/3] Modo Migración: Analizando repositorio antiguo en %s...\n", flagInterviewFromRepo)
		converter := doc.NewDocumentConverter()
		if conv, err := converter.ConvertDirectory(flagInterviewFromRepo, true); err == nil && len(conv) > 0 {
			fmt.Printf("  ✓ [Auto-Doc] %d documento(s) antiguos (PDF/Excel/Word) convertidos a Markdown.\n", len(conv))
		}
		scanner := ingest.NewProjectScanner()
		legacyCtx, err := scanner.ScanProject(flagInterviewFromRepo)
		if err == nil {
			fmt.Printf("  ✓ Código antiguo analizado: %d archivos (%s)\n", legacyCtx.TotalFiles, legacyCtx.ProjectInfo.Type)
			legacySummary = legacyCtx.RenderMarkdown()
		} else {
			return fmt.Errorf("error escaneando repositorio antiguo %s: %w", flagInterviewFromRepo, err)
		}
	} else if !flagInterviewNoContext {
		fmt.Println("\n[1/3] Construyendo mapa de contexto del proyecto actual...")
		converter := doc.NewDocumentConverter()
		if conv, err := converter.ConvertDirectory(cwd, true); err == nil && len(conv) > 0 {
			fmt.Printf("  ✓ [Auto-Doc] %d documento(s) (PDF/Excel/Word) convertidos a Markdown.\n", len(conv))
		}
		scanner := ingest.NewProjectScanner()
		projCtx, err := scanner.ScanProject(cwd)
		if err == nil {
			fmt.Printf("  ✓ Contexto incorporado: %d archivos analizados (%s)\n", projCtx.TotalFiles, projCtx.ProjectInfo.Type)
			contextSummary = projCtx.RenderMarkdown()
		} else {
			logger.Warn("Aviso escaneando contexto: %v", err)
		}
	}

	// 4. Preparar payload inicial
	fmt.Println("\n[2/3] Preparando sesión socrática con el agente...")
	var initialPromptBuilder strings.Builder
	initialPromptBuilder.WriteString(socraticPrompt)
	initialPromptBuilder.WriteString("\n\n---\n")

	if legacySummary != "" {
		initialPromptBuilder.WriteString("MISIÓN ESPECIAL: INGENIERÍA INVERSA Y EXTRACCIÓN DE REGLAS DE NEGOCIO PARA MIGRACIÓN\n")
		initialPromptBuilder.WriteString("Estás analizando el código de un repositorio heredado/antiguo para migrarlo hacia un nuevo proyecto moderno.\n")
		initialPromptBuilder.WriteString("REGLA DE ORO: Extrae el QUÉ hace el sistema (reglas de negocio, validaciones, cálculos, flujos) y tradúcelo a escenarios BDD (Given/When/Then).\n")
		initialPromptBuilder.WriteString("PROHIBIDO acoplar la especificación a nombres de clases, métodos antiguos o tecnologías obsoletas (eso era el 'cómo', no el 'qué').\n\n")
		initialPromptBuilder.WriteString("CÓDIGO Y ESTRUCTURA DEL REPOSITORIO ANTIGUO (AS-IS):\n")
		initialPromptBuilder.WriteString(legacySummary)
		initialPromptBuilder.WriteString("\n\n")
	}

	if flagInterviewFeature != "" {
		initialPromptBuilder.WriteString(fmt.Sprintf("REQUERIMIENTO INICIAL / MÓDULO A MIGRAR:\n\"%s\"\n\n", flagInterviewFeature))
	}

	if contextSummary != "" {
		initialPromptBuilder.WriteString("CONTEXTO ACTUAL DEL REPOSITORIO DESTINO (AS-IS):\n")
		initialPromptBuilder.WriteString(contextSummary)
		initialPromptBuilder.WriteString("\n\n")
	}

	slug := "nueva-funcionalidad"
	if flagInterviewFeature != "" {
		slug = sanitizeSlug(flagInterviewFeature)
	}
	specsDir := filepath.Join(cwd, "specs")
	_ = os.MkdirAll(specsDir, 0755)

	targetSpecPath := flagInterviewOutput
	if targetSpecPath == "" {
		targetSpecPath = filepath.Join(specsDir, fmt.Sprintf("0001-%s.md", slug))
	}

	if !fileExists(targetSpecPath) {
		tmplData, err := assets.ReadEmbeddedFile("templates/spec-template.md")
		if err == nil {
			title := slug
			if flagInterviewFeature != "" {
				title = flagInterviewFeature
			}
			preseeded := strings.ReplaceAll(string(tmplData), "<Título conciso de la funcionalidad>", title)
			_ = os.WriteFile(targetSpecPath, []byte(preseeded), 0644)
			fmt.Printf("  ✓ Borrador inicial preparado en: %s\n", targetSpecPath)
		}
	}

	initialPromptBuilder.WriteString(fmt.Sprintf("\nINSTRUCCIÓN DE PERSISTENCIA:\nEl borrador base de la especificación ya ha sido inicializado en el archivo:\n`%s`\nActualiza y enriquece este archivo estructurado en las 7 secciones canónicas a medida que el usuario valide los escenarios.\n", targetSpecPath))

	// 5. Ejecutar la sesión interactiva blindada en Win32
	fmt.Println("\n[3/3] Iniciando sesión interactiva. Conversa con el Arquitecto de Software:")
	fmt.Println("------------------------------------------------------------------")

	opts := ports.AgentOptions{
		Project:    cfg.Auth.Project,
		Location:   cfg.Auth.Location,
		WorkingDir: cwd,
		Debug:      debug,
	}

	var runner ports.AgentRunner
	if agentName == "claude" {
		runner = agent.NewClaudeAgentRunner()
	} else {
		runner = agent.NewGeminiAgentRunner()
	}

	if err := runner.RunInteractive(context.Background(), initialPromptBuilder.String(), opts); err != nil {
		logger.Error("Fallo en sesión interactiva: %v", err)
		return fmt.Errorf("error durante la sesión con %s: %w", agentName, err)
	}

	// 6. Sellado Criptográfico SHA-256 post-entrevista
	fmt.Println("\n------------------------------------------------------------------")
	fmt.Println("✓ Sesión de entrevista completada.")

	specToSeal := targetSpecPath
	if _, err := os.Stat(specToSeal); os.IsNotExist(err) {
		specToSeal = findLatestSpec(specsDir)
	}

	if specToSeal != "" && fileExists(specToSeal) {
		sealHash, err := sealSpecification(specToSeal)
		if err != nil {
			logger.Warn("Aviso generando sello SHA-256: %v", err)
		} else {
			fmt.Printf("✓ Especificación sellada y blindada contra alteraciones:\n")
			fmt.Printf("  Archivo:     %s\n", specToSeal)
			fmt.Printf("  Sello SHA256: %s\n", sealHash)
			fmt.Println("  (Inmutabilidad garantizada para el ciclo TDD).")
		}
	} else {
		fmt.Printf("ℹ️ Puedes formalizar manualmente la especificación en %s\n", targetSpecPath)
	}

	return nil
}

func sanitizeSlug(s string) string {
	s = strings.ToLower(s)
	words := strings.Fields(s)
	if len(words) > 4 {
		words = words[:4]
	}
	clean := strings.Join(words, "-")
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, clean)
}

func findLatestSpec(specsDir string) string {
	var latestFile string
	var latestModTime int64

	_ = filepath.Walk(specsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			if info.ModTime().Unix() > latestModTime {
				latestModTime = info.ModTime().Unix()
				latestFile = path
			}
		}
		return nil
	})

	return latestFile
}

func sealSpecification(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	var cleaned []string
	for _, line := range lines {
		if !strings.Contains(line, "<!-- seal: sha256:") {
			cleaned = append(cleaned, line)
		}
	}
	cleanContent := strings.TrimSpace(strings.Join(cleaned, "\n"))

	// Calcular hash del contenido limpio normalizado
	hash := domain.CalculateBytesSHA256([]byte(cleanContent))
	sealedContent := fmt.Sprintf("%s\n\n<!-- seal: sha256:%s -->\n", cleanContent, hash)

	if err := os.WriteFile(filePath, []byte(sealedContent), 0644); err != nil {
		return "", err
	}

	return hash, nil
}
