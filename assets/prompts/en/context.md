{{define "context"}}
{{- if and .Legacy (not .Inventory)}}
## Legacy code (read-only reference)
This is a rewrite: the behaviour comes from the legacy code at `{{.Legacy}}`, and the specification's legacy sources say where. Read those sources to reproduce the behaviour exactly, never change a file there, and do not carry over its technology{{if .JavaRelease}}: the new code targets Java {{.JavaRelease}} and uses its idioms (records, `java.time`, `List.of`, try-with-resources, `var` where it reads better){{end}}.{{if .ForbiddenImports}} SpecForge rejects any import of {{range $i, $p := .ForbiddenImports}}{{if $i}}, {{end}}`{{$p}}`{{end}}.{{end}}
{{- if .LegacySources}}

### Where this behaviour comes from
{{.LegacySources}}
{{- end}}
{{end}}
{{- if .Plan}}
## Approved technical plan (follow it; ask before departing from it)
{{.Plan}}
{{end}}
{{- if .Surfaces}}
## Allowed edit surfaces
Change only these files (a path ending in `/` allows everything under it). SpecForge checks every file you change; anything else goes to the developer, who may refuse it. If the task needs another file, ask.
{{range .Surfaces}}- `{{.}}`
{{end}}
{{- end}}
{{- if .Glossary}}
## Ubiquitous language (use these names in code)
{{.Glossary}}
{{end}}
{{- if .Invariants}}
## Domain invariants (must never be violated)
{{.Invariants}}
{{end}}
{{- if .Decisions}}
## Decisions already taken by the developer (do not ask again)
{{.Decisions}}
{{end}}
{{- if .Known}}
## Known failures on this branch (not yours to fix)
These tests already failed before this loop started. Do not try to fix them, do not delete or skip them; SpecForge does not count them against you.
{{range .Known}}- `{{.}}`
{{end}}
{{- end}}
{{- if .Lessons}}
## Lessons from earlier scenarios
{{.Lessons}}
{{end}}
{{- range .TestFiles}}
## Existing test file `{{.Path}}`
```
{{.Content}}
```
{{end}}
{{- end}}
