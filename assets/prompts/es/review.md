# Tarea: REVISIÓN · lente {{.Lens}} — solo lectura

Revisas un cambio en el proyecto {{.Stack}} de la especificación **{{.SpecTitle}}**{{if .Scenario}}, escenario {{.Marker}}{{end}}. **No cambies ningún fichero**: SpecForge comprueba que nada cambió y rechaza la revisión si no.

## Tu lente: {{.Lens}}
{{- if eq .Lens "risk"}}
Seguridad: inyección, autenticación y autorización, secretos en código o logs, deserialización insegura, path traversal, ejecución de procesos, exposición de datos, dinero y redondeo, permisos.
{{- else if eq .Lens "reliability"}}
Corrección frente a la especificación y sus invariantes: resultados erróneos, casos límite olvidados (vacío, cero, negativos, límites, zonas horarias), errores que se esconden, comportamiento que la especificación no pide.
{{- else if eq .Lens "readability"}}
Claridad para el siguiente desarrollador: nombres que no siguen el lenguaje ubicuo, funciones que hacen varias cosas, duplicación, código muerto, comentarios que mienten, capas mezcladas (dominio haciendo E/S).
{{- else}}
Resiliencia ante fallos: timeouts, reintentos sin límite, escrituras parciales, concurrencia y estado compartido, fugas de recursos (ficheros, conexiones, goroutines), comportamiento con una dependencia caída.
{{- end}}
Informa solo de lo que es de esta lente; las otras cubren el resto.
{{if .Scenario}}
## El escenario
```gherkin
{{.Scenario}}
```
{{end}}
{{- if .Invariants}}
## Invariantes del dominio
{{.Invariants}}
{{end}}
{{- if .Plan}}
## Plan aprobado
{{.Plan}}
{{end}}
## El cambio (diff unificado)
```diff
{{.Diff}}
```

## Reglas para cada hallazgo
1. Señala el cambio. `proof_refs` debe nombrar una línea que el diff añade o modifica (`"kind": "changed-hunk"`, con `path` y el número de línea del lado nuevo) o un fichero que crea (`"kind": "new-file"`). SpecForge comprueba cada referencia contra el diff y **descarta** el hallazgo cuya prueba esté fuera.
2. Di cómo lo sabes: `evidence_class` es `deterministic` si se deduce del código, `inferential` si depende de cómo se use (se refutará antes de poder bloquear), `insufficient` si no puedes saberlo.
3. Di si lo causó el cambio: `causal_disposition` es `introduced`, `behavior-activated` (código viejo que ahora se alcanza) o `worsened`; `pre-existing` o `base-only` si ya estaba (un seguimiento, nunca bloquea); `unknown` si no lo sabes (decide el desarrollador).
4. `severity`: `BLOCKER` o `CRITICAL` solo para lo que no debe fusionarse; `WARNING` y `SUGGESTION` son información.
5. IDs: `{{.Prefix}}-001`, `{{.Prefix}}-002`… Mejor ningún hallazgo que uno inventado.
{{if .Feedback}}
## Tu respuesta anterior no se aceptó
{{.Feedback}}
{{end}}
## Respuesta
Termina con exactamente un objeto JSON en un bloque ```json:
```json
{"lens": "{{.Lens}}", "findings": [{"id": "{{.Prefix}}-001", "severity": "CRITICAL", "location": {"path": "src/pago/neto.go", "line": 42}, "claim": "qué está mal y por qué importa, en una o dos frases", "evidence_class": "deterministic", "causal_disposition": "introduced", "proof_refs": [{"kind": "changed-hunk", "path": "src/pago/neto.go", "line": 42}]}], "evidence": ["qué leíste o ejecutaste para llegar aquí"]}
```
Sin hallazgos, `"findings": []`; `evidence` nunca va vacío.
