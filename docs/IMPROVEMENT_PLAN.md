# SpecForge — Plan de mejora integral (v3.0.0 → v4)

> Revisión con el nivel de exigencia de una PR contra spec-kit u OpenSpec. Se ha leído todo el código Go (5.628 líneas de producción, 1.242 de tests), los assets embebidos, los prompts y la guía; se ha compilado, ejecutado la suite (38,4 % de cobertura) y reproducido los fallos marcados con ✔. Cada hallazgo lleva `fichero:línea`.
>
> Severidad: **🔴 bloqueante** = rompe la promesa central · **🟠 mayor** = brecha metodológica o de diseño · **🟡 menor** = pulido.

---

## 0. Los ocho principios que gobiernan este plan

Antes de los hallazgos, la vara de medir. Cada decisión del plan se justifica con uno de estos ocho principios, y cada fase del roadmap se cierra comprobándolos. Si una mejora no sirve a ninguno, sobra.

| # | Principio | Qué significa en SpecForge | Cómo se comprueba |
| :-: | :--- | :--- | :--- |
| **P1** | **Pregunta, no inventes** | Ante cualquier duda funcional, técnica o de contexto, el agente **devuelve una pregunta**, SpecForge **pausa** y se la hace al desarrollador. En **todas** las fases, no solo en la entrevista. Nunca se rellena un hueco con una suposición. | Todo prompt admite la respuesta `{"status":"needs_clarification","question":…}` y el runtime la gestiona. Cero suposiciones silenciosas en el log. |
| **P2** | **Iteración con revisión del desarrollador** | El flujo son ciclos cortos con un punto de revisión humano al final de cada uno: spec, plan, cada escenario, entrega. El desarrollador aprueba, corrige o rechaza; SpecForge recuerda la decisión. | Cada puerta de revisión existe como comando (`approve`) o como pausa interactiva, y queda registrada en el estado. |
| **P3** | **Todo valida, nada aprueba por defecto** | Una puerta que no pudo ejecutarse **no** aprueba. Un JSON ilegible **no** es un informe vacío. Un test que no compila **no** es RED. Fail-closed siempre. | Ningún `return true` sin haber ejecutado la comprobación. Tri-estado `Passed / Failed / Skipped` y `--strict` convierte `Skipped` en fallo. |
| **P4** | **Clean Architecture + SOLID de verdad** | Dominio puro (sin `os`, `exec`, `filepath`), casos de uso en `internal/app`, puertos definidos por quien los consume, adaptadores finos, una sola razón de cambio por tipo. SpecForge exige esto al código del usuario: debe cumplirlo él primero. | `go list -deps ./internal/domain` no contiene `os`/`exec`. Ninguna función > 60 líneas. Composition root único. |
| **P5** | **Código artesano** | Nombres que dicen lo que hacen, errores tipados, sin `_ =` sobre escrituras, sin números mágicos, tests que describen comportamiento (no strings de UI), godoc en inglés. | `golangci-lint` con `errcheck`, `gocyclo`, `funlen`, `goconst`, `revive` en verde. |
| **P6** | **Fácil y sin reinventar la rueda** | Si existe una librería madura y mantenida, se usa. Menos comandos, menos ficheros de memoria, menos modos. Un nombre, una ruta de config, un idioma de código. | Tabla §7: cada pieza "a mano" sustituida por su librería. Superficie del CLI reducida de 11 a 8 comandos. |
| **P7** | **Specs claras y memoria útil** | La spec tiene todo lo que un desarrollador necesita para no preguntar lo obvio (actores, invariantes, NFR, contratos de datos, errores, fuera de alcance). La memoria del agente es corta, curada y relevante para el stack y la fase; nunca crece sin control. | Lint de spec en verde; `lessons.md` ≤ 30 entradas, con dedupe y origen; solo se inyecta lo pertinente. |
| **P8** | **Entrega clara** | Al terminar una feature, el desarrollador recibe un paquete que se explica solo: spec aprobada, plan, tests trazados a escenarios, código, informe de puertas, decisiones tomadas y preguntas respondidas, y una descripción de PR generada a partir de todo ello. | `specforge deliver NNNN` produce `DELIVERY.md` + cuerpo de PR + `trace.json`, y nada en él es inventado: cada línea sale de un artefacto. |

---

## 1. Veredicto en una página

**La idea es muy buena y el hueco existe.** spec-kit y OpenSpec son *paquetes de prompts* que viven dentro del agente: el agente se corrige los deberes a sí mismo. SpecForge es el único que es un **runtime externo y determinista** que ejecuta el compilador y se niega. "CI del proceso, no solo del resultado" es la frase correcta.

**La implementación aún no cumple la promesa.** Medida contra los ocho principios:

| Promesa | Realidad | Principio roto |
| :--- | :--- | :---: |
| Entrevista socrática | Un prompt volcado a `claude`/`gemini` interactivo. SpecForge no ve la conversación ni sabe qué quedó sin resolver. `cmd/interview.go:118-189` | P1, P2 |
| La IA no programa con dudas | Solo en `loop` y solo si el agente **recordó** escribir `[NEEDS CLARIFICATION]`. En RED/GREEN/REFACTOR/E2E/audit **no tiene forma de preguntar**: inventa o falla. `dispatcher.go:52-59`, `loop.go:311-331` | **P1** |
| Spec aprobada y sellada | Se sella **incondicionalmente** al salir el proceso, Ctrl-C incluido. No hay puerta humana. `cmd/interview.go:194-212` | P2, P3 |
| RED: el test debe fallar | Toda la suite y exit code. Un test que **no compila** es RED válido; si el agente no escribió nada, falsa "violación YAGNI". `loop.go:199-209` | P3 |
| GREEN: solo código productivo | Nada prohíbe ni detecta que el agente edite los tests. `loop.go:224-235` | P3 |
| `--resume` exacto | `os.WriteFile` no atómico y **todos** los `state.Save` ignoran el error. `state.go:94`, `loop.go:149…296` | P3, P5 |
| Puertas de calidad | jscpd/knip/stryker **aprueban cuando no pudieron ejecutarse**; SonarGate aprueba sin hacer una petición. ✔ `quality/jscpd.go:37-40`, `quality.go:58-65` | **P3** |
| Auditoría de seguridad | JSON ilegible ⇒ **informe vacío** ⇒ "✓ superada". ✔ `cloudflare_auditor.go:108-114` | **P3** |
| Parser Gherkin | `line[5:]`: `Cuando se retira` → `"o se retira"`. `### Scenario:` colapsa en un escenario. ✔ `spec.go:120-126` | P3, P6 |
| Memoria que aprende | Append ciego de trazas truncadas con solución hardcodeada; se reinyecta todo en cada prompt. `agent_memory.go:65-96` | P7 |
| Entrega | No existe. El loop termina con un banner. Sin informe, sin traza, sin PR. | **P8** |
| Herramienta genérica | ~40 % es el banco: ADC de GCP, Tekton, Nexus, AS/400, javax→jakarta, PATH por PowerShell. | P6 |

