# Tarea: CAMBIO — aplicar una petición de cambio a la especificación

El desarrollador pide un cambio en la especificación `{{.SpecPath}}` ("{{.SpecTitle}}"):

> {{.Request}}

SpecForge lleva la conversación: nunca hablas directamente con el desarrollador. En cada turno haces dos cosas y te detienes.

1. **Aplica lo que esté decidido** en `{{.SpecPath}}`: la petición, y la última respuesta del desarrollador si hay una abajo. Cambia todas las secciones a las que afecte, no solo los escenarios: invariantes, contratos de datos, errores, fuera de alcance, supuestos. Respeta las reglas de la especificación: un comportamiento por escenario, un `Cuando`, al menos un `Entonces`, sin palabras de interfaz ni de tecnología, cada invariante `INV-NN` cubierta por un escenario de fallo que la menciona.
   - **Mantén el título de cada escenario cuyo comportamiento sigue siendo el mismo, aunque edites sus pasos.** SpecForge sigue cada escenario por su título: un título igual conserva sus tests y su historia. Da a un comportamiento nuevo un escenario nuevo con un título nuevo y único; borra un escenario solo cuando la petición quita ese comportamiento.
   - **Nunca inventes.** Si la petición es ambigua, contradice una invariante u otro escenario, o deja un valor sin saber (un límite, un mensaje, quién puede hacerlo), pregunta antes de escribirlo.
   - Mantén las secciones numeradas y el front matter YAML tal como están, y no cambies ningún otro fichero.
2. **Después decide qué sigue sin saberse** de este cambio y haz la pregunta más importante, o termina.

Pregunta en lenguaje de negocio, una cosa cada vez, con opciones cuando haya pocas respuestas probables.
{{- if .Draft}}

## La especificación tal como está ahora
````markdown
{{.Draft}}
````
{{- end}}
{{- if .Decisions}}

## Este cambio hasta ahora
{{.Decisions}}
{{- end}}
{{- if .Answer}}

## El desarrollador acaba de responder
Pregunta: {{.AnsweredQuestion}}
Respuesta: **{{.Answer}}**
{{- end}}
{{- if .Feedback}}

## Aún no ha terminado
{{.Feedback}}
{{- end}}

## Contrato de respuesta (obligatorio)

Termina tu respuesta con exactamente un objeto JSON en un bloque ```json, y no escribas nada después.

Para hacer la siguiente pregunta:
```json
{"status": "needs_clarification", "question": "la pregunta", "options": ["respuesta probable A", "respuesta probable B"], "context": "por qué importa, en una frase", "section": "6. Escenarios", "unknowns": ["todo lo que sigue sin saberse de este cambio, incluida esta pregunta"]}
```

Cuando el cambio esté entero en la especificación y no quede nada sin saber de él:
```json
{"status": "done", "files_written": ["{{.SpecPath}}"], "summary": "qué cambió, en una o dos frases", "unknowns": []}
```

Si la petición no se puede aplicar (se contradice, o pide algo que no cabe en una especificación), responde con `{"status": "blocked", "reason": "…", "suggested_action": "…"}`.
