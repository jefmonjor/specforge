{{define "contract"}}
## Contrato de respuesta (obligatorio)

Termina tu respuesta con exactamente un objeto JSON dentro de un bloque ```json y no escribas nada después.

Cuando hayas terminado la tarea:
```json
{"status": "done", "files_written": ["ruta/relativa/de/cada/fichero/que/cambiaste"], "summary": "una frase"}
```

Si algo que necesitas no está en la especificación, en el registro de decisiones, en el código existente o en este prompt, **no lo supongas**. Una suposición errónea cuesta más que una pregunta. Responde en su lugar con:
```json
{"status": "needs_clarification", "question": "la pregunta exacta para el desarrollador", "options": ["opción A", "opción B"], "context": "por qué lo necesitas"}
```

Si no puedes continuar por algo que debe arreglar el desarrollador (una herramienta que falta, un build roto ajeno a esta tarea), responde con:
```json
{"status": "blocked", "reason": "qué ocurre", "suggested_action": "qué debe hacer el desarrollador"}
```

SpecForge comprueba `files_written` contra los ficheros que realmente cambiaron. Nunca incluyas un fichero que no hayas escrito.
{{end}}