**Qué hacer:** no reescribir. La hexagonal es real y el esqueleto del dominio es bueno. Hay que (1) convertir "pregunta, no inventes" en un mecanismo del runtime en todas las fases, (2) poner puertas de revisión humana donde faltan, (3) hacer que todas las puertas automáticas fallen cerradas, (4) completar el ciclo de vida de la spec, (5) definir la entrega y (6) hacer el Go digno de lo que exige a los demás. En ese orden.

---

## 2. Lo que está bien (y hay que proteger)

- **El posicionamiento**: puerta externa determinista sobre el agente. Único.
- **`cmd/ → domain / ports / adapters`** existe de verdad; los puertos son pequeños.
- **`TDDState` + `AdvanceToNextPhase`** (`state.go:140-158`): modelo simple y correcto.
- **El sello SHA-256 con limpieza de la línea del sello** (`spec.go:41-51`): idea correcta, le falta normalización.
- **La cadena recon → hunt → validate** con contratos JSON (`cloudflare_auditor.go`): el mejor prompt del repo.
- **Binario único con `go:embed`**: distribución impecable.
- **La caja de diagnóstico** (`diagnostic.go`): UX que nadie más tiene; solo necesita errores tipados.
- **`[NEEDS CLARIFICATION]` como bloqueo duro** en `loop`: es la semilla de P1. Hay que generalizarla.

---

## 3. P1 — "Pregunta, no inventes" como mecanismo, no como frase

Hoy el principio vive en dos sitios: una regla del prompt de entrevista (`00_interview_bdd.md:27-30`) y un grep de `[NEEDS CLARIFICATION]` al arrancar `loop` (`loop.go:123-141`). En las otras cinco fases el agente **no tiene cómo preguntar**: el prompt le dice "implementa" y él implementa, suponiendo lo que falte. Eso es exactamente lo que SpecForge dice combatir.

### 3.1 El protocolo de respuesta universal

Todo prompt que SpecForge envía al agente termina con el mismo contrato de salida, y el runtime lo trata igual en todas las fases:

```json
{ "status": "done",
  "files_written": ["internal/auth/reset_test.go"],
  "summary": "Test for scenario SDD-0001-3 asserting single-use link expiry" }
```
```json
{ "status": "needs_clarification",
  "question": "The spec says the link is valid for 30 minutes. Is that wall-clock from request time, or from email delivery?",
  "blocking": true,
  "options": ["request time", "delivery time"],
  "context": "scenario SDD-0001-3, step Then" }
```
```json
{ "status": "blocked",
  "reason": "pytest is not configured in this repo and the spec requires Python tests",
  "suggested_action": "run `specforge setup --stack python` or add pytest to pyproject" }
```

Qué hace SpecForge con cada estado:

| Estado | Runtime | Registro |
| :--- | :--- | :--- |
| `done` | Verifica `files_written` contra `git status` (si no coincide ⇒ error, no se confía). Sigue con la puerta de la fase. | Checkpoint con ficheros y resumen. |
| `needs_clarification` | **Pausa.** Imprime la pregunta con contexto y opciones. En TTY: pregunta al desarrollador y reanuda con la respuesta añadida al prompt. Sin TTY (CI): sale con código 5 y deja la pregunta en `specs/NNNN/questions.md`. Si `blocking:false`, anota y sigue. | La pregunta **y la respuesta** se guardan en `specs/NNNN/decisions.md` (fecha, fase, escenario). Si la respuesta cambia la spec, se enruta a `spec clarify` y se re-sella. |
| `blocked` | Para. Caja de diagnóstico con `suggested_action`. | Checkpoint `BLOCKED`. |
| no parsea | Un reintento con "responde solo el JSON del contrato". Segundo fallo ⇒ `blocked`. **Nunca** se interpreta prosa como éxito. | Log de la salida cruda bajo `--trace-io`. |

### 3.2 Dónde se aplica (hoy / objetivo)

| Fase | Hoy | Objetivo |
| :--- | :--- | :--- |
| Entrevista | El agente puede escribir `[NEEDS CLARIFICATION]` si se acuerda. | Bucle por turnos propiedad de la herramienta (§4.1). Cada turno **es** una pregunta. Finaliza solo con `unknowns_remaining == []`. |
| Plan | No existe. | `needs_clarification` para decisiones de arquitectura ("¿el repositorio va en memoria o Postgres?"). Se responde una vez, queda en `decisions.md`, no se vuelve a preguntar. |
| RED | Inventa el nombre del fichero, el framework de test, el import. | Pregunta si no está en el plan: "No hay fichero de test para `auth`; ¿creo `auth_test.go` o `auth/reset_test.go`?". |
| GREEN | Inventa la implementación de lo que la spec no dice. | Pregunta por reglas de negocio ausentes. Ejemplo real de esta auditoría: la spec dice "válido 30 minutos" sin decir desde cuándo. |
| REFACTOR | Un intento ciego. | Puede responder `needs_clarification` si una violación de calidad entra en conflicto con la spec ("jscpd marca duplicado entre dos DTOs que la spec exige separados"). |
| E2E | Alucina selectores hasta agotar 15 pasos. | `needs_clarification` cuando no encuentra el elemento que el `Then` describe. |
| Audit | — | Un hallazgo `needs_validation` se convierte en pregunta al desarrollador, no en silencio. |

### 3.3 La regla que lo sostiene

En el bloque gestionado de `CLAUDE.md`/`GEMINI.md` que `setup` escribe (una vez, no en cada prompt):

> You are working under SpecForge. If anything needed to complete the current task is not in the spec, the plan, the existing code or the decisions log, **do not assume it**: answer with `status: needs_clarification` and the exact question. A wrong guess costs more than a question. Never edit test files during GREEN or REFACTOR. Never claim to have written a file you did not write.

Es la única regla que importa; las demás ("KISS", "YAGNI", "Dominio Rico") son consecuencia o ruido.

---

## 4. P2 — El bucle de revisión del desarrollador

El flujo actual tiene **una** interacción humana (la conversación de la entrevista) y luego corre solo hasta el final o hasta un error. Un desarrollador que revisa el resultado de ocho escenarios de golpe no revisa: acepta. El objetivo son ciclos cortos con una revisión al final de cada uno, y que la revisión quede registrada.

### 4.1 Puertas de revisión

| Puerta | Cuándo | Qué revisa el desarrollador | Cómo se registra | Hoy |
| :--- | :--- | :--- | :--- | :---: |
| **R0 · Alcance** | Fin de la entrevista | Que la spec dice lo que él quería y nada más. Intención, fuera de alcance, invariantes. | `spec approve` escribe `approved_by`, `approved_at`, sello. | ✗ (se sella sola) |
| **R1 · Plan** | Tras `plan` | Dónde va el código, qué fichero de test por escenario, qué puertos/adaptadores. Es barato corregir aquí y carísimo después. | `plan approve` (o `--auto-approve-plan` para quien confíe). | ✗ (no hay plan) |
| **R2 · Escenario** | Tras cada REFACTOR verde | Diff del escenario (`git diff` acotado a `files_written`), test con marcador, informe de puertas. Opciones: aceptar · pedir cambio (vuelve a GREEN con el comentario) · rechazar (vuelve a RED). | Checkpoint `REVIEWED` con veredicto y comentario. Configurable: `review: every-scenario \| every-N \| end`. | ✗ |
| **R3 · Preguntas** | Cuando el agente devuelve `needs_clarification` | La pregunta. | `decisions.md`. | ✗ |
| **R4 · Entrega** | `deliver` | `DELIVERY.md` y cuerpo de PR. | Commit / PR. | ✗ |

