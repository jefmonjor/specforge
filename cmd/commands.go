package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays version and environment telemetry",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("==================================================================")
		fmt.Println("🚀 SpecForge v3.0.0 (Open Source Engine)")
		fmt.Println("==================================================================")
		fmt.Printf("  • Architecture:      Clean Architecture (Pure Static Go)\n")
		fmt.Printf("  • Platform:          %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("  • Go Version:        %s\n", runtime.Version())
		fmt.Printf("  • License:           Apache 2.0 (Zero-Dependencies)\n")
		fmt.Printf("  • Guardrails:        Strict Linters, jscpd (DRY), Knip, Stryker\n")
		fmt.Printf("  • Security Engine:   Cloudflare Adversarial Harness (6 Phases)\n")
		fmt.Println("==================================================================")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
