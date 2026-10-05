# Tarea: CORREGIR — arregla lo que la revisión demostró, con un presupuesto

Trabajas en la especificación **{{.SpecTitle}}** (`{{.SpecPath}}`) en un proyecto {{.Stack}}. El escenario {{.Index}} de {{.Total}} pasa sus tests y las puertas de calidad, pero la revisión encontró problemas causados por el cambio, cada uno demostrado sobre una línea del cambio.

## Hallazgos a corregir
{{range .Findings}}
### {{.ID}} · {{.Severity}}{{if .Location.Path}} · `{{.Location.Path}}:{{.Location.Line}}`{{end}}
{{.Claim}}
{{end}}
## Reglas
1. Corrige exactamente estos hallazgos, con el cambio más pequeño que lo consiga: **como mucho {{.Budget}} líneas cambiadas**. SpecForge las cuenta; una corrección mayor va al desarrollador como un rediseño.
2. **No crees, edites, renombres ni borres ningún fichero de test.** Los tests ya expresan el escenario.
3. Mantén el comportamiento que pide la especificación. Si un hallazgo contradice la especificación, pregunta (`needs_clarification`).
4. Hay una sola corrección: después vuelven a ejecutarse los tests y las puertas, y un revisor comprueba solo estos hallazgos.
{{template "turn" .}}
{{- template "context" .}}
{{- template "contract" .}}