### 4.2 Iteración corta por diseño

- **Un escenario, un ciclo, un commit.** Tras R2 aceptado, SpecForge hace un commit `feat(SDD-0001-3): <título del escenario>` con los `files_written`. El historial de git **es** la traza. Hoy el loop no commitea nada y `files_written` ni se conoce.
- **El desarrollador puede intervenir a mitad.** `specforge loop --scenario 3 --from green` repite solo esa fase. Hoy `--resume` es todo o nada.
- **Las decisiones no se repreguntan.** Antes de lanzar un prompt, el runtime inyecta `decisions.md`. Si la misma duda vuelve, es un bug del prompt, no una pregunta al usuario.
- **El tamaño del paso lo marca la spec.** Un `Scenario Outline` con 6 `Examples` son 6 pasos pequeños, no uno grande (hoy el Outline no se expande, §5.3).

---

## 5. Spec clara: petición, contenido y ciclo de vida (P7, P1, P2)

### 5.1 Petición de spec (la entrevista)

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `cmd/interview.go:118-189`, `agent/claude.go:104-119` | "Socrático" = un prompt a `claude <prompt>` interactivo. Cero visibilidad: sin turnos, sin lista de incógnitas, sin saber si hubo aprobación. | **La herramienta posee el bucle**: cada turno es una llamada headless con contrato `{question, why_it_matters, section, unknowns_remaining[], draft_delta}`. SpecForge pregunta, lee, persiste `interview.jsonl`, y no finaliza hasta `unknowns_remaining == []`. Una pregunta por turno, con opciones cuando las haya. |
| 🔴 | `cmd/interview.go:194-212` | Sella **siempre** al terminar, aunque sea Ctrl-C en la pregunta 1. La plantilla promete GATE 1 "validación explícita del Product Owner" (`spec-template.md:58`) y no existe. Si el agente no escribió, se sella la plantilla con placeholders y `loop` los parsea. | `spec approve` (puerta R0): lint, rechaza `<…>`/`NNNN`/`DRAFT`, sello + aprobador. `interview` **nunca** sella. |
| 🟠 | `cmd/interview.go:153` | Toda spec es `0001-<slug>.md`; `findLatestSpec` elige por *mtime*. | `NNNN` = máximo + 1; `spec_id` en el estado; `--spec` obligatorio si hay más de una. |
| 🟠 | `cmd/interview.go:88-116` | El context pack entero (código) se incrusta en una entrevista de **negocio** que prohíbe hablar del "cómo". | A la entrevista: árbol + glosario + specs existentes. El código va al `plan`. |
| 🟠 | `00_interview_bdd.md:15-30` | Faltan **actores**, **NFR**, **contratos de datos** (`service-ficha.template.md` los tiene y nadie lo usa), **catálogo de errores**, **supuestos**, **métricas**. Las INV-xx no se trazan a escenarios. | Secciones 8-10 y lint "toda INV-xx referenciada por ≥1 escenario". Ver §5.4. |
| 🟡 | `cmd/interview.go:163,220-233` | Solo se sustituye un placeholder; `sanitizeSlug` tira acentos (`Liquidación` → `liquidacin`, y el test lo da por bueno). | `text/template`; NFD + quitar `Mn`. |

### 5.2 Ciclo de vida (lo que OpenSpec tiene y SpecForge no)

| Sev | Problema | Mejora |
| :---: | :--- | :--- |
| 🟠 | No hay `clarify`. Los `[NEEDS CLARIFICATION]` se resuelven editando a mano (`loop.go:135-136`), lo que **rompe el sello** que el mismo comando exige. | `spec clarify NNNN`: itera marcadores, pregunta uno a uno (P1), parchea, re-sella con `supersedes:`. Las respuestas van a `decisions.md`. |
| 🟠 | No hay gestión de cambios: un hash y un error. | `spec amend NNNN "…"` → `changes/0002-<slug>.md` con ADDED/MODIFIED/REMOVED por escenario, re-sello, reset del estado **solo** para escenarios afectados. |
| 🟠 | `--resume` (`loop.go:74-90`) compara contra el hash **viejo**: una spec re-sellada legítimamente nunca se reanuda. | Releer, revalidar, diffear por título: conservar completados, re-RED los cambiados. |
| 🟠 | Dos algoritmos de limpieza del sello (`interview.go:255-280` vs `spec.go:41-51`); en Windows el hash depende de CRLF ⇒ `git autocrlf` **rompe todos los sellos al clonar**. Regex sin anclar coge la *primera* coincidencia. | Un único `domain.Seal`/`StripSeal`; normalizar `\r\n` y espacios finales; `sha256-v1:`; regex anclado a la última línea. |
| 🟠 | No hay plan. `plan-template.md` se extrae y **nadie lo lee**. El escenario 3 puede contradecir la estructura del 1. | `plan NNNN` (una llamada) → `plan.md` + puerta R1. |

### 5.3 Parser y lint

