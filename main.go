package main

import (
	"os"
	"strings"

	"specforge/cmd"
)

func main() {
	initConsole()

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && len(arg) > 2 {
			os.Args[i] = "-" + arg
		}
	}

	cmd.Execute()
}
