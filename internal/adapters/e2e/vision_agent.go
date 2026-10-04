package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"specforge/internal/adapters/agent"
	"specforge/internal/adapters/storage"
	"specforge/internal/domain"
	"specforge/internal/ports"
)

type VisionAgent struct {
	cfg *domain.SDDConfig
}

func NewVisionAgent(cfg *domain.SDDConfig) ports.E2EEngine {
	return &VisionAgent{cfg: cfg}
}

func (v *VisionAgent) RunVisualSpec(ctx context.Context, spec *domain.Spec, targetURL string, opts ports.E2EOptions) error {
	logger := storage.GetLogger()

	if opts.MaxSteps <= 0 {
		opts.MaxSteps = 15
	}
	if opts.ScreenshotPath == "" {
		opts.ScreenshotPath = filepath.Join("docs", "e2e", "screenshot.png")
	}

	agentName := v.cfg.Agent
	if opts.Agent != "" {
		agentName = opts.Agent
	}

	var runner ports.AgentRunner
	if agentName == "claude" {
		runner = agent.NewClaudeAgentRunner()
	} else {
		runner = agent.NewGeminiAgentRunner()
	}

	fmt.Println("==================================================================")
	fmt.Println("🧪 SpecForge — Motor E2E Autónomo (TesterArmy Engine)")
	fmt.Println("==================================================================")
	fmt.Printf("  • URL Objetivo:     %s\n", targetURL)
	fmt.Printf("  • Agente Visual:    %s (Vertex AI ADC: %v)\n", agentName, v.cfg.Auth.UseVertexAI)
	fmt.Printf("  • Modo Navegador:   Headless: %v (Chrome/Edge nativo)\n", opts.Headless)
	fmt.Printf("  • Límite de Pasos:  %d pasos máximos\n", opts.MaxSteps)
	fmt.Printf("  • Captura en Fallo: %s\n", opts.ScreenshotPath)
	fmt.Println("==================================================================")

	// 1. Inicializar navegador con Chromedp
	driver, err := NewChromedpDriver(opts.Headless)
	if err != nil {
		return fmt.Errorf("error inicializando driver chromedp: %w", err)
	}
	defer driver.Close()

	// 2. Navegar a la URL inicial
	fmt.Printf("\n[Paso 0] Navegando a %s...\n", targetURL)
	if err := driver.Navigate(ctx, targetURL); err != nil {
		_ = driver.CaptureScreenshot(ctx, opts.ScreenshotPath)
		return fmt.Errorf("no se pudo cargar la URL %s: %w", targetURL, err)
	}
	fmt.Println("  ✓ Página cargada correctamente.")

	history := []string{fmt.Sprintf("Paso 0: Navegación inicial a %s", targetURL)}

	agentOpts := ports.AgentOptions{
		Project:  v.cfg.Auth.Project,
		Location: v.cfg.Auth.Location,
	}

	// 3. Action Loop autónomo (hasta éxito o maxSteps)
	for step := 1; step <= opts.MaxSteps; step++ {
		// Capturar snapshot del DOM visible
		snapshot, err := driver.CaptureUISnapshot(ctx)
		if err != nil {
			logger.Warn("Aviso capturando UI Snapshot en paso %d: %v", step, err)
		}

		// Construir prompt socrático para la IA
		prompt := buildVisionPrompt(spec, snapshot, history)

		// Consultar al agente de IA
		rawResponse, err := runner.RunHeadless(ctx, prompt, agentOpts)
		if err != nil {
			_ = driver.CaptureScreenshot(ctx, opts.ScreenshotPath)
			return fmt.Errorf("error en llamada al agente visual: %w", err)
		}

		// Parsear la acción decidida
		action, err := parseVisualAction(rawResponse)
		if err != nil {
			logger.Warn("Respuesta no parseable del agente: %s (error: %v)", rawResponse, err)
			action = &domain.VisualAction{
				ActionType:  domain.ActionWait,
				Explanation: "Pausa por respuesta ambigua",
			}
		}

		// Reportar paso en consola
		fmt.Printf("\n[Paso %02d/%d] Acción: %s\n", step, opts.MaxSteps, strings.ToUpper(string(action.ActionType)))
		if action.Explanation != "" {
			fmt.Printf("  • Razón:    %s\n", action.Explanation)
		}
		if action.TargetSelector != "" {
			fmt.Printf("  • Selector: %s\n", action.TargetSelector)
		}
		if action.Value != "" && action.ActionType == domain.ActionTypeInput {
			fmt.Printf("  • Valor:    \"%s\"\n", action.Value)
		}

		// Evaluar aserción terminal
		if action.ActionType == domain.ActionAssert {
			if action.IsSuccess {
				fmt.Println("\n==================================================================")
				fmt.Println("✓ PRUEBA E2E SUPERADA CON ÉXITO")
				fmt.Printf("  Aserción validada por el agente visual: %s\n", action.Explanation)
				fmt.Printf("  Pasos ejecutados: %d | Tiempo total: normal\n", step)
				fmt.Println("==================================================================")
				return nil
			}

			// Aserción negativa (Fallo del test)
			_ = driver.CaptureScreenshot(ctx, opts.ScreenshotPath)
			fmt.Println("\n==================================================================")
			fmt.Println("❌ FALLO EN PRUEBA E2E: El agente determinó que el assert no se cumple")
			fmt.Printf("  Causa:               %s\n", action.Explanation)
			fmt.Printf("  Captura de pantalla: %s\n", opts.ScreenshotPath)
			fmt.Println("==================================================================")
			return fmt.Errorf("aserción E2E fallida: %s", action.Explanation)
		}

		// Ejecutar la acción en el navegador
		if err := driver.ExecuteAction(ctx, *action); err != nil {
			errMsg := fmt.Sprintf("Fallo ejecutando acción %s en %s: %v", action.ActionType, action.TargetSelector, err)
			logger.Warn("%s", errMsg)
			history = append(history, fmt.Sprintf("Paso %d: Error -> %s", step, errMsg))
		} else {
			history = append(history, fmt.Sprintf("Paso %d: Ejecutado %s en %s (%s)", step, action.ActionType, action.TargetSelector, action.Explanation))
		}

		// Breve estabilización visual
		time.Sleep(500 * time.Millisecond)
	}

	// Límite de pasos alcanzado sin aserción final
	_ = driver.CaptureScreenshot(ctx, opts.ScreenshotPath)
	fmt.Println("\n==================================================================")
	fmt.Printf("❌ TIMEOUT E2E: Se alcanzó el límite de %d pasos sin verificar el escenario BDD.\n", opts.MaxSteps)
	fmt.Printf("  Captura de pantalla guardada en: %s\n", opts.ScreenshotPath)
	fmt.Println("==================================================================")

	return fmt.Errorf("límite de %d pasos alcanzado sin validar el escenario", opts.MaxSteps)
}