| Sev | Dónde | Problema | Mejora (P6: no reinventar) |
| :---: | :--- | :--- | :--- |
| 🔴 ✔ | `spec.go:120-126` | `line[5:]` para seis keywords. Verificado: `Cuando se retira 30` → `"o se retira 30"`. El test solo mira `len > 0`. | **`github.com/cucumber/gherkin/go`**: parser oficial, 70+ idiomas, And/But, Background, Outline + Examples, tablas. Borrar el parser casero. |
| 🔴 ✔ | `spec.go:97,134-141` | `### Scenario:` no casa ⇒ **toda la spec es un escenario** "Requerimiento General". | Extraer los bloques ```` ```gherkin ```` del Markdown y pasarlos al parser oficial. `ErrNoScenarios` en vez de inventar. |
| 🟠 | `spec.go:69-83` | `ExtractNeedsClarification` casa la prosa de la plantilla ⇒ spec bloqueada para siempre. | Regex estructurado; saltar código y comentarios. |
| 🟠 | — | Sin lint: un `When`, ≥1 `Then`, títulos únicos, términos de implementación ("click", "endpoint", "JPA"). | `spec lint` dentro de `approve` y `loop`. |

### 5.4 Qué tiene una spec clara (plantilla objetivo)

Una spec es clara cuando el desarrollador y el agente pueden implementarla **sin preguntar lo que ya debería estar escrito**, y cuando lo que no está escrito está marcado como abierto. Cada sección tiene regla de lint:

| § | Sección | Regla de lint | Para qué sirve al agente |
| :-: | :--- | :--- | :--- |
| 1 | Intención (por qué) | ≥ 2 frases; sin "cómo". | Elegir entre dos implementaciones válidas. |
| 2 | Actores y roles | ≥ 1 actor; cada US usa uno de ellos. | No inventar permisos. |
| 3 | Lenguaje ubicuo | Cada término usado en escenarios está definido. | Nombres de tipos y funciones. **Se genera el glosario de memoria desde aquí**, no al revés. |
| 4 | Invariantes INV-xx | Cada INV referenciada por ≥ 1 escenario (Sad Path). | Qué validar siempre. |
| 5 | Historias de usuario | Formato Como/Quiero/Para; ≥ 1 escenario por US. | Alcance. |
| 6 | Escenarios Gherkin | Un `When`, ≥ 1 `Then`, título único, sin UI/tecnología, Outline con Examples. | El test. |
| 7 | Contratos de datos | Tabla campo / tipo / obligatorio / regla; cada campo nombrado en un escenario. | Firmas y DTOs sin adivinar. |
| 8 | Catálogo de errores | Código / cuándo / mensaje; cada error aparece en un Sad Path. | Manejo de errores sin inventar. |
| 9 | Requisitos no funcionales | Latencia, volumen, disponibilidad, seguridad, con número o "no aplica" explícito. | Decidir estructuras de datos. |
| 10 | Fuera de alcance | ≥ 1 ítem. | YAGNI con base. |
| 11 | Supuestos | Lo que se da por hecho y quién lo confirmó. | Trazabilidad. |
| 12 | Cuestiones abiertas | `- [NEEDS CLARIFICATION]: …`; **debe estar vacía para `approve`**. | P1. |
| — | Metadatos | `id`, `estado`, `aprobado_por`, `fecha`, `sello`, `supersede`. | Ciclo de vida. |

---

## 6. P3 — Todo valida: el bucle TDD, las puertas, el auditor, el E2E

### 6.1 Semántica RED / GREEN / REFACTOR

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `loop.go:224-235`, `:275` | **Manipulación de tests ni prohibida ni detectada.** | Tras RED: SHA-256 de cada fichero de test en el estado. Tras GREEN y REFACTOR: re-hash; diff ⇒ `TEST TAMPERING`. Prohibición en el bloque gestionado (§3.3). |
| 🔴 | `loop.go:199-209`, `dispatcher.go:81-94` | RED = exit code de toda la suite. No compila ⇒ RED válido. Sin cambios ⇒ falsa YAGNI. Test rojo preexistente ⇒ todo RED pasa. Stack desconocido ⇒ `echo` exit 0 ⇒ **todo proyecto no soportado es "violación YAGNI"**. | `TestOutcome{Compiled, Passed, Failed int, Output}` parseando **`go test -json`**, **surefire XML**, **vitest `--reporter=json`**, **pytest `--junitxml`** (formatos existentes, P6). Gate RED = `Compiled && Failed > 0`. `git status` antes de ejecutar. `ErrUnsupportedStack`. |
| 🔴 | `loop.go:201-224` | La salida del RED se descarta: el **primer** GREEN va a ciegas. | `state.LastError = output` antes de avanzar. |
| 🟠 | `dispatcher.go:81-94` | `scenario` se acepta y **se ignora**. | Marcador `[SDD-0001-3]` en el nombre del test; ejecutar solo ese (`-run`, `-Dtest=`, `-t`, `-k`); suite completa en REFACTOR. |
| 🟠 | `loop.go:218,245-254` | El `3` mágico 7 veces; "Intento 4/3" tras resume. | `const maxGreenRetries`; `--max-retries`; reset en resume. |
| 🟠 | `loop.go:201,233,276,279` | Errores de `RunTests`, agente y gate silenciados. Toolchain ausente = "RED legítimo". | `errors.Is(err, exec.ErrNotFound)` ⇒ `blocked`. Propagar. |

### 6.2 Puertas de calidad

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 ✔ | `jscpd.go:37-40`, `knip.go:33-36`, `stryker.go:42-44` | `err != nil && ExitCode == 1 && Output == ""` ⇒ **`passed = true`**. Casa también con `npx` sin red o la herramienta crasheando. | `GateResult{Passed, Skipped, Tool, Report}`; `ErrNotFound` ⇒ `Skipped` (⚠); `--strict` ⇒ fallo. |
| 🔴 ✔ | `quality.go:58-65` | SonarGate siempre `true`, "validado contra <host>" sin petición. | Borrar. |
| 🟠 | `stryker.go:52`, `jscpd.go:48`, `knip.go:32` | Veredicto por substring de salida humana. | `--reporters json` y leer el número (P6). |
| 🟠 | Umbrales: Stryker 80 (constitución) vs `break: 50` (scaffold); JaCoCo 70 vs "≥80" sin gate. | Inconsistentes y no aplicados. | Bloque `quality:` en `.specify/config.yaml` leído por scaffolds **y** gates. |
| 🟠 | `linter.go:28,37-44,63` | `npm run lint --if-present` sin script ⇒ limpio. Error de `golangci-lint` ⇒ "violación" que el loop pide arreglar a la IA. Python sin ruff ⇒ aprobado. | Detectar script; exit 1 vs ≥ 2; fallback `go vet`; `Skipped`. |

### 6.3 Auditor y E2E

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 ✔ | `cloudflare_auditor.go:108-114` | JSON ilegible ⇒ informe vacío ⇒ "✓ superada". | Fail-closed; un reintento con el error de esquema (§3.1). |
| 🔴 | `:134`, `audit.go:39,95` | `git diff <target>` sin validar: `--target='--output=~/.bashrc'` ⇒ inyección de argumentos. | `--end-of-options`; rechazar `-*`; `rev-parse --verify`. |
| 🟠 | `:108` | `ValidateFindingsAgainstSchema(..., nil)`: el esquema embebido **nunca se usa**. | **`santhosh-tekuri/jsonschema`** con `report-schema.json` (P6). |
| 🟠 | `security.go:178-181` | `needs_validation` ignorado por `--fail-on`. | ≥ high bloquea; cada uno es una pregunta R3. |
| 🔴 | `chromedp_driver.go:53-54,166-185` | Todo ignora el `ctx`; `WaitVisible` sobre selector alucinado **cuelga para siempre**. | ctx del llamador; timeout por acción; ambos cancels. |
| 🟠 | `vision_agent.go:226-241` | Sin validar: cualquier `action_type`, cualquier selector, `navigate` a cualquier URL. Texto de la página en el prompt ⇒ **inyección desde una página maliciosa**. | Enum, selector ∈ snapshot, same-origin, tope de `value`. |
| 🟠 | `chromedp_driver.go:39,117-122` | `ignore-certificate-errors` siempre; el snapshot **muta el DOM**. | Opt-in; selectores sin mutar. |

---

## 7. P6 — No reinventar la rueda

Cada pieza escrita a mano que ya existe madura, mantenida y con tests ajenos:

| Hoy a mano | Problema real que ha causado | Sustituir por |
| :--- | :--- | :--- |
| Parser Gherkin (`spec.go:86-144`) | Mutila el español, ignora And/But/Outline/Background. ✔ | `github.com/cucumber/gherkin/go` |
| Matcher de `.gitignore` (`context.go:65-102`) | Ignora negaciones, anclas, `**`; `bin` mata `src/bin/x.go`. | `github.com/sabhiram/go-gitignore` o `go-git/plumbing/format/gitignore` |
| Exec + Windows `.cmd/.exe` (`dispatcher.go:96-148` y copia en `quality/process.go`) | Duplicado byte a byte; sin timeout; `ErrNotFound` indistinguible. | Un `adapters/exec` propio fino sobre `os/exec` + `exec.LookPath`, con `WaitDelay`. (Aquí sí, uno propio: 60 líneas.) |
| Logger singleton (`storage/logger.go`) | Todo al fichero, 0644, sin rotación. | `log/slog` + `gopkg.in/natefinch/lumberjack.v2` |
| Validación de esquema (`security.go`) | Esquema embebido sin usar. | `github.com/santhosh-tekuri/jsonschema/v6` |
| Parseo de resultados de test (exit code) | No distingue compila/falla. | `go test -json`, surefire XML, vitest JSON, pytest junitxml |
| Veredicto de gates por substring | Depende de versión e idioma. | Reporters JSON de jscpd / knip / stryker |
| PATH por PowerShell interpolado (`system/path.go`) | Inyección con `'` en la ruta. | **Eliminar.** Homebrew / Scoop / `go install` / release con `goreleaser`. |
| Build scripts × 4 (`build.sh`, `build.ps1`, `build-all.*`) | Sin `-X`, sin checksums. | `goreleaser` (binarios, checksums, SBOM, release notes) |
| Enums como `string` sin validar | `--agent copilot` ⇒ Gemini en silencio. | `type AgentName string` + `Parse*` + `pflag.Value` |
| Config en `~/.specforge` y `~/.sdd` | Dos rutas, 0644 con token. | `os.UserConfigDir()`; 0600 |
| 6 ficheros de memoria inyectados en `-p` | Cosmético; los agentes ya leen `CLAUDE.md`/`GEMINI.md`. | Bloque gestionado en esos ficheros; `claude -p --append-system-prompt --output-format json` |
| Spinner + `time.Sleep(50ms)` | Carrera; CI lleno de `\r`. | `github.com/briandowns/spinner` o `charmbracelet/bubbles`, con TTY check |
| Conversión PDF/Word (`doc/`) | Shell-out a Python en un "zero deps". | Flag `--docs` opcional o eliminar del core |

