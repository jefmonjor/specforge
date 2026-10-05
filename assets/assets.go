// Package assets embeds everything SpecForge writes or sends to an agent:
// the managed-block rules and per-stack standards, the prompt templates,
// the security audit method, the review contracts, the specification
// template and the project scaffolds.
package assets

import "embed"

// FS holds the embedded files, one directory per kind and, where the text
// is shown to people, one subdirectory per language.
//
//go:embed all:agent all:standards all:prompts all:audit all:review all:templates all:scaffolds
var FS embed.FS
