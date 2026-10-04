# Tarea: GREEN — haz pasar el test con el código mínimo

Trabajas sobre la especificación **{{.SpecTitle}}** (`{{.SpecPath}}`) en un proyecto {{.Stack}}.

## Escenario {{.Index}} de {{.Total}}
```gherkin
{{.Scenario}}```

## El test `{{.Marker}}` falla
```
{{.LastFailure}}
```

## Reglas
1. Cambia solo código de producción. **No crees, edites, renombres ni borres ningún fichero de test**: SpecForge guarda su huella y detiene el ciclo si alguno cambia.
2. Escribe el código mínimo que haga pasar este test (YAGNI). Mantén en verde el resto de la suite.
3. Nombra las cosas con el lenguaje ubicuo de abajo.
4. Si el escenario deja una regla de negocio sin definir, pregunta (`needs_clarification`) en lugar de decidir por el desarrollador.

SpecForge ejecutará `{{.TestCommand}}` y espera que pase.
{{if .Answer}}
## Tu pregunta tiene respuesta
Preguntaste: {{.AnsweredQuestion}}
El desarrollador respondió: **{{.Answer}}**
Queda en el registro de decisiones. Continúa la tarea con ella.
{{end}}
{{if .Feedback}}
## Tu intento anterior ({{.Attempt}} de {{.MaxAttempts}}) fue rechazado
{{.Feedback}}
{{end}}
{{- template "context" .}}
{{- template "contract" .}}