**Menos superficie**: de 11 comandos (`init setup ingest doc interview loop e2e audit consistency build version`) a 8 (`init setup spec plan loop deliver e2e audit`), con `spec` agrupando `new interview clarify lint approve amend`. `ingest` pasa a ser interno de `plan`; `doc` es un flag; `consistency` y `build` se van.

---

## 8. P7 — Memoria útil

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🟠 | `agent_memory.go:65-96`, `loop.go:238,287` | `RecordLesson` = timestamp + 120 chars del **stderr del compilador** + solución **hardcodeada**. Sin dedupe ni tope. Todo se reinyecta en cada prompt. | La lección la escribe el agente al cerrar el escenario: "en ≤ 2 líneas, la regla que lo habría evitado". Dedupe por texto normalizado, tope 30, etiqueta `stack/gate`, inyectar solo las pertinentes. |
| 🟠 | `assets/baseline/agent-context/*` | Glosario con MDCMS/AS400/Tekton; `references.md` es YAML de Spring Actuator; reglas de IBAN para una SPA. | Baselines por stack. **Glosario generado desde la spec §3**, no al revés. |
| 🟡 | Seis ficheros | `agente`+`persona`+`ng-rules` son uno; `glossary` duplica la spec. `.sdd/` gitignored ⇒ las "lecciones del equipo" no se comparten. `setup` sobreescribe sin `--force`. | Bloque gestionado en `CLAUDE.md`/`GEMINI.md` (reglas) + `lessons.md` **versionado** + `decisions.md`. |

### Qué recuerda SpecForge y dónde

| Memoria | Fichero | Quién la escribe | Cuándo se inyecta |
| :--- | :--- | :--- | :--- |
| Reglas del proyecto (P1, no tocar tests, estándar del stack) | `CLAUDE.md` / `GEMINI.md` bloque gestionado | `setup` | Siempre (nativo del agente) |
| Decisiones y respuestas a preguntas | `specs/NNNN/decisions.md` | Runtime en cada R3 | Siempre, en la spec actual |
| Lecciones (fallos y la regla que los evita) | `.sdd/lessons.md` (versionado) | Agente al cerrar escenario; dedupe | Las del stack y gate actuales |
| Glosario | Sección 3 de la spec | Entrevista | Con el escenario |
| Plan | `specs/NNNN/plan.md` | `plan` | RED y GREEN |
| Transcripción de la entrevista | `specs/NNNN/interview.jsonl` | `spec interview` | Nunca en prompts; para auditoría y replay |

---

## 9. P8 — Entrega clara

Hoy el loop termina con "🎉 CICLO TDD COMPLETADO" y nada más. No hay informe, no se sabe qué ficheros cambiaron, qué puertas corrieron ni qué se preguntó. El desarrollador tiene que reconstruirlo. Eso es lo contrario de una entrega.

### 9.1 `specforge deliver NNNN`

Genera, **solo a partir de artefactos existentes** (nada lo escribe el modelo de memoria):

```text
specs/0001-password-reset/
  DELIVERY.md        ← el paquete legible
  trace.json         ← escenario → test → ficheros → commit → puertas
  PR_BODY.md         ← listo para pegar o para `gh pr create --body-file`
```

`DELIVERY.md`:

```markdown
# Entrega · SDD-0001 Password reset

**Spec** 0001 · aprobada por @jefmonjor el 2026-10-06 · sello sha256-v1:f15155…
**Plan** aprobado el 2026-10-06 · 3 componentes · 4 ficheros de test
**Escenarios** 6/6 verdes · 2 revisados con cambios · 0 rechazados

| # | Escenario | Test | Commit | Gates |
|---|---|---|---|---|
| 1 | User requests a reset link | auth/reset_test.go::TestReset_RequestsLink [SDD-0001-1] | a1b2c3d | lint ✓ dup ✓ |
| … |

## Decisiones tomadas durante el desarrollo
- (GREEN, esc. 3) "¿30 min desde la petición o desde la entrega?" → **desde la petición** (@jefmonjor)
- (PLAN) "¿repositorio en memoria o Postgres?" → **Postgres, puerto `ResetTokenRepo`**

## Lecciones registradas
- Go: `time.Now()` en el dominio rompe el test de expiración → inyectar `Clock`.

## Puertas
- Tests: 14 nuevos, 0 modificados fuera de RED (hashes verificados)
- Calidad: golangci-lint ✓ · jscpd 0,0 % ✓ · cobertura 84 % (umbral 75)
- Seguridad (`audit --diff`): 0 confirmados · 1 needs_validation (medium) → pregunta abierta #7
- E2E: 5/6 escenarios · 1 no aplicable (sin UI)

## Pendiente / fuera de alcance
- Invalidación de enlaces anteriores (fuera de alcance, §10 de la spec)
```

