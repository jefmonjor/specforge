package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"specforge/internal/adapters/agent"
	"specforge/internal/adapters/compiler"
	"specforge/internal/adapters/quality"
	"specforge/internal/adapters/storage"
	"specforge/internal/domain"
	"specforge/internal/ports"
)

var (
	flagLoopResume bool
	flagLoopSpec   string
	flagLoopAgent  string
)

var loopCmd = &cobra.Command{
	Use:   "loop",
	Short: "Ejecuta el ciclo TDD resiliente (Red -> Green -> Refactor) con tolerancia a fallos",
	Long: `Orquesta la línea de ensamblaje de artesanía de software:
1. RED: Genera stubs de test y verifica que fallen (abortando si hay violación YAGNI).
2. GREEN: Solicita a la IA la implementación mínima estricta para poner los tests en verde.
3. REFACTOR: Audita Clean Architecture y SRP con Quality Gates (jscpd, Knip, Stryker, Linters).
Cada transición se persiste en .sdd-state.json para reanudación instantánea con --resume.`,
	RunE: runLoop,
}

func init() {
	loopCmd.Flags().BoolVar(&flagLoopResume, "resume", false, "reanudar el ciclo exactamente en el estado persistido en .sdd-state.json")
	loopCmd.Flags().StringVar(&flagLoopSpec, "spec", "", "ruta a la especificación BDD sellada")
	loopCmd.Flags().StringVar(&flagLoopAgent, "agent", "", "agente de desarrollo ('gemini' o 'claude')")

	rootCmd.AddCommand(loopCmd)
}

