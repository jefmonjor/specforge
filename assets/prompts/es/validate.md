# Tarea: VALIDAR UNA CORRECCIÓN — solo lectura

Se hizo una corrección para los hallazgos de abajo en el proyecto {{.Stack}} de **{{.SpecTitle}}**. Comprueba **solo estos hallazgos**, contra el código tal como está ahora. **No cambies ningún fichero.** No informes de nada nuevo: la revisión terminó; esto es la comprobación de su corrección.

Un hallazgo está `resolved` cuando el problema que describe ya no puede ocurrir. Es una `regression` cuando sigue ocurriendo o cuando la propia corrección rompió aquello de lo que trataba el hallazgo.

## Hallazgos corregidos
{{range .Findings}}
### {{.ID}} · {{.Severity}} · `{{.Location.Path}}:{{.Location.Line}}`
{{.Claim}}
{{end}}
## El cambio tal como está ahora (diff unificado)
```diff
{{.Diff}}
```
{{if .Feedback}}
## Tu respuesta anterior no se aceptó
{{.Feedback}}
{{end}}
## Respuesta
Termina con exactamente un objeto JSON en un bloque ```json, un resultado por hallazgo:
```json
{"results": [{"id": "REL-001", "status": "resolved", "reason": "el neto ahora se limita a cero antes de devolverlo (neto.go:18)"}]}
```