### 9.2 Reglas de la entrega

- **Trazable**: cada fila de la tabla enlaza a un commit real y a un test real que existe en el árbol.
- **Honesta**: las puertas `Skipped` aparecen como ⚠, nunca como ✓. Las preguntas sin responder aparecen como pendientes.
- **Completa**: incluye lo que **no** se hizo (fuera de alcance, escenarios rechazados) para que el revisor no lo busque.
- **Reutilizable**: `PR_BODY.md` sigue la plantilla del repo si existe (`.github/pull_request_template.md`).

---

## 10. P4 + P5 — Go: Clean Architecture, SOLID y artesanía

### 10.1 Clean Architecture

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `domain/spec.go:6,54-66`, `state.go:7,94-118`, `seal.go`, `project.go:33-96`, `agent_memory.go:28-96`, `context.go:59-82`, `diagnostic.go:159-182` | El dominio importa `os`/`filepath`/`io`, hace I/O y escribe en stderr. | Dominio = funciones puras. Persistencia tras `StateRepository`, `SpecRepository`, `LessonsRepository`, `ProjectDetector`. Presentación en `ui/`. |
| 🔴 | `loop.go:44-309` | `runLoop` (265 líneas) **es** la capa de aplicación. | `internal/app/tddloop.Service` con `runRed/runGreen/runRefactor/review`; `cmd/` solo flags. |
| 🟠 | `loop.go:157-172`, `interview.go:182-187`, `audit.go:78-99`, `e2e.go:101`… | Cada comando instancia adaptadores; `if claude else gemini` ×3; `--agent copilot` ⇒ Gemini. | `cmd/wire.go` con `Deps`; `AgentName` tipado; registro. |
| 🟠 | `ports/compiler.go:20`, `ports/security.go:18` | Compiler depende de Agent (`GenerateTestStubs(runner…)`). | Prompts al app layer; runner por constructor. |
| 🟠 | `ports/*` | Falta `CommandRunner`; `winCmd`+`runProcessWithPipes` copiados byte a byte en dos paquetes. | `adapters/exec` inyectado en todos. |
| 🟠 | `ports/mainframe.go`, `ports/agent.go` | Puerto sin implementación; `Temperature/Debug` nunca leídos; `Project/Location` son GCP. | Borrar; interfaces donde se consumen. |
| 🟡 | `diagnostic.go:21-156` | Diagnóstico por substring del mensaje **en español**. | Errores centinela/tipados + `errors.Is/As`. |

### 10.2 SOLID

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🟠 | `state.go:48-75` | `LoopState` y `TDDState` en el **mismo fichero** con esquemas incompatibles; `LoopState` sin llamadores. | Borrar `LoopState`/`StatePhase`. |
| 🟠 | `spec.go:21-27`, `seal.go:12`, `config.go:6-13` | `BDDFeature` y `CalculateSHA256` muertos; `SDDConfig` God-config con `Mode` documentado `nexus/local/file` y escrito `remote/embedded`. | Borrar; enums con `Validate()`. |
| 🟠 | `project.go:33-88` | First-match: `go.mod`+`package.json` ⇒ Go. Angular → `react.md`. `JavaLegacy` nunca se emite. | Marcadores ordenados + prioridad. |
| 🟠 | `dispatcher.go:66-94`, `linter.go:25`, `quality.go:76-110` | El mismo `switch project.Type` ×4. | `domain.StackProfile{Build, Test(filter), Lint, Gates, TestGlob}`. |
| 🟠 | `agent/claude.go` vs `gemini.go` | ~90 % duplicado. | `cliRunner` table-driven. |
| 🟡 | `cmd/*.go` | 30+ globales de flags; `init()` por fichero. | Options struct por comando. |

### 10.3 Artesanía: errores, durabilidad, concurrencia

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `loop.go:149…296` (11 sitios) | **Todos** los `state.Save` ignorados. | `persist(st) error` y `%w`. |
| 🔴 | `state.go:94-102` | "Atómico" en el comentario; `os.WriteFile` en el código. | tmp + `Sync` + `Rename`. |
| 🔴 | `agent/claude.go:40-41`, `gemini.go:40-45`, `cloudflare_auditor.go:75,89,100` | **Prompt entero como un argumento de argv**: 128 KiB en Linux (`E2BIG`), 32 K en Windows; visible en `ps`. | Por `stdin` o fichero 0600. |
| 🔴 | `storage/logger.go:94-109,83` | Todo nivel al fichero 0644: prompts con código fuente persistidos en **cada** ejecución, sin rotación. | `slog` + lumberjack; `--trace-io` opt-in; 0600. |
| 🔴 | `system/path.go:40-50,73,83` | Inyección por `'` en PowerShell y por `"`/`$(` en rc files. | Eliminar (§7). |
| 🟠 | Todos los `cmd/*.go` | `context.Background()` en todas partes; Ctrl-C deja huérfanos `gemini`/`mvn`. | `signal.NotifyContext`; `cmd.Context()`; `--step-timeout`; `WaitDelay`. |
| 🟠 | `setup.go:49-51`, `consistency.go:26-33`, `build_cmd.go:20` | `os.Exit` en `RunE`; `deployScaffold` devuelve `nil` aunque fallen todos los `WriteFile` y luego imprime "✓". `setup.go:97-104` extrae un asset **inexistente** y también imprime "✓". | Errores tipados; **nunca ✓ tras un warning**. |
| 🟠 | `setup.go:47-52`, `main.go:13-18`, `audit.go:64,40` | Solo `master` protegido; `-xyz`→`--xyz` rompe pflag; `--full --diff` ambos true; `--fail-on hgih` no casa nada ⇒ pasa. | `main`; borrar; `MarkFlagsMutuallyExclusive`; enum en parse. |
| 🟠 | `dispatcher.go:75,86` | `python -m py_compile` **sin ficheros** ⇒ exit 0 sin compilar. `--run` es solo Vitest. | `compileall`; detectar runner. |
| 🟡 | `audit.go:121`, `loop.go:208…` | Errores con mayúscula y emoji. | Minúscula; decorar en `ui/`. |

### 10.4 Tests

Cobertura: **38,4 %** · `cmd` 26 % · `domain` 60 % · `agent`/`auth`/`security`/`system` **0 %** · `quality` falla en este host.

| Sev | Problema | Mejora |
| :---: | :--- | :--- |
| 🔴 | `loop_test.go`, `setup_test.go` ejecutan el `rootCmd` **global** con `os.Chdir` y flags que se heredan entre tests. El producto (RED→GREEN→REFACTOR) **no tiene un solo test** con agente/compilador falsos. | Constructores con deps; test de tabla sobre compilador scripted cubriendo: RED compila-y-pasa, RED no compila, RED sin cambios, GREEN con test editado, GREEN agota reintentos, REFACTOR con gate Skipped, agente devuelve `needs_clarification` en cada fase. |
| 🟠 | Tests que ejecutan `npx`, `golangci-lint`, `markitdown` reales; tres tests **no asertan nada**. | `CommandRunner` falso con fixtures; reales tras `-tags integration`. |
| 🟠 | Aserciones sobre strings de UI en español. | `errors.Is`. |
| 🟠 | Sin golden, sin fuzz, sin `-race`, sin umbral. | `testdata/specs/*.md`; `FuzzParseScenarios`; CI con umbral creciente. |

