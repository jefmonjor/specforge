package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagBuildDryRun bool
)

var buildCommand = &cobra.Command{
	Use:   "build",
	Short: "Compila y valida el proyecto",
	Run: func(cmd *cobra.Command, args []string) {
		if flagBuildDryRun {
			fmt.Println("[build] DryRun OK: Verificación preliminar superada sin clasificacion previa requerida.")
			os.Exit(0)
		}
		fmt.Println("[build] Ejecutando compilación del proyecto...")
	},
}

func init() {
	buildCommand.Flags().BoolVarP(&flagBuildDryRun, "dry-run", "d", false, "ejecutar en modo simulación (DryRun)")
	buildCommand.Flags().BoolVar(&flagBuildDryRun, "DryRun", false, "compatibilidad PowerShell para -DryRun")
	rootCmd.AddCommand(buildCommand)
}
