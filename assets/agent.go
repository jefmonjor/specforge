package assets

import "embed"

// AgentFS holds the rules SpecForge writes into CLAUDE.md / GEMINI.md and
// the per-stack standards, one directory per language.
//
//go:embed all:agent all:standards
var AgentFS embed.FS