### 10.5 Higiene y contrato de CLI

| Sev | Problema | Mejora |
| :---: | :--- | :--- |
| 🔴 | **71/71 ficheros CRLF**; `gofmt -l` lista todos. | `.gitattributes`; `gofmt -w`; CI. |
| 🟠 | `chromedp` `// indirect` siendo directo. (`go 1.26` **es** necesario: lo exigen `chromedp v0.16`, `cdproto` y `x/oauth2`; el README debe leerlo de `go.mod`, no fijar "1.24+".) | `go mod tidy`; CI con `tidy -diff`. |
| 🟠 | `--FailOn`/`--DryRun`; `flagConsistencyFailOn` nunca leído; stdout mezcla datos y banners; sin `--json`/`--quiet`; exit codes sin contrato. | Kebab-case; stderr/stdout; `--output json`; códigos: 0 ok · 1 uso · 2 gate · 3 integridad · 4 toolchain · **5 pregunta pendiente**. |
| 🟠 | Cinco nombres (`specforge`/`sdd`/`forge`/`SDD-Free`/`SDDFramework`), dos rutas de config, una guía referenciada que no existe. | Uno de cada. |
| 🟡 | `setup` lee stdin sin TTY check (CI se cuelga); `--location` default anula la detección; `--recursive` solo se apaga con `=false`. | `term.IsTerminal` + `--non-interactive`; `Changed`; `--no-recursive`. |

---

## 11. Flujo y arquitectura objetivo

### 11.1 El ciclo completo con sus puertas

```text
specforge init                        agent, language. Nada más.
specforge setup                       bloque gestionado en CLAUDE.md/GEMINI.md · .specify/config.yaml · specs/

specforge spec new "Password reset"   NNNN asignado · spec.md desde plantilla §5.4
specforge spec interview NNNN         bucle por turnos · una pregunta por turno · interview.jsonl
specforge spec clarify NNNN           resuelve [NEEDS CLARIFICATION] uno a uno → decisions.md
specforge spec lint NNNN              §5.4 en verde
specforge spec approve NNNN           ── R0 ── humano · sello · aprobador

specforge plan NNNN                   una llamada → plan.md (puede preguntar)
                                      ── R1 ── humano aprueba el plan

specforge loop NNNN [--resume] [--scenario k] [--review every|3|end]
  por escenario:
    RED       agente escribe test [SDD-NNNN-k] ─┐ puede responder needs_clarification ── R3 ──
              compila · solo ese test · falla por aserción · hash de tests · git status
    GREEN     prompt = reglas + plan + glosario/INV + escenario + test + decisions + tail(error)
              solo ese test · hashes intactos · ≤ maxRetries
    REFACTOR  suite completa · gates (tri-estado, umbrales de config) · hashes intactos
              lección escrita por el agente → lessons.md (dedupe)
    REVIEW    ── R2 ── humano: aceptar / cambiar / rechazar · commit feat(SDD-NNNN-k)

specforge spec amend NNNN "…"         delta ADDED/MODIFIED/REMOVED · re-sello · invalida escenarios afectados
specforge e2e NNNN --url …            por escenario · Then verificado en Go · report.json
specforge audit --diff                fail-closed · needs_validation → R3
specforge deliver NNNN                ── R4 ── DELIVERY.md · trace.json · PR_BODY.md
```

### 11.2 Árbol objetivo

```text
cmd/specforge/main.go                 NotifyContext · buildinfo · mapa de exit codes
internal/
  app/                                casos de uso (lo que hoy vive en cmd/)
    interview/  spec/  plan/  tddloop/  deliver/  audit/  e2e/
    protocol/   response.go            el contrato §3.1: done | needs_clarification | blocked
    review/     gates.go               R0-R4
  domain/                             PURO
    spec/       (gherkin oficial) seal.go lint.go sections.go
    tdd/        state.go outcome.go errors.go
    stack/      profile.go detect.go
    quality/    result.go thresholds.go
    delivery/   report.go trace.go
  ports/        agent exec state_repo spec_repo vcs logger prompter(UI para R3)
  adapters/
    agent/      cli_runner.go (claude, gemini; stdin; JSON)
    exec/       runner.go
    vcs/        git.go
    storage/    state_fs.go(atómico) spec_fs.go lessons_fs.go decisions_fs.go config.go
    quality/    jscpd knip stryker linter archunit (JSON reporters)
    testparse/  gojson.go surefire.go vitest.go junit.go
    e2e/        chromedp_driver.go vision_agent.go
    security/   auditor.go (jsonschema)
  ui/           spinner box prompt messages_{es,en}
assets/
  prompts/{es,en}/   interview plan red green refactor vision audit/*  (text/template, todos con el contrato §3.1)
  templates/         spec.md plan.md delivery.md pr_body.md
  baseline/{go,react,java,python}/
contrib/bank/       tekton nexus consistency adc path   (fuera del core)
```

---

## 12. Roadmap por fases

Cada fase se cierra con una *definición de hecho* y una **comprobación de principios**: ¿qué principios avanza y cómo se demuestra?

### Fase 0 — Higiene y red de seguridad (1 semana) · P5, P6

- [x] LF + `.gitattributes`; `gofmt -w`; `go mod tidy` (`go 1.26` se mantiene: lo exigen las dependencias). → PR #2
- [x] CI: `gofmt -l`, `go vet`, `tidy -diff`, `go test -race -cover` con umbral 35 % (el real sin tests de integración; no bajar), build cross-platform. → PR #2
- [ ] `golangci-lint` en CI (errcheck, funlen, gocyclo, goconst, revive) con `.golangci.yml`.
- [x] Tests con herramientas externas → `-tags integration`; los que no asertaban ahora asertan. → PR #2
- [x] `internal/buildinfo` con `-X`; `Makefile` + `.goreleaser.yaml`; borrados los `3.0.0` hardcodeados y los 4 build scripts. → PR #2
- [x] Corregidas las afirmaciones falsas de USER_GUIDE y la referencia a `GUIA_DE_USO_V3.md`. → PR #2 (el README ya se corrigió en PR #1)
- **Hecho cuando:** CI verde en Linux/macOS/Windows en un clon limpio. **Principios:** P5 medible (lint en verde), P6 (goreleaser en vez de 4 scripts).

### Fase 1 — Que las puertas se nieguen y la IA pregunte (3 semanas) · P1, P3, P5