func buildVisionPrompt(spec *domain.Spec, snapshot *domain.UISnapshot, history []string) string {
	var sb strings.Builder

	sb.WriteString("INSTRUCCIÓN DEL SISTEMA:\n")
	sb.WriteString("Eres un QA Tester Autónomo (estilo TesterArmy). Tu misión es interactuar con la aplicación web para verificar el cumplimiento del escenario BDD especificado.\n")
	sb.WriteString("En cada paso recibes la URL actual, el título y los elementos interactivos visibles en la pantalla (UI Tree).\n")
	sb.WriteString("Debes responder ÚNICAMENTE con un JSON parseable (sin texto antes ni después) con esta estructura exacta:\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"action_type\": \"click\" | \"type\" | \"assert\" | \"wait\" | \"navigate\",\n")
	sb.WriteString("  \"target_selector\": \"<selector CSS exacto de la lista de elementos>\",\n")
	sb.WriteString("  \"value\": \"<texto a escribir o motivo del assert>\",\n")
	sb.WriteString("  \"explanation\": \"<motivo conciso del paso>\",\n")
	sb.WriteString("  \"is_success\": true | false\n")
	sb.WriteString("}\n\n")

	sb.WriteString("REGLAS ESTRICTAS:\n")
	sb.WriteString("1. Usa EXACTAMENTE los selectores CSS provistos en 'elements[].selector'.\n")
	sb.WriteString("2. Para rellenar inputs, usa action_type 'type' con el selector del input y el texto en 'value'.\n")
	sb.WriteString("3. Para botones/enlaces, usa action_type 'click'.\n")
	sb.WriteString("4. Si el escenario BDD ya se ha cumplido visualmente (ej. se muestra el mensaje de confirmación, la tabla esperada o el estado final), devuelve action_type: 'assert' con is_success: true.\n")
	sb.WriteString("5. Si detectas un error visible que contradiga el BDD o un bloqueo, devuelve action_type: 'assert' con is_success: false.\n")
	sb.WriteString("6. Responde EXCLUSIVAMENTE el JSON, sin bloques de código ```json ni comentarios.\n\n")

	sb.WriteString("--- ESPECIFICACIÓN BDD A VERIFICAR ---\n")
	if spec != nil && spec.Content != "" {
		sb.WriteString(spec.Content)
	} else {
		sb.WriteString("Verifica que la página principal cargue y los elementos interactivos funcionen correctamente.")
	}
	sb.WriteString("\n\n")

	sb.WriteString("--- HISTORIAL DE PASOS PREVIOS ---\n")
	for _, h := range history {
		sb.WriteString(fmt.Sprintf("• %s\n", h))
	}
	sb.WriteString("\n")

	sb.WriteString("--- ESTADO ACTUAL DE LA INTERFAZ DE USUARIO (UI TREE) ---\n")
	if snapshot != nil {
		sb.WriteString(fmt.Sprintf("URL:   %s\n", snapshot.URL))
		sb.WriteString(fmt.Sprintf("Title: %s\n", snapshot.Title))
		sb.WriteString("Elementos Interactivos Visibles:\n")

		// Serializar elementos en formato conciso
		for _, el := range snapshot.Elements {
			desc := el.Text
			if desc == "" {
				desc = el.Placeholder
			}
			if desc == "" {
				desc = el.AriaLabel
			}
			sb.WriteString(fmt.Sprintf("  - [%s] tag=%s type=%s text=\"%s\" selector=\"%s\"\n",
				el.ID, el.Tag, el.Type, desc, el.Selector))
		}
	} else {
		sb.WriteString("No se pudieron extraer elementos del DOM.")
	}

	sb.WriteString("\nDecide la siguiente acción en JSON:")
	return sb.String()
}

func parseVisualAction(raw string) (*domain.VisualAction, error) {
	clean := strings.TrimSpace(raw)
	if idx := strings.Index(clean, "{"); idx != -1 {
		clean = clean[idx:]
	}
	if idx := strings.LastIndex(clean, "}"); idx != -1 {
		clean = clean[:idx+1]
	}

	var action domain.VisualAction
	if err := json.Unmarshal([]byte(clean), &action); err != nil {
		return nil, err
	}

	return &action, nil
}