func runLoop(cmd *cobra.Command, args []string) error {
	logger := storage.GetLogger()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error obteniendo directorio actual: %w", err)
	}

	// 1. Cargar configuración de SpecForge
	store := storage.NewConfigStorage(cfgFile)
	cfg, err := store.Load()
	if err != nil {
		cfg = domain.NewDefaultConfig()
	}

	agentName := cfg.Agent
	if flagLoopAgent != "" {
		agentName = strings.ToLower(flagLoopAgent)
	}

	fmt.Println("==================================================================")
	fmt.Println("🚀 SpecForge v3.0 — TDD Loop Resiliente (Red -> Green -> Refactor)")
	fmt.Println("==================================================================")

	proj := domain.DetectProject(cwd)
	fmt.Printf("  ✓ Stack detectado:    %s\n", proj.Type)
	fmt.Printf("  ✓ Agente activo:      %s\n", agentName)

	var state *domain.TDDState

	// 2. Inicialización o Reanudación con --resume
	if flagLoopResume {
		fmt.Println("\n[Modo Resiliencia: --resume]")
		loaded, err := domain.LoadTDDState(cwd)
		if err != nil {
			return fmt.Errorf("no se pudo cargar .sdd-state.json para reanudar: %w", err)
		}

		// Regla de Integridad Criptográfica: Verificar que spec.md no ha sido alterado manualmente
		if err := domain.VerifySpecIntegrity(loaded.SpecPath, loaded.SpecHash); err != nil {
			logger.Error("Violación de integridad: %v", err)
			return fmt.Errorf("FATAL: %w", err)
		}

		state = loaded
		fmt.Printf("  ✓ Integridad de especificación validada (%s)\n", state.SpecHash[:12])
		fmt.Printf("  ✓ Reanudando en escenario %d/%d (Fase: %s, Reintentos: %d)\n",
			state.CurrentScenario+1, len(state.Scenarios), state.CurrentPhase, state.RetryCount)
	} else {
		// Localizar especificación BDD
		specPath := flagLoopSpec
		if specPath == "" {
			specsDir := filepath.Join(cwd, "specs")
			specPath = findLatestSpec(specsDir)
			if specPath == "" {
				specPath = filepath.Join(cwd, "spec.md")
			}
		}

		if !fileExists(specPath) {
			return fmt.Errorf("no se encontró ninguna especificación BDD en %s. Ejecuta 'specforge interview' primero", specPath)
		}

		specData, err := os.ReadFile(specPath)
		if err != nil {
			return fmt.Errorf("error leyendo especificación %s: %w", specPath, err)
		}

		specContent := string(specData)
		specHash, err := domain.ExtractSpecSeal(specContent)
		if err != nil {
			return fmt.Errorf("la especificación no tiene un sello criptográfico válido: %w. Ejecuta 'specforge interview' o sella el archivo", err)
		}

		// Validar que el sello coincide con el contenido
		cleanHash := domain.CalculateCleanSpecHash(specContent)
		if cleanHash != specHash {
			return fmt.Errorf("el sello criptográfico no coincide con el contenido de la especificación (esperado: %s, actual: %s)", specHash, cleanHash)
		}

		// Regla Inquebrantable de Negocio: Bloqueo duro ante cualquier [NEEDS CLARIFICATION]
		if items := domain.ExtractNeedsClarification(specContent); len(items) > 0 {
			fmt.Println("\n==================================================================")
			fmt.Println("🛑 BLOQUEO DURO: [NEEDS CLARIFICATION] DETECTADO EN LA ESPECIFICACIÓN")
			fmt.Println("==================================================================")
			fmt.Printf("  • Causa:            Se encontraron %d cuestión(es) abierta(s) sin resolver.\n", len(items))
			fmt.Println("  • Regla del SDD:    La IA tiene estrictamente prohibido programar o generar tests")
			fmt.Println("                      mientras quede una sola duda funcional sin aclarar por Negocio.")
			fmt.Println("  • Cuestiones pendientes:")
			for _, item := range items {
				fmt.Printf("    - %s\n", item)
			}
			fmt.Println("\n  • Acción Inmediata: Aclara estas dudas con el Product Owner y elimínalas")
			fmt.Println("                      de la especificación antes de iniciar el ciclo TDD.")
			fmt.Printf("  • Archivo:          %s\n", specPath)
			fmt.Println("==================================================================")
			logger.Error("Ejecución abortada: %d [NEEDS CLARIFICATION] pendientes en %s", len(items), specPath)
			return fmt.Errorf("especificación bloqueada: contiene %d cuestión(es) [NEEDS CLARIFICATION] sin resolver", len(items))
		}

		scenarios, err := domain.ParseScenarios(specContent)
		if err != nil || len(scenarios) == 0 {
			return fmt.Errorf("no se pudieron parsear escenarios BDD de %s: %v", specPath, err)
		}

		state = domain.NewTDDState(cwd, specPath, specHash, scenarios)
		_ = state.Save(cwd)

		fmt.Printf("  ✓ Especificación BDD verificada: %s (Hash: %s)\n", filepath.Base(specPath), specHash[:12])
		fmt.Printf("  ✓ Escenarios BDD cargados:    %d\n", len(scenarios))
		fmt.Println("  ✓ Estado inicial guardado en .sdd-state.json")
	}

	// 3. Inicializar adaptadores
	var runner ports.AgentRunner
	if agentName == "claude" {
		runner = agent.NewClaudeAgentRunner()
	} else {
		runner = agent.NewGeminiAgentRunner()
	}

	agentOpts := ports.AgentOptions{
		Project:    cfg.Auth.Project,
		Location:   cfg.Auth.Location,
		WorkingDir: cwd,
		Debug:      debug,
	}

	comp := compiler.NewDispatcherCompiler()
	qualityGate := quality.NewCompositeQualityGate(proj)

	// 4. Bucle TDD Infinito Resiliente
	for state.CurrentPhase != domain.TDDPhaseCompleted {
		currentSc := state.CurrentScenarioData()
		if currentSc == nil {
			state.CurrentPhase = domain.TDDPhaseCompleted
			_ = state.Save(cwd)
			break
		}

		switch state.CurrentPhase {

		case domain.TDDPhaseRed:
			fmt.Printf("\n------------------------------------------------------------------\n")
			fmt.Printf("🔴 [FASE RED] Escenario %d/%d: \"%s\"\n", state.CurrentScenario+1, len(state.Scenarios), currentSc.Title)
			fmt.Println("------------------------------------------------------------------")
			logger.Info("[RED] Escenario %d: %s", state.CurrentScenario+1, currentSc.Title)

			// 1. Pedir a la IA que genere el stub de prueba
			fmt.Println("  [1/2] Generando stubs de prueba unitaria...")
			if err := comp.GenerateTestStubs(context.Background(), proj, *currentSc, runner, agentOpts); err != nil {
				state.LastError = err.Error()
				_ = state.Save(cwd)
				return fmt.Errorf("error generando test stub: %w", err)
			}

			// 2. Ejecutar tests con buffers aislados
			fmt.Println("  [2/2] Validando fallo del test (Gate YAGNI)...")
			passed, output, _ := comp.RunTests(context.Background(), proj, currentSc.Title)

			if passed {
				// El test pasó sin implementación: VIOLACIÓN YAGNI
				state.RecordStep("red_validation", "FAILED", "YAGNI Violation: el test pasa antes de escribir la implementación")
				_ = state.Save(cwd)
				logger.Error("YAGNI Violation detectada en escenario '%s'. Salida: %s", currentSc.Title, output)
				return fmt.Errorf("❌ YAGNI Violation: El test pasa antes de escribir la implementación. Revisa el código o la prueba")
			}

			fmt.Println("  ✓ Test en rojo verificado correctamente (falló según lo esperado).")
			state.AdvanceToNextPhase() // Avanza a GREEN
			state.RecordStep("red_test_failing", "SUCCESS", "Test en rojo confirmado")
			_ = state.Save(cwd)

		case domain.TDDPhaseGreen:
			fmt.Printf("\n------------------------------------------------------------------\n")
			fmt.Printf("🟢 [FASE GREEN] Implementación Estricta (Intento %d/3)\n", state.RetryCount+1)
			fmt.Println("------------------------------------------------------------------")
			logger.Info("[GREEN] Implementando código para escenario %d (intento %d)", state.CurrentScenario+1, state.RetryCount+1)

			// 1. Enviar error del test a la IA para implementación mínima
			fmt.Println("  [1/2] Solicitando implementación mínima a la IA...")
			prompt := buildGreenPrompt(proj, *currentSc, state.LastError)
			if _, err := runner.RunHeadless(context.Background(), prompt, agentOpts); err != nil {
				state.LastError = err.Error()
				_ = state.Save(cwd)
				return fmt.Errorf("error en invocación de IA para GREEN: %w", err)
			}

			// 2. Ejecutar tests
			fmt.Println("  [2/2] Ejecutando suite de pruebas...")
			passed, output, _ := comp.RunTests(context.Background(), proj, currentSc.Title)

			if passed {
				fmt.Println("  ✓ ¡Test en VERDE! La implementación mínima cumple los requisitos.")
				if state.RetryCount > 0 {
					_ = domain.RecordLesson(cwd, "Compilación / Test", state.LastError, "Implementación mínima y corrección de tipos/sintaxis")
					fmt.Println("  🧠 [Auto-Aprendizaje] Lección aprendida registrada en .sdd/agent/lessons.md")
				}
				state.AdvanceToNextPhase() // Avanza a REFACTOR
				state.RecordStep("green_test_passing", "SUCCESS", "Test en verde confirmado")
				_ = state.Save(cwd)
			} else {
				state.RetryCount++
				state.LastError = output
				_ = state.Save(cwd)
				logger.Warn("Test falló en fase GREEN. Intento %d/3", state.RetryCount)

				if state.RetryCount >= 3 {
					return fmt.Errorf("⚠️ Límite de 3 reintentos alcanzado en GREEN. Estado guardado en .sdd-state.json. Revisa el error y ejecuta 'specforge loop --resume'")
				}
				fmt.Printf("  ⚠️ El test no pasó aún. Reintentando con feedback de error (%d/3)...\n", state.RetryCount)
			}

		case domain.TDDPhaseRefactor:
			fmt.Printf("\n------------------------------------------------------------------\n")
			fmt.Printf("🔵 [FASE REFACTOR] Auditoría de Clean Architecture y Guardarraíles de Calidad\n")
			fmt.Println("------------------------------------------------------------------")
			logger.Info("[REFACTOR] Ejecutando Quality Gates para escenario %d", state.CurrentScenario+1)

			fmt.Println("  [1/2] Ejecutando guardarraíles (Linters, DRY/jscpd, Knip, Stryker)...")
			passed, report, err := qualityGate.RunStaticAnalysis(context.Background(), proj)
			if err != nil {
				logger.Warn("Error durante análisis estático: %v", err)
			}

			if !passed {
				fmt.Printf("  ❌ Violación de guardarraíles detectada:\n%s\n", report)
				fmt.Println("  Solicitando a la IA refactorización de calidad sin romper tests...")

				state.LastError = report
				_ = state.Save(cwd)

				refactorPrompt := fmt.Sprintf("El código ha pasado los tests funcionales, pero ha fallado los guardarraíles de calidad. Corrige las violaciones descritas a continuación sin romper los tests existentes:\n\n%s", report)
				_, _ = runner.RunHeadless(context.Background(), refactorPrompt, agentOpts)

				// Re-evaluar tras corrección
				passedAfter, reportAfter, _ := qualityGate.RunStaticAnalysis(context.Background(), proj)
				if !passedAfter {
					state.LastError = reportAfter
					_ = state.Save(cwd)
					logger.Warn("Guardarraíles no superados tras refactor: %s", reportAfter)
					return fmt.Errorf("⚠️ Quality Gate no superado en REFACTOR. Estado guardado en .sdd-state.json. Revisa los hallazgos y ejecuta 'specforge loop --resume'")
				}
				fmt.Println("  ✓ Violaciones de calidad corregidas exitosamente.")
				_ = domain.RecordLesson(cwd, "Quality Gate", report, "Refactorización limpia y eliminación de violaciones de guardarraíles")
				fmt.Println("  🧠 [Auto-Aprendizaje] Regla de calidad aprendida y registrada en .sdd/agent/lessons.md")
			} else {
				fmt.Printf("  ✓ Guardarraíles aprobados:\n%s\n", report)
			}

			fmt.Println("  ✓ Fase REFACTOR completada. Escenario consolidado.")
			state.RecordStep("refactor_quality_gate", "SUCCESS", "Quality Gates aprobados")
			state.AdvanceToNextPhase() // Avanza al siguiente escenario (RED) o COMPLETED
			_ = state.Save(cwd)
		}
	}

	// 5. Cierre exitoso del ciclo completo
	fmt.Println("\n==================================================================")
	fmt.Println("🎉 CICLO TDD RESILIENTE COMPLETADO CON ÉXITO")
	fmt.Println("==================================================================")
	fmt.Printf("  ✓ Todos los escenarios (%d) han sido implementados y auditados.\n", len(state.Scenarios))
	fmt.Println("  ✓ Código en VERDE cumpliendo Clean Architecture y principios SOLID.")
	fmt.Println("  ✓ Estado final guardado en .sdd-state.json.")

	return nil
}

func buildGreenPrompt(project domain.ProjectInfo, scenario domain.Scenario, lastError string) string {
	var sb strings.Builder
	sb.WriteString("MISIÓN: TDD FASE GREEN (IMPLEMENTACIÓN ESTRICTA YAGNI)\n\n")
	sb.WriteString(fmt.Sprintf("Stack del proyecto: %s\n", project.Type))
	sb.WriteString("Objetivo: Implementa EXCLUSIVAMENTE el código productivo necesario para que el test pase.\n")
	sb.WriteString("Aplica KISS y YAGNI estricto: no agregues funcionalidades extras ni sobre-ingeniería.\n\n")

	sb.WriteString(fmt.Sprintf("ESCENARIO BDD:\n%s\n\n", scenario.RawContent))

	if lastError != "" {
		sb.WriteString("ERROR PREVIO DEL TEST QUE DEBE RESOLVERSE:\n```\n")
		errSnippet := lastError
		if len(errSnippet) > 2000 {
			errSnippet = errSnippet[:2000] + "\n...[error truncado]"
		}
		sb.WriteString(errSnippet)
		sb.WriteString("\n```\n")
	}

	return sb.String()
}
