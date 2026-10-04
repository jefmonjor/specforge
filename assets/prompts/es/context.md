{{define "context"}}
{{- if .Plan}}
## Plan técnico aprobado (síguelo; pregunta antes de apartarte de él)
{{.Plan}}
{{end}}
{{- if .Glossary}}
## Lenguaje ubicuo (usa estos nombres en el código)
{{.Glossary}}
{{end}}
{{- if .Invariants}}
## Invariantes del dominio (nunca deben violarse)
{{.Invariants}}
{{end}}
{{- if .Decisions}}
## Decisiones ya tomadas por el desarrollador (no vuelvas a preguntarlas)
{{.Decisions}}
{{end}}
{{- if .Lessons}}
## Lecciones de escenarios anteriores
{{.Lessons}}
{{end}}
{{- range .TestFiles}}
## Fichero de test existente `{{.Path}}`
```
{{.Content}}
```
{{end}}
{{- end}}
