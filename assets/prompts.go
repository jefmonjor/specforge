package assets

import "embed"

// PromptsFS holds the agent prompt templates, one directory per language.
//
//go:embed all:prompts
var PromptsFS embed.FS
