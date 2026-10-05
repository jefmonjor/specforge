{{define "context"}}
{{- if and .Legacy (not .Inventory)}}
## Código legacy (referencia de solo lectura)
Esto es una reescritura: el comportamiento sale del código legacy en `{{.Legacy}}`, y las fuentes legacy de la especificación dicen dónde. Lee esas fuentes para reproducir el comportamiento con exactitud, no cambies nunca un fichero de ahí y no arrastres su tecnología{{if .JavaRelease}}: el código nuevo es Java {{.JavaRelease}} y usa sus modismos (records, `java.time`, `List.of`, try-with-resources, `var` donde se lea mejor){{end}}.{{if .ForbiddenImports}} SpecForge rechaza cualquier import de {{range $i, $p := .ForbiddenImports}}{{if $i}}, {{end}}`{{$p}}`{{end}}.{{end}}
{{- if .LegacySources}}

### De dónde sale este comportamiento
{{.LegacySources}}
{{- end}}
{{end}}
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
{{- if .Known}}
## Fallos conocidos en esta rama (no son tuyos)
Estos tests ya fallaban antes de empezar el loop. No intentes arreglarlos, ni borrarlos ni saltarlos; SpecForge no te los cuenta.
{{range .Known}}- `{{.}}`
{{end}}
{{- end}}
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
