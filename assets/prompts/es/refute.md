# Tarea: REFUTAR — solo lectura

Las lentes de revisión informaron de los hallazgos de abajo sobre un cambio en el proyecto {{.Stack}} de **{{.SpecTitle}}**. Todos son *inferenciales*: dependen de cómo se use el código. Tu trabajo es intentar **refutar** cada uno, con independencia de quien lo encontró. **No cambies ningún fichero.**

Para cada hallazgo, lee el código de alrededor y el que lo llama. Está `refuted` cuando algo del código lo hace imposible (un tipo que no admite el valor malo, una comprobación previa, un llamador que nunca lo pasa). Está `confirmed` cuando puedes seguir un camino real hasta el problema. Si no puedes decidirlo, está `confirmed`: solo una refutación que puedas mostrar elimina un hallazgo.

## Hallazgos
{{range .Findings}}
### {{.ID}} · {{.Severity}} · `{{.Location.Path}}:{{.Location.Line}}`
{{.Claim}}
{{end}}
## El cambio (diff unificado)
```diff
{{.Diff}}
```
{{if .Feedback}}
## Tu respuesta anterior no se aceptó
{{.Feedback}}
{{end}}
## Respuesta
Termina con exactamente un objeto JSON en un bloque ```json, un veredicto por hallazgo:
```json
{"verdicts": [{"id": "REL-001", "verdict": "refuted", "reason": "Money rechaza importes negativos en su constructor (money.go:14), así que el bonus nunca es negativo"}]}
```
