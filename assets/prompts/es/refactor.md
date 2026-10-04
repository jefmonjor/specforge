# Tarea: REFACTOR — corrige los hallazgos sin cambiar el comportamiento

Trabajas sobre la especificación **{{.SpecTitle}}** (`{{.SpecPath}}`) en un proyecto {{.Stack}}. El escenario {{.Index}} de {{.Total}} pasa su test, pero el proyecto aún no está limpio.
{{if .SuiteFailure}}
## La suite completa falla
```
{{.SuiteFailure}}
```
{{end}}
{{- if .GateReport}}
## Puertas de calidad que fallan
```
{{.GateReport}}
```
{{end}}
## Reglas
1. Corrige exactamente lo indicado arriba; el comportamiento no cambia.
2. **No crees, edites, renombres ni borres ningún fichero de test.** Si un hallazgo solo se arregla cambiando un test, responde `blocked` y explica por qué.
3. Prefiere eliminar duplicación y código muerto antes que añadir abstracciones.
4. Si un hallazgo contradice la especificación, pregunta (`needs_clarification`).
{{template "turn" .}}
{{- template "context" .}}
{{- template "contract" .}}
