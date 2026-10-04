package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"specforge/internal/adapters/auth"
	"specforge/internal/adapters/storage"
	"specforge/internal/adapters/system"
	"specforge/internal/domain"
)

var (
	flagAgent          string
	flagProject        string
	flagLocation       string
	flagNexusURL       string
	flagNonInteractive bool
	flagForce          bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inicializa la configuración de SpecForge con Zero-Config y detección ADC",
	Long: `Inicia el asistente interactivo para configurar el SpecForge.
Detecta automáticamente las credenciales de Google Cloud (ADC) de gcloud,
el proyecto de trabajo, y genera el archivo ~/.specforge/config.json sin necesidad
de configurar manualmente variables de entorno del sistema operativo.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&flagAgent, "agent", "", "agente de IA a utilizar ('gemini' o 'claude')")
	initCmd.Flags().StringVar(&flagProject, "project", "", "ID de proyecto de Google Cloud")
	initCmd.Flags().StringVar(&flagLocation, "location", "europe-west1", "región de Google Cloud")
	initCmd.Flags().StringVar(&flagNexusURL, "nexus-url", "", "URL base de Nexus para distribución de baseline")
	initCmd.Flags().BoolVar(&flagNonInteractive, "non-interactive", false, "ejecutar en modo desatendido usando defaults y flags")
	initCmd.Flags().BoolVar(&flagForce, "force", false, "sobrescribir configuración existente sin preguntar")

	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	logger := storage.GetLogger()
	store := storage.NewConfigStorage(cfgFile)

	fmt.Println("==================================================================")
	fmt.Println("🚀 SpecForge — Asistente de Configuración Zero-Config")
	fmt.Println("==================================================================")

	if store.Exists() && !flagForce && !flagNonInteractive {
		fmt.Print("Ya existe una configuración previa en ~/.specforge/config.json. ¿Deseas reconfigurar? [s/N]: ")
		reader := bufio.NewReader(os.Stdin)
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(strings.ToLower(resp))
		if resp != "s" && resp != "si" && resp != "y" && resp != "yes" {
			fmt.Println("Operación cancelada. Configuración existente preservada.")
			return nil
		}
	}

	cfg := domain.NewDefaultConfig()

	// 1. Detección automática de Google Cloud ADC (Zero-Keys)
	fmt.Println("\n[1/3] Detectando credenciales corporativas (Zero-Keys)...")
	detector := auth.NewGCPADCDetector()
	detectedAuth, err := detector.DetectCredentials(context.Background())
	if err != nil {
		logger.Warn("Error detectando credenciales ADC: %v", err)
	}

	if detectedAuth != nil && detectedAuth.HasADC {
		cfg.Auth = *detectedAuth
		fmt.Printf("  ✓ Application Default Credentials (ADC) detectadas con éxito.\n")
		if detectedAuth.Account != "" {
			fmt.Printf("  ✓ Cuenta activa:   %s\n", detectedAuth.Account)
		}
		if detectedAuth.Project != "" {
			fmt.Printf("  ✓ Proyecto GCP:    %s\n", detectedAuth.Project)
		}
		fmt.Printf("  ✓ Región VertexAI: %s\n", detectedAuth.Location)
	} else {
		fmt.Println("  ⚠️ No se detectaron Application Default Credentials (ADC).")
		fmt.Println("     Recomendación corporativa: ejecuta 'gcloud auth application-default login'")
	}

	reader := bufio.NewReader(os.Stdin)

	// 2. Selección de Agente de IA
	if flagAgent != "" {
		cfg.Agent = strings.ToLower(flagAgent)
	} else if !flagNonInteractive {
		fmt.Println("\n[2/3] Selección de Agente de Desarrollo:")
		fmt.Println("  1) Gemini CLI (Recomendado / Nativo Vertex AI)")
		fmt.Println("  2) Claude Code")
		fmt.Print("Selecciona una opción [1]: ")
		opt, _ := reader.ReadString('\n')
		opt = strings.TrimSpace(opt)
		if opt == "2" {
			cfg.Agent = "claude"
		} else {
			cfg.Agent = "gemini"
		}
	}
	fmt.Printf("  → Agente configurado: %s\n", cfg.Agent)

	// Sobrescribir proyecto GCP si se pasó por flag o wizard
	if flagProject != "" {
		cfg.Auth.Project = flagProject
	} else if !flagNonInteractive && (cfg.Auth.Project == "" || cfg.Auth.Project == "default-project") {
		fmt.Printf("\nProyecto de Google Cloud [%s]: ", cfg.Auth.Project)
		projInput, _ := reader.ReadString('\n')
		projInput = strings.TrimSpace(projInput)
		if projInput != "" {
			cfg.Auth.Project = projInput
		}
	}

	if flagLocation != "" {
		cfg.Auth.Location = flagLocation
	}

	// 3. Origen del Baseline (Embebido o Remoto)
	if flagNexusURL != "" {
		cfg.Repository.Mode = "remote"
		cfg.Repository.NexusURL = flagNexusURL
		cfg.Repository.AIFrameworkURL = flagNexusURL
	} else if !flagNonInteractive {
		fmt.Println("\n[3/3] Distribución del Baseline:")
		fmt.Println("  1) Embebido Nativo (Recomendado - Zero Dependencies)")
		fmt.Println("  2) Servidor Remoto / Artifact Repository")
		fmt.Print("Selecciona una opción [1]: ")
		opt, _ := reader.ReadString('\n')
		opt = strings.TrimSpace(opt)
		if opt == "2" {
			cfg.Repository.Mode = "remote"
			defaultURL := "https://artifacts.example.com/repository/sdd-raw"
			fmt.Printf("URL del repositorio [%s]: ", defaultURL)
			urlInput, _ := reader.ReadString('\n')
			urlInput = strings.TrimSpace(urlInput)
			if urlInput != "" {
				cfg.Repository.NexusURL = urlInput
				cfg.Repository.AIFrameworkURL = urlInput
			} else {
				cfg.Repository.NexusURL = defaultURL
				cfg.Repository.AIFrameworkURL = defaultURL
			}
		} else {
			cfg.Repository.Mode = "embedded"
		}
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validación de configuración fallida: %w", err)
	}

	// Guardar configuración en ~/.specforge/config.json
	cfg.PopulateEnvMap()
	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("error guardando ~/.specforge/config.json: %w", err)
	}

	configPath, _ := store.GetConfigPath()
	fmt.Println("\n==================================================================")
	fmt.Printf("✓ Configuración generada exitosamente en:\n  %s\n", configPath)
	fmt.Println("==================================================================")
	fmt.Println("Variables aisladas en el perfil:")
	for k, v := range cfg.Environment {
		fmt.Printf("  • %s=%s\n", k, v)
	}

	// 4. Integración automática en el PATH del sistema
	fmt.Println("\n[4/4] Verificando integración en el PATH del sistema...")
	if added, msg, err := system.EnsureExeInUserPath(); err == nil {
		if added {
			fmt.Println("  " + msg)
			logger.Info("sdd añadido al PATH del usuario: %s", msg)
		} else {
			fmt.Println("  ✓ El comando 'sdd' ya se encuentra accesible en el PATH.")
		}
	} else {
		logger.Warn("Aviso comprobando PATH: %v", err)
	}

	fmt.Println("\nListo para trabajar. El puente híbrido y los comandos sdd usarán esta configuración.")
	logger.Info("sdd init completado con éxito. Guardado en %s", configPath)

	return nil
}
