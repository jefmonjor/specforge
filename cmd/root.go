package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"specforge/internal/adapters/storage"
	"specforge/internal/buildinfo"
	"specforge/internal/domain"
)

var (
	cfgFile string
	debug   bool
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:     "specforge",
	Aliases: []string{"sdd", "forge"},
	Short:   "SpecForge — Resilient Spec-Driven & TDD Assembly Line",
	Version: buildinfo.Version,
	Long: `==================================================================
🚀 SpecForge (Open Source Engine)
   Resilient Spec-Driven Development & TDD Assembly Line
==================================================================

RECOMMENDED WORKFLOW:
  1. specforge init         Zero-Keys setup (GCP ADC, AI agent, local profile)
  2. specforge setup        Deploy baseline and scaffolding (--stack react/java/go)
  3. specforge ingest       Ultra-fast As-Is project architecture scanner (~20 ms)
  4. specforge doc          Convert PDF/Word/Excel documents to Markdown
  5. specforge interview    Socratic BDD interview and SHA-256 spec sealing (--from-repo)
  6. specforge loop         Resilient TDD assembly line (Red -> Green -> Refactor) with --resume
  7. specforge audit        Adversarial security audit (Cloudflare Harness with --diff)
  8. specforge consistency  Deterministic architectural consistency verification`,
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger, err := storage.InitGlobalLogger(debug, verbose)
		if err == nil {
			logger.Debug("Command started: %s %v", cmd.Name(), args)
		}
	},
}

// Execute inicia la ejecución del árbol de comandos de Cobra
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		diag := domain.DiagnoseError(err)
		domain.PrintDiagnosticBox(diag)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "archivo de configuración (por defecto ~/.specforge/config.json)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "habilita logging de depuración en ~/.specforge/logs/")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "salida detallada por consola")
}
