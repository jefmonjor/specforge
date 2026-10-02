package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"specforge/internal/adapters/e2e"
	"specforge/internal/adapters/storage"
	"specforge/internal/domain"
	"specforge/internal/ports"
)

var (
	flagE2EURL        string
	flagE2ESpec       string
	flagE2EHeadless   bool
	flagE2EMaxSteps   int
	flagE2EScreenshot string
	flagE2EAgent      string
)

var e2eCmd = &cobra.Command{
	Use:   "e2e",
	Short: "Ejecuta pruebas E2E visuales y autónomas (estilo TesterArmy) con chromedp",
	Long: `Motor de pruebas de extremo a extremo (E2E) autónomo en Go puro utilizando chromedp.
El agente de IA lee la especificación BDD, inspecciona el árbol simplificado de la UI
en tiempo real y navega e interactúa con la aplicación web (click, type, assert)
sin depender de Node.js, Puppeteer, Playwright ni APIs de terceros.`,
	RunE: runE2E,
}

func init() {
	e2eCmd.Flags().StringVar(&flagE2EURL, "url", "", "URL objetivo de la aplicación web (requerido, ej. http://localhost:3000)")
	e2eCmd.Flags().StringVar(&flagE2ESpec, "spec", "", "ruta a la especificación BDD a validar (por defecto la última en specs/)")
	e2eCmd.Flags().BoolVar(&flagE2EHeadless, "headless", true, "ejecutar el navegador en segundo plano (headless)")
	e2eCmd.Flags().IntVar(&flagE2EMaxSteps, "max-steps", 15, "límite máximo de acciones autónomas")
	e2eCmd.Flags().StringVar(&flagE2EScreenshot, "screenshot", "docs/e2e/screenshot.png", "ruta donde guardar la captura en caso de fallo")
	e2eCmd.Flags().StringVar(&flagE2EAgent, "agent", "", "agente de IA a utilizar ('gemini' o 'claude')")

	_ = e2eCmd.MarkFlagRequired("url")
	rootCmd.AddCommand(e2eCmd)
}

func runE2E(cmd *cobra.Command, args []string) error {
	logger := storage.GetLogger()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error obteniendo directorio actual: %w", err)
	}

	if flagE2EURL == "" {
		return fmt.Errorf("debes especificar la URL objetivo con --url <URL>")
	}

	// 1. Cargar configuración
	store := storage.NewConfigStorage(cfgFile)
	cfg, err := store.Load()
	if err != nil {
		cfg = domain.NewDefaultConfig()
	}

	if flagE2EAgent != "" {
		cfg.Agent = strings.ToLower(flagE2EAgent)
	}

	// 2. Localizar y cargar la especificación BDD
	specPath := flagE2ESpec
	if specPath == "" {
		specPath = findLatestSpec(filepath.Join(cwd, "specs"))
	}

	var spec *domain.Spec
	if specPath != "" && fileExists(specPath) {
		data, err := os.ReadFile(specPath)
		if err == nil {
			spec = &domain.Spec{
				FilePath: specPath,
				Content:  string(data),
			}
			fmt.Printf("✓ Especificación BDD cargada desde: %s\n", specPath)
		}
	} else {
		fmt.Println("ℹ️ No se encontró especificación BDD en specs/. Se ejecutará verificación exploratoria general.")
		spec = &domain.Spec{
			Content: "Verificar que la aplicación web responde, los elementos principales son visibles y no hay errores de consola.",
		}
	}

	opts := ports.E2EOptions{
		Headless:       flagE2EHeadless,
		MaxSteps:       flagE2EMaxSteps,
		ScreenshotPath: flagE2EScreenshot,
		Agent:          cfg.Agent,
	}

	// 3. Arrancar el motor E2E con el agente visual
	engine := e2e.NewVisionAgent(cfg)
	if err := engine.RunVisualSpec(context.Background(), spec, flagE2EURL, opts); err != nil {
		logger.Error("Prueba E2E fallida: %v", err)
		return err
	}

	logger.Info("Prueba E2E superada exitosamente en %s", flagE2EURL)
	return nil
}
