// Command specforge drives AI coding agents through a verified,
// specification-first Red → Green → Refactor loop.
package main

import (
	"os"

	"specforge/cmd"
)

func main() {
	initConsole()
	os.Exit(cmd.Main())
}
