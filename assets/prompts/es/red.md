# Tarea: RED — escribe el test que falla para un escenario

Trabajas sobre la especificación **{{.SpecTitle}}** (`{{.SpecPath}}`) en un proyecto {{.Stack}}.

## Escenario {{.Index}} de {{.Total}}
```gherkin
{{.Scenario}}```

## Qué hacer
1. Escribe el test automatizado que verifica los pasos `Then`/`Entonces` de este escenario a través del comportamiento observable, no de detalles de implementación. Un test por escenario.
2. El nombre del test **debe contener el marcador `{{.Marker}}`** para que SpecForge pueda ejecutarlo en solitario. {{if .MarkerHint}}Ejemplo: `{{.MarkerHint}}`{{end}}
3. El test debe **compilar y fallar en una aserción**. Si los tipos o funciones de producción que necesita aún no existen, añade los stubs mínimos para que compile (firmas que devuelvan valores vacíos o lancen "no implementado"). No implementes ningún comportamiento.
4. Coloca el test donde este proyecto guarda sus tests; amplía un fichero de test existente de esta funcionalidad si lo hay.
5. No modifiques ningún otro test.

SpecForge ejecutará `{{.TestCommand}}` y espera que falle en tu aserción.
{{if .Feedback}}
## Tu intento anterior ({{.Attempt}} de {{.MaxAttempts}}) fue rechazado
{{.Feedback}}
{{end}}
{{- if .LastFailure}}
### Salida de la ejecución anterior
```
{{.LastFailure}}
```
{{end}}
{{- template "context" .}}
{{- template "contract" .}}
