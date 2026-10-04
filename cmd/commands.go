package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"specforge/internal/buildinfo"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays version and platform information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("SpecForge %s\n", buildinfo.String())
		fmt.Printf("  Platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("  Go runtime: %s\n", runtime.Version())
		fmt.Printf("  License:    Apache-2.0\n")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