- [ ] **Protocolo §3.1** en `internal/app/protocol`; todos los prompts terminan con el contrato; runtime gestiona `done`/`needs_clarification`/`blocked`; `decisions.md`; exit code 5.
- [ ] Bloque gestionado en `CLAUDE.md`/`GEMINI.md` con la regla §3.3; `setup` lo escribe; `-p` solo lleva la tarea.
- [ ] `ports.CommandRunner` + `adapters/exec`; borrar duplicados; prompt por stdin; `cliRunner` único; streaming; spinner con TTY.
- [ ] `signal.NotifyContext`, `cmd.Context()`, `--step-timeout`; chromedp con ctx.
- [ ] `TestOutcome` por stack vía formatos JSON/XML; RED = falla por aserción; `git status` antes/después; `files_written` verificado.
- [ ] Hash de tests entre fases + `TEST TAMPERING`.
- [ ] `LastError` al primer GREEN; `maxGreenRetries`.
- [ ] Gates tri-estado con JSON reporters; umbrales en config; borrar SonarGate.
- [ ] Auditor fail-closed; `--end-of-options`; jsonschema real; `needs_validation` → R3.
- [ ] `state.Save` atómico y propagado; config 0600; `slog` + lumberjack; `--trace-io`.
- [ ] Parser: `cucumber/gherkin/go` sobre bloques ```` ```gherkin ````; `ErrNoScenarios`; golden + fuzz.
- **Hecho cuando:** un test de aplicación con agente y compilador falsos cubre los 7 casos de §10.4, **incluido "el agente pregunta en cada fase y el runtime pausa"**; `cmd` ≥ 60 %. **Principios:** P1 demostrable (test), P3 (cero `return true` sin ejecutar), P5 (cero `_ =` en escrituras).

### Fase 2 — Spec clara, plan y revisión (3 semanas) · P2, P7, P4

- [ ] `internal/app` + composition root; `runLoop` → `tddloop.Service`; dominio sin I/O.
- [ ] Plantilla §5.4 y `spec lint`; `spec new` (NNNN); `spec approve` (R0); `interview` no sella.
- [ ] `plan` + R1; `PromptContext`; prompts como `text/template` en `{es,en}`.
- [ ] R2 por escenario con `--review`; commit por escenario; `--scenario k --from green`.
- [ ] Sello unificado y normalizado; `--resume` revalida y diffea; `spec clarify`; `spec amend` con deltas.
- [ ] Marcador en tests + ejecución filtrada.
- [ ] Lecciones escritas por el agente, dedupe, tope, scope; `lessons.md` versionado; glosario desde la spec.
- **Hecho cuando:** el flujo `new → interview → clarify → lint → approve → plan → loop (con R2) → amend → loop --resume` funciona en un repo Go de ejemplo, grabado. **Principios:** P2 (cada R0-R3 queda en el estado), P7 (lint en verde, lecciones ≤ 30), P4 (`go list -deps ./internal/domain` sin `os`).

### Fase 3 — Entrega y recorte (2 semanas) · P8, P6

- [ ] `deliver`: `DELIVERY.md`, `trace.json`, `PR_BODY.md` desde artefactos; plantilla de PR del repo si existe.
- [ ] `contrib/bank`: Tekton, Nexus, `consistency`, ADC, PATH. Config = `{agent, language}`.
- [ ] Borrar `build`, `doc` (→ flag), alias raíz, `LoopState`, `BDDFeature`, `MainframeConfigurator`, `openspec/config.yaml`.
- [ ] Contrato CLI: kebab-case, stderr/stdout, `--json`, `--quiet`, exit codes, `--non-interactive`.
- [ ] `os.UserConfigDir()`, un nombre, godoc en inglés.
- **Hecho cuando:** `grep -ri "nexus\|tekton\|as400\|jakarta\|vertex" internal cmd assets` = 0 fuera de `contrib/`; 8 comandos; `deliver` produce un `DELIVERY.md` en el que cada fila enlaza a un commit real. **Principios:** P8 verificable, P6 medible.

### Fase 4 — Entrevista por turnos y E2E real (3-4 semanas) · P1, P2, P3

- [ ] `spec interview` con bucle propiedad de la herramienta, contrato por turno, `interview.jsonl`, libro de incógnitas, replay.
- [ ] E2E por escenario con `then_index`, `evidence` verificada en Go, acciones validadas, `report.json`, `--min-pass-rate`.
- [ ] Demo grabada; v4.0.0.
- **Hecho cuando:** un mantenedor de spec-kit podría leer el README y no encontrar una afirmación que no pueda reproducir.

---

## 13. Métricas: hoy → v4

| Métrica | Hoy | v4 | Principio |
| :--- | :---: | :---: | :---: |
| Fases en las que el agente puede preguntar | 1 de 7 | 7 de 7 | P1 |
| Puertas de revisión humana registradas | 0 | 5 (R0-R4) | P2 |
| Puertas que aprueban sin ejecutarse | 5 | 0 | P3 |
| Falsos "YAGNI" / falsos "superado" reproducibles | 4 | 0 | P3 |
| Imports de `os`/`exec`/`filepath` en `domain` | 7 ficheros | 0 | P4 |
| Funciones > 60 líneas | 12 | 0 | P4, P5 |
| `_ =` sobre escrituras | ~25 | 0 | P5 |
| Cobertura total / `cmd` / adaptadores exec | 38 / 26 / 0 % | ≥ 75 / 70 / 60 % | P5 |
| Parsers/matchers/loggers caseros con librería madura disponible | 6 | 0 | P6 |
| Comandos del CLI | 11 | 8 | P6 |
| Secciones obligatorias de spec con lint | 0 | 12 | P7 |
| Tamaño máximo de memoria inyectada | ilimitado | 30 lecciones + decisiones de la spec | P7 |
| Artefactos de entrega | 0 | 3 (`DELIVERY.md`, `trace.json`, `PR_BODY.md`) | P8 |
| Referencias al banco en el core | ~40 % | 0 | P6 |

---

## 14. Las diez cosas que haría mañana, en orden

1. **Protocolo `done | needs_clarification | blocked`** en todos los prompts y el runtime que pausa y pregunta (§3). Es P1 hecho mecanismo y son ~150 líneas. Sin esto, todo lo demás sigue inventando.
2. **Hash de tests entre fases** + regla en el bloque gestionado (`loop.go`). 40 líneas.
3. **`TestOutcome` y RED "falla por aserción"** (`dispatcher.go`, `loop.go:199-254`). Mata los falsos YAGNI.
4. **Gates tri-estado** y borrar SonarGate (`quality/*`).
5. **Auditor fail-closed** + `--end-of-options` (`cloudflare_auditor.go:108-114,134`).
6. **Prompt por stdin** y `cliRunner` único (`agent/*`). Sin esto no funciona en proyectos reales.
7. **`state.Save` atómico y propagado** (`state.go`, `loop.go`).
8. **`cucumber/gherkin/go`** en lugar del parser casero (`spec.go`). Una tarde y desaparecen tres bugs ✔.
9. **`spec approve`** (R0) y que `interview` no selle (`interview.go:194-212`).
10. **LF + `go mod tidy` + CI + goreleaser**. Media hora, y nada vuelve a entrar roto.

---

*Generado a partir de la lectura completa del repositorio en `main` (commit `71334e4`), ejecución de `go build`, `go vet`, `go test -cover`, y reproducción manual de los fallos marcados con ✔. Los ocho principios de §0 son la vara con la que debe medirse cualquier PR futura, incluidas las que salgan de este plan.*
