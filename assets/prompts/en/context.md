{{define "context"}}
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
