// Command specforge drives AI coding agents through a verified,
// specification-first Red → Green → Refactor loop.
package main

import (
	"os"

	"github.com/jefmonjor/specforge/v6/cmd"
)

func main() {
	initConsole()
	os.Exit(cmd.Main())
}
