# SpecForge — Plan maestro de mejora (v3.0.0 → v4)

> Documento único que reúne la auditoría completa del repositorio, los principios que gobiernan cada decisión, el diseño objetivo, el roadmap por fases con su estado de ejecución y los anexos de reproducción. Sustituye a las dos versiones anteriores del plan.
>
> **Base de la auditoría:** lectura íntegra del código Go (5.628 líneas de producción, 1.242 de tests), de los assets embebidos, prompts y guía; `go build`, `go vet`, `go test -cover`; y reproducción manual con el binario compilado de los fallos marcados con ✔ (Anexo A).
>
> **Severidad:** 🔴 bloqueante = rompe la promesa central · 🟠 mayor = brecha metodológica o de diseño · 🟡 menor = pulido.
> **Estado:** ✅ hecho · 🔄 en PR · ⬜ pendiente.
>
> **Estado final (Fases 0-4 ejecutadas):** las cinco fases están implementadas en PRs apiladas #2 → #3 → #4 → #5 → #6, con CI en verde y verificadas con ejecuciones reales de Claude Code (`docs/DEMO.md`). Desviaciones del plan, cada una anotada en su fase: el código de banco se borró en lugar de moverse a `contrib/bank`; no hay `spec amend` (enmendar = editar + `spec approve`, con delta registrado); el E2E por escenario se adelantó a la Fase 1. Queda a decisión del mantenedor publicar v4.0.0. Las secciones 1-16 describen el estado de partida (v3) y se conservan como auditoría.

---

## Índice

0. [Los ocho principios](#0-los-ocho-principios-que-gobiernan-este-plan)
1. [Veredicto en una página](#1-veredicto-en-una-página)
2. [Lo que está bien](#2-lo-que-está-bien-y-hay-que-proteger)
3. [P1 · Pregunta, no inventes, como mecanismo](#3-p1--pregunta-no-inventes-como-mecanismo-no-como-frase)
4. [P2 · El bucle de revisión del desarrollador](#4-p2--el-bucle-de-revisión-del-desarrollador)
5. [Idea de negocio y posicionamiento](#5-idea-de-negocio-y-posicionamiento)
6. [Spec clara: petición, contenido y ciclo de vida](#6-spec-clara-petición-contenido-y-ciclo-de-vida)
7. [P3 · Todo valida: loop TDD, puertas, auditor, E2E](#7-p3--todo-valida-bucle-tdd-puertas-auditor-e2e)
8. [Prompts](#8-prompts)
9. [P7 · Memoria útil](#9-p7--memoria-útil)
10. [P8 · Entrega clara](#10-p8--entrega-clara)
11. [P6 · No reinventar la rueda](#11-p6--no-reinventar-la-rueda)
12. [P4 + P5 · Go: arquitectura, SOLID, artesanía](#12-p4--p5--go-clean-architecture-solid-y-artesanía)
13. [Auditoría por paquete (adaptadores)](#13-auditoría-por-paquete-adaptadores)
14. [Tests](#14-tests)
15. [Idioms, contrato de CLI, dependencias y build](#15-idioms-contrato-de-cli-dependencias-y-build)
16. [Flujo y arquitectura objetivo](#16-flujo-y-arquitectura-objetivo)
17. [Roadmap por fases y estado](#17-roadmap-por-fases-y-estado)
18. [Métricas hoy → v4](#18-métricas-hoy--v4)
19. [Las diez cosas que haría mañana](#19-las-diez-cosas-que-haría-mañana-en-orden)
- [Anexo A · Reproducción de los bloqueantes](#anexo-a--reproducción-de-los-bloqueantes-verificados)
- [Anexo B · Índice de hallazgos por fichero](#anexo-b--índice-de-hallazgos-por-fichero)
- [Anexo C · Estado de PRs](#anexo-c--estado-de-prs)

---

## 0. Los ocho principios que gobiernan este plan

Cada decisión del plan se justifica con uno de estos ocho principios y cada fase del roadmap se cierra comprobándolos. Si una mejora no sirve a ninguno, sobra.

| # | Principio | Qué significa en SpecForge | Cómo se comprueba |
| :-: | :--- | :--- | :--- |
| **P1** | **Pregunta, no inventes** | Ante cualquier duda funcional, técnica o de contexto, el agente **devuelve una pregunta**, SpecForge **pausa** y se la hace al desarrollador. En **todas** las fases, no solo en la entrevista. Nunca se rellena un hueco con una suposición. | Todo prompt admite `{"status":"needs_clarification"}` y el runtime lo gestiona. Cero suposiciones silenciosas. |
| **P2** | **Iteración con revisión del desarrollador** | Ciclos cortos con un punto de revisión humano al final de cada uno: spec, plan, cada escenario, entrega. SpecForge recuerda la decisión. | Cada puerta existe como comando (`approve`) o pausa interactiva, y queda en el estado. |
| **P3** | **Todo valida, nada aprueba por defecto** | Una puerta que no pudo ejecutarse **no** aprueba. Un JSON ilegible **no** es un informe vacío. Un test que no compila **no** es RED. Fail-closed siempre. | Ningún `return true` sin haber ejecutado la comprobación. Tri-estado `Passed/Failed/Skipped`; `--strict` convierte `Skipped` en fallo. |
| **P4** | **Clean Architecture + SOLID de verdad** | Dominio puro (sin `os`, `exec`, `filepath`), casos de uso en `internal/app`, puertos definidos por quien los consume, adaptadores finos, una razón de cambio por tipo. SpecForge exige esto al código del usuario: debe cumplirlo él primero. | `go list -deps ./internal/domain` sin `os`/`exec`. Ninguna función > 60 líneas. Composition root único. |
| **P5** | **Código artesano** | Nombres que dicen lo que hacen, errores tipados, sin `_ =` sobre escrituras, sin números mágicos, tests que describen comportamiento, godoc en inglés. | `golangci-lint` con `errcheck`, `gocyclo`, `funlen`, `goconst`, `revive` en verde. |
| **P6** | **Fácil y sin reinventar la rueda** | Si existe una librería madura, se usa. Menos comandos, menos ficheros de memoria, menos modos. Un nombre, una ruta de config, un idioma de código. | Tabla §11. CLI de 11 a 8 comandos. |
| **P7** | **Specs claras y memoria útil** | La spec tiene todo lo que hace falta para no preguntar lo obvio; lo que no está, está marcado como abierto. La memoria es corta, curada y relevante; nunca crece sin control. | Lint de spec en verde; `lessons.md` ≤ 30 entradas con dedupe; solo se inyecta lo pertinente. |
| **P8** | **Entrega clara** | Al terminar una feature, el desarrollador recibe un paquete que se explica solo y en el que nada es inventado: cada línea sale de un artefacto. | `specforge deliver NNNN` produce `DELIVERY.md` + `PR_BODY.md` + `trace.json`. |

---

## 1. Veredicto en una página

**La idea es muy buena y el hueco existe.** spec-kit y OpenSpec son *paquetes de prompts* que viven dentro del agente: el agente se corrige los deberes a sí mismo. Kiro tiene specs → tareas pero sin puerta externa. Tessl es spec-as-source para librerías. SpecForge es el único que es un **runtime externo y determinista** que ejecuta el compilador y se niega. "CI del proceso, no solo del resultado" es la frase correcta.

**La implementación aún no cumple la promesa.** Medida contra los ocho principios:

| Promesa | Realidad | Principio |
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
| Prompt al agente | **Entero como un argumento de argv**: 128 KiB en Linux, 32 K en Windows, visible en `ps`. `agent/claude.go:40-41` | P3, P5 |
| Memoria que aprende | Append ciego de trazas truncadas con solución hardcodeada; todo se reinyecta en cada prompt. `agent_memory.go:65-96` | P7 |
| Entrega | No existe. El loop termina con un banner. | **P8** |
| Herramienta genérica | ~40 % es el banco: ADC de GCP, Tekton, Nexus, AS/400, javax→jakarta, PATH por PowerShell. | P6 |
| Higiene | 71/71 ficheros CRLF, `chromedp` indirecto, 10+ "3.0.0" fijos, 4 scripts de build, sin CI. | P5 |

**Estado de ejecución:** la higiene (última fila) está resuelta en PR #2 (Fase 0 ✅). Todo lo demás es Fase 1-4.

**Qué hacer:** no reescribir. La hexagonal es real y el esqueleto del dominio es bueno. Hay que (1) convertir "pregunta, no inventes" en un mecanismo del runtime en todas las fases, (2) poner puertas de revisión humana donde faltan, (3) hacer que todas las puertas automáticas fallen cerradas, (4) completar el ciclo de vida de la spec, (5) definir la entrega y (6) hacer el Go digno de lo que exige a los demás. En ese orden.

---

## 2. Lo que está bien (y hay que proteger)

- **El posicionamiento**: puerta externa determinista sobre el agente. Único.
- **`cmd/ → domain / ports / adapters`** existe de verdad; los puertos son pequeños.
- **`TDDState` + `AdvanceToNextPhase`** (`state.go:140-158`): modelo simple y correcto.
- **El sello SHA-256 con limpieza de la línea del sello** (`spec.go:41-51`): idea correcta, le falta normalización.
- **La cadena recon → hunt → validate** con contratos JSON (`cloudflare_auditor.go`): el prompt mejor diseñado del repo.
- **Binario único con `go:embed`**: distribución impecable.
- **La caja de diagnóstico** (`diagnostic.go`): UX que nadie más tiene; solo necesita errores tipados.
- **`[NEEDS CLARIFICATION]` como bloqueo duro** en `loop`: es la semilla de P1. Hay que generalizarla.
- **El prompt de vision** (`vision_agent.go:163-224`): JSON estricto, selectores exactos, historial. Buen punto de partida.

---

## 3. P1 — "Pregunta, no inventes" como mecanismo, no como frase

Hoy el principio vive en dos sitios: una regla del prompt de entrevista (`00_interview_bdd.md:27-30`) y un grep de `[NEEDS CLARIFICATION]` al arrancar `loop` (`loop.go:123-141`). En las otras cinco fases el agente **no tiene cómo preguntar**: el prompt le dice "implementa" y él implementa, suponiendo lo que falte.

### 3.1 El protocolo de respuesta universal

Todo prompt termina con el mismo contrato de salida y el runtime lo trata igual en todas las fases:

```json
{ "status": "done",
  "files_written": ["internal/auth/reset_test.go"],
  "summary": "Test for scenario SDD-0001-3 asserting single-use link expiry" }
```
```json
{ "status": "needs_clarification",
  "question": "The spec says the link is valid for 30 minutes. From request time or from email delivery?",
  "blocking": true,
  "options": ["request time", "delivery time"],
  "context": "scenario SDD-0001-3, step Then" }
```
```json
{ "status": "blocked",
  "reason": "pytest is not configured in this repo and the spec requires Python tests",
  "suggested_action": "run `specforge setup --stack python` or add pytest to pyproject" }
```

| Estado | Runtime | Registro |
| :--- | :--- | :--- |
| `done` | Verifica `files_written` contra `git status` (si no coincide ⇒ error). Sigue con la puerta de la fase. | Checkpoint con ficheros y resumen. |
| `needs_clarification` | **Pausa.** Imprime pregunta, contexto y opciones. En TTY pregunta y reanuda con la respuesta añadida al prompt. Sin TTY (CI): exit 5 y la pregunta en `specs/NNNN/questions.md`. `blocking:false` ⇒ anota y sigue. | Pregunta **y respuesta** en `specs/NNNN/decisions.md` (fecha, fase, escenario). Si la respuesta cambia la spec, se enruta a `spec clarify`. |
| `blocked` | Para. Caja de diagnóstico con `suggested_action`. | Checkpoint `BLOCKED`. |
| no parsea | Un reintento con "responde solo el JSON". Segundo fallo ⇒ `blocked`. **Nunca** prosa = éxito. | Salida cruda bajo `--trace-io`. |

### 3.2 Dónde se aplica

| Fase | Hoy | Objetivo |
| :--- | :--- | :--- |
| Entrevista | El agente puede escribir `[NEEDS CLARIFICATION]` si se acuerda. | Bucle por turnos propiedad de la herramienta (§6.1). Cada turno **es** una pregunta. Finaliza solo con `unknowns_remaining == []`. |
| Plan | No existe. | `needs_clarification` para decisiones de arquitectura. Se responde una vez, queda en `decisions.md`. |
| RED | Inventa fichero, framework, imports. | "No hay fichero de test para `auth`; ¿creo `auth_test.go` o `auth/reset_test.go`?" |
| GREEN | Inventa lo que la spec no dice. | Pregunta por reglas de negocio ausentes (el ejemplo real de los 30 minutos). |
| REFACTOR | Un intento ciego. | Puede preguntar si una violación de calidad choca con la spec. |
| E2E | Alucina selectores hasta agotar 15 pasos. | Pregunta cuando no encuentra lo que el `Then` describe. |
| Audit | — | Un `needs_validation` es una pregunta al desarrollador. |

### 3.3 La regla que lo sostiene

En el bloque gestionado de `CLAUDE.md`/`GEMINI.md` que `setup` escribe una vez:

> You are working under SpecForge. If anything needed to complete the current task is not in the spec, the plan, the existing code or the decisions log, **do not assume it**: answer with `status: needs_clarification` and the exact question. A wrong guess costs more than a question. Never edit test files during GREEN or REFACTOR. Never claim to have written a file you did not write.

---

## 4. P2 — El bucle de revisión del desarrollador

El flujo actual tiene **una** interacción humana (la entrevista) y luego corre solo. Un desarrollador que revisa ocho escenarios de golpe no revisa: acepta.

### 4.1 Puertas de revisión

| Puerta | Cuándo | Qué revisa | Registro | Hoy |
| :--- | :--- | :--- | :--- | :---: |
| **R0 · Alcance** | Fin de la entrevista | Intención, fuera de alcance, invariantes. | `spec approve` → `approved_by`, `approved_at`, sello. | ✗ |
| **R1 · Plan** | Tras `plan` | Dónde va el código, test por escenario, puertos. Barato aquí, carísimo después. | `plan approve` (o `--auto-approve-plan`). | ✗ |
| **R2 · Escenario** | Tras cada REFACTOR verde | Diff acotado a `files_written`, test con marcador, informe de puertas. Aceptar · pedir cambio (vuelve a GREEN con comentario) · rechazar (vuelve a RED). | Checkpoint `REVIEWED`. `review: every-scenario \| every-N \| end`. | ✗ |
| **R3 · Preguntas** | `needs_clarification` | La pregunta. | `decisions.md`. | ✗ |
| **R4 · Entrega** | `deliver` | `DELIVERY.md` y cuerpo de PR. | Commit / PR. | ✗ |

### 4.2 Iteración corta por diseño

- **Un escenario, un ciclo, un commit** `feat(SDD-0001-3): <título>` con los `files_written`. El historial de git **es** la traza. Hoy el loop no commitea y `files_written` ni se conoce.
- **Intervención a mitad**: `loop --scenario 3 --from green`. Hoy `--resume` es todo o nada.
- **Las decisiones no se repreguntan**: `decisions.md` se inyecta antes de cada prompt.
- **El tamaño del paso lo marca la spec**: un `Scenario Outline` con 6 `Examples` son 6 pasos (hoy no se expande, §6.3).

---

## 5. Idea de negocio y posicionamiento

### 5.1 ¿Para quién es?

| El código dice | El README dice |
| :--- | :--- |
| Equipo Java/Spring hispanohablante en un banco: OpenShift + Tekton (`setup.go:138-186`, `standards/ci-tekton.md`), Nexus corporativo (`openspec/config.yaml`, `init.go:127-155`, `scaffold/java/.mvn/settings.xml`), driver `jt400` de AS/400 + Oracle + MySQL en el `pom.xml` del scaffold, Vertex AI con ADC (`init.go:68-89`), portátiles Windows (`console_windows.go`, `diagnostic.go:114`), gate javax→jakarta (`consistency.go`). | Cualquier desarrollador que use Claude Code o Gemini CLI. |

Son dos productos. Hay que elegir el segundo y mover el primero a `contrib/`.

### 5.2 Cortar o mover a `contrib/bank`

| Pieza | Por qué sobra en el core | Acción |
| :--- | :--- | :--- |
| Inyección en PATH / registro `HKCU` (`system/path.go`) | Un CLI que edita el registro y tus dotfiles **sin preguntar** (`init.go:178`) es bandera roja OSS. Además es inyectable (§13.10). | Eliminar. Homebrew / Scoop / `go install` / goreleaser. |
| ADC de GCP, `UseVertexAI`, `Project/Location` en `ports.AgentOptions` | Los agentes se autentican solos. | `config = { agent, language }`. |
| `values/` Tekton en `setup` | Política de un empleador. | `contrib/bank`. |
| `consistency` (javax→jakarta, Java < 17, `answers.json`) | Política de un empleador y **inalcanzable**: compara con `"java-legacy"`, valor que `DetectProject` nunca emite (`project.go:14,33-88` vs `consistency.go:99`). | `contrib/bank` o borrar. |
| MarkItDown auto-convert en `interview`/`ingest` | Shell-out a Python en un producto que se vendía "zero deps". | Flag `--docs` o borrar. |
| SonarGate, modo Nexus, `build --dry-run` (imprime un string, `build_cmd.go:18-21`), `version` "telemetry" | Stubs que aparentan funcionalidad. | Borrar. |
| `openspec/config.yaml` (referencia `sdd.ps1`, `/opsx:propose`) | No existen en este repo. | Borrar. |
| `scaffold/react/README.md:26`, `scaffold/java/README.md:25-31` | Hablan de comandos `/opsx` inexistentes. | Reescribir. |
| Cinco nombres (`specforge`/`sdd`/`forge`/`SDD-Free`/`SDDFramework`), dos rutas de config | Los alias en el comando raíz **no funcionan en Cobra** (`root.go:19`). | Un nombre, `os.UserConfigDir()/specforge`. ✅ parcialmente en PR #2 (`~/.sdd` → `~/.specforge`). |

### 5.3 Las tres funciones que darían envidia a spec-kit

1. **Entrevista por turnos propiedad de la herramienta**, con libro de incógnitas persistido y puerta `approve`. El `/clarify` de spec-kit es una pasada de 5 preguntas dentro del agente.
2. **RED/GREEN a prueba de trampas con trazabilidad**: hash de tests entre fases, marcador de escenario, selección por escenario, "falla por la razón correcta", y `specforge trace`. Nadie lo tiene.
3. **Deltas de spec con estado reanudable**: `spec amend` con ADDED/MODIFIED/REMOVED, invalidación solo de los escenarios afectados, `--resume` desde ahí. OpenSpec tiene deltas sin runtime; spec-kit, plantillas sin runtime.

### 5.4 Decisiones de producto

- **Idioma**: prompts, salida, comentarios y errores en español; README/guía en inglés; `persona.md:8` exige identificadores en inglés; `openspec/config.yaml:6` fuerza "SIEMPRE en español". El agente genera Gherkin en español que el parser mutila y código en inglés. → Código, godoc y errores en inglés; salida y prompts con `language:` en config y plantillas en `assets/prompts/{es,en}/`.
- **Versionado**: "v3.0.0" en un repo de 2 commits. ✅ PR #2: `internal/buildinfo` con `-X`. v4.0.0 cuando el core cumpla la promesa.

---

## 6. Spec clara: petición, contenido y ciclo de vida

### 6.1 Petición de spec (la entrevista)

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `cmd/interview.go:118-189`, `agent/claude.go:104-119`, `gemini.go:117-135` | "Socrático" = un prompt a CLI interactivo. Cero visibilidad: sin turnos, sin incógnitas, sin saber si hubo aprobación (Etapa 3 del prompt es prosa no exigible). | **La herramienta posee el bucle**: cada turno es una llamada headless con contrato `{question, why_it_matters, section, unknowns_remaining[], draft_delta}`. SpecForge pregunta, lee, persiste `interview.jsonl`, finaliza solo con `unknowns_remaining == []`. Una pregunta por turno. |
| 🔴 | `cmd/interview.go:194-212` | Sella **siempre** al terminar, aunque sea Ctrl-C. La plantilla promete GATE 1 "validación explícita del Product Owner" (`spec-template.md:58`) y no existe. Si el agente no escribió, se sella la plantilla con `<estado inicial válido>` y `loop` parsea los placeholders. | `spec approve` (R0): lint, rechaza `<…>`/`NNNN`/`DRAFT`, sello + aprobador. `interview` **nunca** sella. |
| 🟠 | `cmd/interview.go:153,235-253` | Toda spec es `0001-<slug>.md`; `findLatestSpec` elige por *mtime* (un `git checkout` cambia mtimes). | `NNNN` = máximo + 1; `spec_id` en el estado; `--spec` obligatorio si hay más de una. |
| 🟠 | `cmd/interview.go:88-116` | El context pack entero (código: hasta 80 líneas de cada fichero cuyo path contenga "app"/"api"/"index"/"service", `ingest/scanner.go:143-145`) se incrusta en una entrevista de **negocio**. | A la entrevista: árbol + glosario + specs existentes. El código va al `plan`. |
| 🟠 | `00_interview_bdd.md:15-30` | Faltan actores, NFR, contratos de datos (`service-ficha.template.md` los tiene y nadie lo usa), catálogo de errores, supuestos, métricas. Las INV-xx no se trazan a escenarios. | Plantilla §6.4 + lint. |
| 🟡 | `cmd/interview.go:163` | Solo se sustituye el placeholder del título; `NNNN`, `<Nombre / Rol>`, `<YYYY-MM-DD>`, `<TICKET-XXXX>` quedan. `:135` envuelve la feature en comillas literales. `:169` ruta absoluta con backslashes en el prompt. | `text/template` con datos reales. |
| 🟡 | `cmd/interview.go:220-233` | `sanitizeSlug` tira acentos: `Liquidación` → `liquidacin` (y `interview_test.go:16` lo da por bueno). | NFD + quitar `Mn` → `liquidacion`. |
| 🟡 | `agent/claude.go:98-102` | La memoria de seis ficheros (ArchUnit, SQL, IBAN) se inyecta también en la entrevista de negocio. | Memoria por fase (§9). |

### 6.2 Ciclo de vida (lo que OpenSpec tiene y SpecForge no)

| Sev | Problema | Mejora |
| :---: | :--- | :--- |
| 🟠 | No hay `clarify`. Los marcadores se resuelven editando a mano (`loop.go:135-136`), lo que **rompe el sello** que el mismo comando exige (`loop.go:118-121`). La salida sugiere re-ejecutar `interview` (`diagnostic.go:87`), que reinicia todo y re-sella a ciegas. | `spec clarify NNNN`: itera marcadores, pregunta uno a uno (P1), parchea, re-sella con `supersedes:`. Respuestas en `decisions.md`. |
| 🟠 | No hay gestión de cambios: un hash y un error. `VerifySpecIntegrity` dice "Genera un nuevo sello" sin comando para hacerlo. | `spec amend NNNN "…"` → `changes/0002-<slug>.md` con ADDED/MODIFIED/REMOVED por escenario, re-sello, reset del estado **solo** para escenarios afectados (match por hash del título). Pie `seal-history`. |
| 🟠 | `--resume` (`loop.go:74-90`) salta validación de formato y gate de `[NEEDS CLARIFICATION]`, y compara contra el hash **viejo**: una spec re-sellada legítimamente nunca se reanuda; hay que borrar el estado. `--spec` se ignora con `--resume`. | Releer, revalidar, diffear por título: conservar completados, re-RED los cambiados. |
| 🟠 | Dos algoritmos de limpieza del sello: `sealSpecification` (`interview.go:255-280`) quita líneas que *contienen* `<!-- seal: sha256:`; `CalculateCleanSpecHash` (`spec.go:41-51`) solo las del regex de 64 hex. Un sello parcial en un ejemplo ⇒ hashes distintos ⇒ "no coincide" sobre un fichero recién sellado. En Windows el hash depende de CRLF: `git autocrlf` **rompe todos los sellos al clonar**. | Un único `domain.Seal`/`StripSeal`; normalizar `\r\n` y espacios finales; versionar (`sha256-v1:`). |
| 🟠 | `spec.go:29` regex sin anclar y `FindStringSubmatch` coge la **primera** coincidencia: un sello antiguo citado en el cuerpo valida contra el hash equivocado. | `(?m)^<!-- seal: … -->\s*$`, última coincidencia. |
| 🟠 | No hay plan. `plan-template.md` se extrae a `.specify/` y **nadie lo lee**. El escenario 3 puede contradecir la estructura del 1. | `plan NNNN` → `plan.md` + R1. |

### 6.3 Parser y lint

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 ✔ | `spec.go:120-126` | `line[5:]` para seis keywords de distinta longitud. `Cuando se retira 30` → `When=["o se retira 30"]`; `Entonces el saldo es 70` → `Then=["ces el saldo es 70"]`. `spec_test.go:37` solo mira `len > 0`. | **`github.com/cucumber/gherkin/go`**: oficial, 70+ idiomas, And/But, Background, Outline+Examples, tablas. Borrar el parser casero. |
| 🔴 ✔ | `spec.go:97,134-141` | `### Scenario:` (formato de la plantilla) no cumple `HasPrefix(lower,"scenario:")` ⇒ **toda la spec es un escenario** "Requerimiento General" que incluye sello y boilerplate. | Extraer bloques ```` ```gherkin ```` del Markdown al parser oficial. `ErrNoScenarios` en vez de inventar. |
| 🟠 | `spec.go:86-144` | `And`/`But`/`Y`/`Pero` se pierden de los arrays; `Scenario Outline` detectado pero `Examples:` nunca expandido (placeholders `<x>` llegan al agente); `Background:` ignorado; `Feature:` no capturado (`BDDFeature` muerto). `loop.go:144-146` comprueba `err != nil \|\| len == 0`, pero el parser nunca devuelve error ni cero escenarios: rama muerta. | Parser oficial; expandir Outlines en N escenarios; `ErrNoScenarios`. |
| 🟠 | `spec.go:69-83` | `ExtractNeedsClarification` casa **cualquier** línea con el marcador, incluida la prosa de la plantilla que `interview` pre-siembra ⇒ spec bloqueada para siempre. `TrimPrefix` de un solo carácter deja `:`. | `^\s*[-*]\s*\[NEEDS CLARIFICATION\]\s*:?\s*(.+)$`; saltar código y comentarios HTML. |
| 🟠 | — | Sin lint: un `When`, ≥1 `Then`, títulos únicos (el estado indexa por título), términos de implementación. | `spec lint` dentro de `approve` y `loop`. |

### 6.4 Qué tiene una spec clara (plantilla objetivo con reglas de lint)

| § | Sección | Regla de lint | Para qué sirve al agente |
| :-: | :--- | :--- | :--- |
| 1 | Intención (por qué) | ≥ 2 frases; sin "cómo". | Elegir entre dos implementaciones válidas. |
| 2 | Actores y roles | ≥ 1 actor; cada US usa uno. | No inventar permisos. |
| 3 | Lenguaje ubicuo | Cada término de los escenarios está definido. | Nombres. **El glosario de memoria se genera desde aquí.** |
| 4 | Invariantes INV-xx | Cada INV referenciada por ≥ 1 Sad Path. | Qué validar siempre. |
| 5 | Historias de usuario | Como/Quiero/Para; ≥ 1 escenario por US. | Alcance. |
| 6 | Escenarios Gherkin | Un `When`, ≥ 1 `Then`, título único, sin UI/tecnología, Outline con Examples. | El test. |
| 7 | Contratos de datos | Campo / tipo / obligatorio / regla; cada campo nombrado en un escenario. | Firmas y DTOs. |
| 8 | Catálogo de errores | Código / cuándo / mensaje; cada error en un Sad Path. | Manejo de errores. |
| 9 | Requisitos no funcionales | Con número o "no aplica" explícito. | Estructuras de datos. |
| 10 | Fuera de alcance | ≥ 1 ítem. | YAGNI con base. |
| 11 | Supuestos | Qué se da por hecho y quién lo confirmó. | Trazabilidad. |
| 12 | Cuestiones abiertas | `- [NEEDS CLARIFICATION]: …`; **vacía para `approve`**. | P1. |
| — | Metadatos | `id`, `estado`, `aprobado_por`, `fecha`, `sello`, `supersede`. | Ciclo de vida. |

---

## 7. P3 — Todo valida: bucle TDD, puertas, auditor, E2E

### 7.1 Semántica RED / GREEN / REFACTOR

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `loop.go:224-235`, `:275` | **Manipulación de tests ni prohibida ni detectada.** GREEN dice "código productivo" pero no "no toques `*_test.go`"; REFACTOR dice "sin romper los tests", que se cumple borrando la aserción. | Tras RED: SHA-256 de cada fichero de test en el estado. Tras GREEN y REFACTOR: re-hash; diff ⇒ `TEST TAMPERING`. Regla en el bloque gestionado (§3.3). |
| 🔴 | `loop.go:199-209`, `dispatcher.go:81-94` | RED = exit code de toda la suite. (1) No compila ⇒ RED válido y GREEN arranca contra un error de compilación quemando 3 reintentos. (2) Sin cambios ⇒ falsa YAGNI con diagnóstico engañoso (`diagnostic.go:69-78`). (3) Test rojo preexistente ⇒ todo RED pasa y GREEN es imposible. (4) Stack desconocido ⇒ `echo` exit 0 (`dispatcher.go:91-92`) ⇒ **todo proyecto no soportado es "violación YAGNI"**. | `TestOutcome{Compiled, Passed, Failed int, Output}` parseando `go test -json`, surefire XML, vitest `--reporter=json`, pytest `--junitxml`. Gate RED = `Compiled && Failed > 0`. `git status` antes de ejecutar. `ErrUnsupportedStack`. |
| 🔴 | `loop.go:201-224` | La salida del RED se descarta y `LastError` no se asigna (`AdvanceToNextPhase` lo limpia en `:143`) ⇒ el **primer** GREEN va a ciegas. | `state.LastError = output` tras avanzar. |
| 🟠 | `dispatcher.go:81-94` | `scenario` se acepta y **se ignora**. | Marcador `[SDD-0001-3]` en el test; `-run`/`-Dtest=`/`-t`/`-k`; suite completa en REFACTOR. |
| 🟠 | `loop.go:218,245-254`, `diagnostic.go:108-110` | El `3` mágico 7 veces; tras agotar, `--resume` muestra "Intento 4/3", da uno y aborta. | `const maxGreenRetries`; `--max-retries`; reset en resume. |
| 🟠 | `loop.go:201,233` | `passed, output, _ := comp.RunTests(...)`: toolchain ausente ⇒ "RED legítimo". | `errors.Is(err, exec.ErrNotFound)` ⇒ `blocked`. |
| 🟠 | `loop.go:276,279` | `_, _ = runner.RunHeadless` y `passedAfter, reportAfter, _ :=` en REFACTOR: un fallo del agente es "refactor hecho". | Propagar. |
| 🟡 | `state.go:133-138` vs `loop.go:193` | `CurrentScenarioData` devuelve puntero; el loop lo desreferencia (copia). | Elegir uno. |

### 7.2 Puertas de calidad

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 ✔ | `jscpd.go:37-40`, `knip.go:33-36`, `stryker.go:42-44` | `err != nil && ExitCode == 1 && Output == ""` ⇒ **`passed = true`**. Casa también con `npx` sin red, jscpd crasheando por config, knip sin leer tsconfig, stryker sin arrancar el runner. Todo imprime "✓ DRY cumplido al 100 %". **Una puerta que aprueba cuando no pudo ejecutarse es peor que ninguna.** | `GateResult{Passed, Skipped, Tool, Report}`; `ErrNotFound` ⇒ `Skipped` (⚠); `--strict` ⇒ fallo. |
| 🔴 ✔ | `quality.go:58-65` | SonarGate siempre `true`; con `SONAR_HOST_URL` imprime "validado contra <host>" sin petición. | Borrar (o `GET /api/qualitygates/project_status`). |
| 🟠 | `stryker.go:52-53`, `jscpd.go:48-51`, `knip.go:32` | Veredicto por substring de salida humana ("is below threshold", "Surviving mutants", "Unused files"): varía por versión e idioma. jscpd con `--threshold 0` escribe el fallo en stderr y los clones en stdout, así que la heurística `Output == ""` puede tragarse un **fallo real**. | `--reporters json`; leer `mutationScore` / `statistics.total.percentage` / longitudes. |
| 🟠 | `constitution.md:37` (Stryker ≥80), `scaffold/react/stryker.conf.json:7-11` (`break: 50`), `java.md:105` (JaCoCo 70), `go.md:16` (≥80 sin gate) | Umbrales inconsistentes y no aplicados. jscpd al 0 % falla en cualquier Java con DTOs y el agente tiene **un** intento (`loop.go:276-285`). | Bloque `quality:` en `.specify/config.yaml` leído por scaffolds **y** gates. |
| 🟠 | `linter.go:28,37-44,55-66` | `npm run lint --if-present` sin script ⇒ "limpio". Error de `golangci-lint` (versión/config, reproducido) ⇒ "violación" que el loop pide arreglar a la IA. Python sin ruff y stacks desconocidos ⇒ aprobado. | Detectar script; exit 1 vs ≥ 2 ⇒ fallback `go vet`; `Skipped`. |
| 🟠 | `quality.go:27-36` | ArchUnit "obligatorio" pero aprueba si no hay test; `mvn test` completo sin `-q -DfailIfNoTests=false`. | Presencia obligatoria en JVM; flags. |
| 🟠 | `quality.go:36`, `jscpd.go:33`, `stryker.go:41` | Sin timeouts; `stryker run` puede tardar horas. | Deadlines por gate. |
| 🟡 | `quality.go:76-110` | Cuatro de cinco ramas del `switch` son los mismos tres gates. `stryker.go:52` mezcla `\|\|`/`&&` sin paréntesis. | "Base gates + extras por stack" vía `StackProfile`. |

### 7.3 Auditor de seguridad

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 ✔ | `cloudflare_auditor.go:108-114` | JSON del verificador ilegible ⇒ **informe vacío**, `findings.json` vacío, Info "Confirmadas: 0", `audit.go:105` "✓ superada". | Fail-closed; un reintento con el error de esquema. |
| 🔴 | `:134`, `audit.go:39,95` | `git diff <target>` sin validar: `--target='--output=~/.bashrc'`, `--ext-diff` ⇒ sobrescritura / ejecución. | `--end-of-options`; rechazar `-*`; `rev-parse --verify`. |
| 🟠 | `:108` | `ValidateFindingsAgainstSchema(..., nil)`: `report-schema.json` embebido **nunca se usa**. | `santhosh-tekuri/jsonschema`. |
| 🟠 | `:76-90` | Recon fallido solo avisa; Hunting corre con ledger vacío (`:81-90`) y el ledger solo se escribe si no está vacío ⇒ no se sabe que recon no ocurrió. | Fallar o `recon_failed`. |
| 🟠 | `security.go:178-181`, `:171-202` | `needs_validation` ignorado por `--fail-on`; sin `low`; `"confirmed"` (estado) usado como umbral de severidad. `ExtractJSONFromMarkdown` `:87-91` coge del primer `{` al último `}`: dos bloques JSON ⇒ inválido. | Enum `Threshold` ordenado; `needs_validation ≥ high` bloquea → R3; parsear bloques cercados iterativamente. |
| 🟠 | `:59-63`, `:66-68` | `--full` solo ve el context pack (snippets de 80 líneas de ficheros con "service" en el path): la mayor parte del código no se audita. Si no hay ficheros, se envía al LLM "No se detectaron archivos…" como objetivo. | Chunkear por directorio; abortar sin código. |
| 🟠 | `:75,89,100` | El contexto entero incrustado tres veces en argv (§13.1). | stdin; presupuesto de tokens. |
| 🟠 | `:134-156` | `exec.Command` sin ctx (3 sitios); `CombinedOutput` mezcla stderr en el diff que va al LLM; fallback silencioso a `--staged` y a `""`. `git diff HEAD~1` = working tree vs HEAD~1, no "último commit". | ctx; stdout solo; `HEAD~1..HEAD`. |
| 🟠 | `:84,118,121` | `findings.json`/`REPORT.md` con PoCs de vulnerabilidades 0644 con errores ignorados. | 0600; propagar. |
| 🟡 | `audit.go:28`, `commands.go:22` | "6 fases" vs 3-4 reales. ✅ PR #2 corrige el texto. | — |
| 🟡 | `:59` | Dependencia concreta de `ingest.NewProjectScanner()`. | Puerto `ContextProvider`. |

### 7.4 E2E

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `chromedp_driver.go:53-54,71-76,146,166-185,204` | Todos los métodos ignoran el `ctx` del llamador y usan `d.taskCtx` de `context.Background()`. `WaitVisible` sobre selector alucinado **cuelga para siempre**; Ctrl-C nunca llega al navegador. `:54` descarta un cancel ⇒ fuga del contexto de la pestaña. | ctx del llamador; `WithTimeout(10s)` por acción; guardar y llamar ambos cancels. |
| 🟠 | `vision_agent.go:226-241`, `:207-216` | `parseVisualAction` sin validación: `action_type` fuera del enum, selector no comprobado contra el snapshot (regla 1 del prompt sin aplicar), `navigate` a cualquier URL. El texto de la página entra literal al prompt ⇒ **inyección desde una página maliciosa**. | Enum; selector ∈ snapshot; same-origin; tope de `value`. |
| 🟠 | `vision_agent.go:98-105` | Respuesta ilegible ⇒ `wait` y sigue, quemando hasta 15 llamadas de pago. | Reintento "solo JSON", luego fallo (§3.1). |
| 🟠 | `vision_agent.go:82-85` | Error de snapshot solo avisa; se pide al agente actuar a ciegas. | Fallo de paso, screenshot, reintento/aborto. |
| 🟠 | `vision_agent.go:40-45,58` | Instancia `agent.NewClaudeAgentRunner()` y el driver dentro del adaptador (adaptador→adaptador). Imposible testear sin navegador y LLM. | Inyectar `ports.AgentRunner` y `BrowserDriver`. |
| 🟠 | `vision_agent.go:74-77` | `agentOpts` sin `WorkingDir` ni `Model`: la memoria no se inyecta en E2E y el modelo configurado se ignora. | Opciones completas. |
| 🟠 | `vision_agent.go:120-128`, `:186-192` | Fin cuando el modelo emite `is_success:true`: SpecForge nunca comprueba la página contra el `Then`. Envía la spec **entera** cada paso. Un booleano por spec. | Por escenario con `then_index`; `evidence` verificada en Go; `report.json`; `--min-pass-rate`. |
| 🟠 | `chromedp_driver.go:39`, `:117-122` | `ignore-certificate-errors` siempre; el snapshot **muta el DOM** (`data-sdd-id`, contador por snapshot ⇒ IDs colisionan). | Opt-in; XPath/`nth-of-type` sin mutar. |
| 🟡 | `chromedp_driver.go:221-261`, `:43-47` | Discovery sin `google-chrome-stable`, `brave`, `LOCALAPPDATA`, `CHROME_PATH`; sin error si no hay navegador. `Headless` duplicado en opciones. | Env override; error claro. |
| 🟡 | `vision_agent.go:32,125,150`, `e2e.go:86-91` | Screenshot único sobrescrito; "Tiempo total: normal" placeholder; `time.Sleep` no ctx-aware; modo "exploratorio" con frase en español hardcodeada. | Por paso; medir; ctx; `--exploratory`. |

---

## 8. Prompts

Los prompts tal como se envían:

- **RED** (`dispatcher.go:52-59`): «MISIÓN: TDD FASE RED … Stack: %s … Solo genera el archivo de prueba … Crea o actualiza el archivo de test en el directorio estándar.»
- **GREEN** (`loop.go:311-331`): «MISIÓN: TDD FASE GREEN … Implementa EXCLUSIVAMENTE el código productivo … ESCENARIO BDD … ERROR PREVIO ```<2000 chars>```»
- **REFACTOR** (`loop.go:275`): «El código ha pasado los tests… Corrige las violaciones … sin romper los tests existentes: %s»

| Sev | Problema | Mejora |
| :---: | :--- | :--- |
| 🟠 | **Casi sin contexto.** Falta: plan, tests existentes de la feature (el escenario 2 crea un segundo fichero), árbol, estándar (`project.StandardDoc` se calcula en `project.go:42,53…` y nunca se inyecta), constitución (se extrae y nadie la lee), cabecera de la spec (glosario e invariantes se tiran), comando de test, ruta exacta. `ingest` escribe `docs/_context/context-pack.md` y **ningún prompt lo lee**. | `PromptContext{constitution, standard, plan, glossary+invariants, testFiles, tree, testCmd, scenario, lastError, decisions}`; prompts como `text/template`. |
| 🟠 | Sin contrato de salida en RED/GREEN/REFACTOR; `RunHeadless` devuelve stdout y se descarta (`dispatcher.go:62`, `loop.go:225`). El agente es de confianza para "haber escrito ficheros". | Protocolo §3.1; `files_written` vs `git status`; fallo si diff vacío. |
| 🟠 | Error truncado a los **primeros 2000 chars** (`loop.go:323`): en Maven/Jest son banners. | Head 500 + tail 3000, o bloque del test fallido por stack. |
| 🟠 | Seis ficheros de memoria envueltos en `<system_instruction>` dentro del **prompt de usuario** vía `-p` (`claude.go:34`, `gemini.go:34`). Cosmético. Claude Code ya carga `CLAUDE.md`; Gemini, `GEMINI.md`; y `.gemini/` está gitignored por `setup` (`setup.go:134`). | Bloque gestionado en esos ficheros; `-p` solo con la tarea; `claude -p --append-system-prompt --output-format json`. |
| 🟠 | Vision: spec entera cada paso, sin saber qué `Then` comprueba, sin `scroll`/`select`/`press_key`. | §7.4. |
| 🟡 | `GenerateTestStubs` genera tests completos. Mayúsculas gritonas ("MISIÓN", "PROHIBIDO", "REGLA DE ORO"). | Renombrar; cabeceras Markdown y restricciones numeradas. |

---

## 9. P7 — Memoria útil

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🟠 | `agent_memory.go:65-96`, `loop.go:238,287` | `RecordLesson` = timestamp + 120 chars del **stderr del compilador** + solución **hardcodeada** ("Implementación mínima y corrección de tipos/sintaxis"). Sin dedupe ni tope. Todo (`LoadAgentContext` `:46-55` concatena los seis) se reinyecta en cada prompt. "Nunca repite el mismo error" (`USER_GUIDE.md:158`) sin soporte. ✅ PR #2 corrige el texto de la guía. | Lección escrita por el agente ("≤ 2 líneas, la regla que lo habría evitado"); dedupe; tope 30; etiqueta `stack/gate`; inyectar solo las pertinentes. |
| 🟠 | `agent-context/*` | `glossary.md:9-11` define MDCMS/MDOpen (AS/400), Tekton, probes; `references.md:33-43` es YAML de Spring Actuator; `ng-rules.md:6` cita IBAN/divisas; `agente.md`+`ng-rules.md` exigen "Dominio Rico"/hexagonal para una SPA React o un script Python. | Baselines por stack. **Glosario generado desde la spec §3.** |
| 🟡 | Seis ficheros | `agente`+`persona`+`ng-rules` son uno; `glossary` duplica la spec; `references` nadie lo actualiza. `.sdd/` gitignored ⇒ "lecciones del equipo" nunca compartidas. `setup.go:106-130` sobreescribe sin `--force`. | Bloque gestionado en `CLAUDE.md`/`GEMINI.md` + `lessons.md` **versionado** + `decisions.md`. |

### Qué recuerda SpecForge y dónde

| Memoria | Fichero | Quién escribe | Cuándo se inyecta |
| :--- | :--- | :--- | :--- |
| Reglas (P1, no tocar tests, estándar) | `CLAUDE.md` / `GEMINI.md` bloque gestionado | `setup` | Siempre (nativo) |
| Decisiones y respuestas | `specs/NNNN/decisions.md` | Runtime en R3 | Siempre, en la spec actual |
| Lecciones | `.sdd/lessons.md` (versionado) | Agente al cerrar escenario; dedupe | Las del stack y gate actuales |
| Glosario | Sección 3 de la spec | Entrevista | Con el escenario |
| Plan | `specs/NNNN/plan.md` | `plan` | RED y GREEN |
| Transcripción | `specs/NNNN/interview.jsonl` | `spec interview` | Nunca en prompts; auditoría y replay |

---

## 10. P8 — Entrega clara

Hoy el loop termina con "🎉 CICLO TDD COMPLETADO" y nada más.

### 10.1 `specforge deliver NNNN`

Genera, **solo a partir de artefactos existentes**:

```text
specs/0001-password-reset/
  DELIVERY.md        ← el paquete legible
  trace.json         ← escenario → test → ficheros → commit → puertas
  PR_BODY.md         ← listo para `gh pr create --body-file`
```

```markdown
# Entrega · SDD-0001 Password reset

**Spec** 0001 · aprobada por @jefmonjor el 2026-10-06 · sello sha256-v1:f15155…
**Plan** aprobado el 2026-10-06 · 3 componentes · 4 ficheros de test
**Escenarios** 6/6 verdes · 2 revisados con cambios · 0 rechazados

| # | Escenario | Test | Commit | Gates |
|---|---|---|---|---|
| 1 | User requests a reset link | auth/reset_test.go::TestReset_RequestsLink [SDD-0001-1] | a1b2c3d | lint ✓ dup ✓ |

## Decisiones tomadas durante el desarrollo
- (GREEN, esc. 3) "¿30 min desde la petición o desde la entrega?" → **desde la petición** (@jefmonjor)

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

### 10.2 Reglas

- **Trazable**: cada fila enlaza a un commit y a un test que existen.
- **Honesta**: `Skipped` aparece como ⚠, nunca ✓. Preguntas sin responder, como pendientes.
- **Completa**: incluye lo que **no** se hizo.
- **Reutilizable**: `PR_BODY.md` sigue la plantilla del repo si existe.

---

## 11. P6 — No reinventar la rueda

| Hoy a mano | Problema real que ha causado | Sustituir por | Estado |
| :--- | :--- | :--- | :---: |
| Parser Gherkin (`spec.go:86-144`) | Mutila el español, ignora And/But/Outline. ✔ | `github.com/cucumber/gherkin/go` | ⬜ |
| Matcher `.gitignore` (`context.go:65-102`) | Ignora negaciones, anclas, `**`; `bin` mata `src/bin/x.go`; `target` mata `src/target-audience/`. | `sabhiram/go-gitignore` o `go-git` | ⬜ |
| Exec + Windows `.cmd/.exe` (`dispatcher.go:96-148` = `quality/process.go:12-64`) | Duplicado byte a byte; sin timeout; `ErrNotFound` indistinguible. | `adapters/exec` propio (60 líneas) sobre `os/exec`. | ⬜ |
| Logger singleton (`storage/logger.go`) | Todo al fichero, 0644, sin rotación. | `log/slog` + `lumberjack` | ⬜ |
| Validación de esquema | Esquema embebido sin usar. | `santhosh-tekuri/jsonschema/v6` | ⬜ |
| Resultado de tests por exit code | No distingue compila/falla. | `go test -json`, surefire, vitest JSON, junitxml | ⬜ |
| Gates por substring | Depende de versión e idioma. | Reporters JSON | ⬜ |
| PATH por PowerShell (`system/path.go`) | Inyección con `'`. | **Eliminar.** Homebrew/Scoop/`go install`/goreleaser | ⬜ |
| 4 scripts de build | Sin `-X`, sin checksums. | `Makefile` + `goreleaser` | ✅ PR #2 |
| Enums `string` sin validar | `--agent copilot` ⇒ Gemini. | `type AgentName string` + `Parse*` + `pflag.Value` | ⬜ |
| Config en `~/.specforge` y `~/.sdd` | Dos rutas, 0644 con token. | `os.UserConfigDir()`; 0600 | 🔄 (rutas unificadas en PR #2; perms ⬜) |
| 6 ficheros de memoria en `-p` | Cosmético. | Bloque gestionado + `--append-system-prompt` | ⬜ |
| Spinner + `time.Sleep(50ms)` | Carrera; CI lleno de `\r`. | `briandowns/spinner` con TTY check | ⬜ |
| Conversión PDF/Word (`doc/`) | Python en un "zero deps". | `--docs` opcional o eliminar | ⬜ |
| Versión hardcodeada ×10 | Inconsistente. | `internal/buildinfo` + `-X` | ✅ PR #2 |

**Menos superficie**: de 11 comandos (`init setup ingest doc interview loop e2e audit consistency build version`) a 8 (`init setup spec plan loop deliver e2e audit`), con `spec` agrupando `new interview clarify lint approve amend`.

---

## 12. P4 + P5 — Go: Clean Architecture, SOLID y artesanía

### 12.1 Clean Architecture

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `domain/spec.go:6,54-66`, `state.go:7,94-118,187-208`, `seal.go:8-25`, `project.go:4-5,33-96`, `agent_memory.go:5-6,28-96`, `context.go:5,59-82`, `diagnostic.go:5,159-182` | El dominio importa `os`/`filepath`/`io`, hace I/O (`VerifySpecIntegrity`, `Save/Load`, `CalculateSHA256`, `DetectProject`, `LoadAgentContext`, `RecordLesson`, `loadIgnoreFile`) y escribe en stderr (`PrintDiagnosticBox`). | Dominio = puro. Persistencia tras `StateRepository`, `SpecRepository`, `LessonsRepository`, `ProjectDetector`. Presentación en `ui/`. |
| 🔴 | `loop.go:44-309` | `runLoop` (265 líneas) **es** la capa de aplicación: spec, sello, gate, máquina de estados, reintentos, lecciones, prompts. No existe `internal/app`. | `internal/app/tddloop.Service{Agent, Compiler, Quality, States, Specs, Clock, Out}`; `runRed/runGreen/runRefactor/review`; `cmd/` solo flags. |
| 🟠 | `loop.go:157-172`, `interview.go:182-187`, `audit.go:78-99`, `init.go:70`, `e2e.go:101`, `ingest.go:57,70`, `doc.go:47` | Cada comando instancia adaptadores. Sin composition root. `if claude else gemini` ×3; valor desconocido ⇒ Gemini. | `cmd/wire.go` con `Deps`; `newLoopCmd(deps)`; `AgentName` + registro. |
| 🟠 | `ports/compiler.go:19-21`, `ports/security.go:18` | Compiler depende de Agent (`GenerateTestStubs(runner…)`); `Test()` y `RunTests(scenario)` solapan. | Prompts al app layer; runner por constructor; un método de test con filtro. |
| 🟠 | `ports/*` | Falta `CommandRunner`, `Logger`, `ProjectScanner`, `DocumentConverter`. `cmd/doc.go`, `cmd/ingest.go`, `security/cloudflare_auditor.go:13,59` dependen de structs concretos. | Puertos donde se consumen. |
| 🟠 | `ports/mainframe.go:6-9`, `ports/agent.go:6-13`, `ports/quality.go:11` | `MainframeConfigurator` sin implementación ni llamador. `Temperature`/`Debug` nunca leídos; `Project/Location` son GCP. `(passed, report, err)` no distingue "pasó" de "no pudo correr". | Borrar; `GateResult`. |
| 🟠 | `consistency.go:42-149`, `setup.go:331-340`, `ingest.go:118` | `cmd/` ejecuta `git`, `codegraph` y recorre el FS (`findJavaFiles`); exit code 6 mágico. | Adaptadores `vcs`; errores tipados. |
| 🟡 | `diagnostic.go:21-156` | Diagnóstico por substring del mensaje **en español** ("límite de 3 reintentos"). | Centinelas/tipados + `errors.Is/As`. |

### 12.2 SOLID / diseño

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🟠 | `state.go:48-60` vs `63-75`, `14-27` | `LoopState` y `TDDState` en el **mismo fichero** con esquemas incompatibles (`LoadState` sobre un fichero TDD da basura). `LoopState`, `StatePhase` y 12 constantes sin llamadores de producción. `RecordStep:125` hace `StatePhase(s.CurrentPhase)`. | Borrar `LoopState`/`StatePhase`; `Checkpoint.Phase = TDDPhase`. |
| 🟠 | `spec.go:21-27`, `seal.go:12`, `config.go:6-13,27` | `BDDFeature` y `CalculateSHA256` muertos. `SDDConfig` God-config (agente + GCP + Nexus + Sonar + env); `Repository.Mode` documentado `nexus/local/file`, escrito `remote/embedded` (`init.go:129,140,153`). Nexus/Sonar nunca se leen salvo `PopulateEnvMap`. | Borrar; enums con `Validate()`. |
| 🟠 | `project.go:33-88,70` | First-match: `go.mod`+`package.json` ⇒ Go. Angular → `react.md`. `JavaLegacy` nunca se emite. | Marcadores ordenados + prioridad. |
| 🟠 | `dispatcher.go:66-94`, `linter.go:25`, `quality.go:76-110` | El mismo `switch project.Type` ×4. | `domain.StackProfile{Build, Test(filter), Lint, Gates, TestGlob}`. |
| 🟠 | `agent/claude.go:17-119` vs `gemini.go:17-135` | ~90 % duplicado (memoria, spinner, buffers, logging, wrapping). Solo cambian binario, `-m`, `-i`, env. | `cliRunner{bin, headlessArgs, interactiveArgs, env}`. |
| 🟡 | `cmd/*.go`, `agent_memory.go:18` | 30+ globales de flags (`init.go:18,23` `flagAgent`/`flagForce` genéricos); `init()` por fichero; `ContextMemoryFiles` exportado mutable. | Options struct por comando; unexported. |

### 12.3 Artesanía: errores, durabilidad, concurrencia

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `loop.go:149,179,195,206,214,227,243,247,273,282,296` | **Todos** los `state.Save` ignorados. | `persist(st) error`; `%w`. |
| 🔴 | `state.go:94-102,187-195` | "Atómico" en el comentario; `os.WriteFile` en el código. ✅ PR #2 corrige el texto de la guía; el código ⬜. | tmp + `Sync` + `Rename`. |
| 🟠 | `setup.go:49-51`, `consistency.go:26-33`, `build_cmd.go:20` | `os.Exit` en `RunE`: salta diagnóstico y `defer`; `TestSetup*` saldrían del proceso en `master`. `deployScaffold:260-283` devuelve `nil` aunque fallen todos los `WriteFile`. | Errores tipados; `Execute()` mapea exit codes. |
| 🟠 | `setup.go:97-104` | `ExtractDir("commands/gemini/.gemini")` apunta a un path **inexistente**; warning en log y "✓ Comandos /speckit instalados": falso éxito en cada setup con Gemini. | Shippear o borrar. **Nunca ✓ tras un warning.** |
| 🟠 | `setup.go:47-52`, `:308-329`, `:275` | Solo `master` protegido (`main` no). `ensureGitIgnoreEntry` con `strings.Contains`: `.sdd/` "ya presente" si existe `.sdd-cache/`. Scaffold Go `module <dirname>` puede ser inválido. | `{master, main}`; comparar líneas; validar módulo. |
| 🟠 | `main.go:13-18` | Reescribe `-xyz` → `--xyz`: rompe bundling de pflag, muta `os.Args` para los hijos. Existe por `-DryRun`. | Borrar; `SetNormalizeFunc`. |
| 🟠 | `audit.go:64,40`, `security.go:183-198` | `--full --diff` ambos true; `--fail-on hgih` no casa nada ⇒ pasa. | `MarkFlagsMutuallyExclusive`; enum en parse. |
| 🟠 | `root.go:36-41`, `interview.go:91,105`, ~25 sitios | `InitGlobalLogger` error ignorado; `ConvertDirectory` errores ignorados; ~25 `_ =` sobre `WriteFile`/`MkdirAll`. | Propagar; `errors.Join`. |
| 🟠 | Todos los `cmd/*.go` | `context.Background()` (`loop.go:193,201,225,233,263,276,279`, `interview.go:189`, `audit.go:100`, `init.go:71`, `e2e.go:102`). Ctrl-C deja huérfanos. | `signal.NotifyContext`; `cmd.Context()`; `--step-timeout`; `WaitDelay`. |
| 🟡 | `audit.go:121`, `loop.go:208,251,284`, `state.go:201` | Errores con mayúscula y emoji; `LoadState` sin wrapping. | Minúscula; `%w`. |
| 🟡 | `console_windows.go:27` | `uintptr(0xFFFFFFF5)` no es `STD_OUTPUT_HANDLE` en 64 bits (funciona por truncado accidental); no maneja stderr ni redirección. | `windows.STD_OUTPUT_HANDLE`, `GetConsoleMode/SetConsoleMode`. |

---

## 13. Auditoría por paquete (adaptadores)

### 13.1 `adapters/agent`

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `claude.go:40-41,104-109`, `gemini.go:40,45,117-125` | **Prompt entero como un argumento de argv.** Linux `MAX_ARG_STRLEN` 128 KiB (`E2BIG`); Windows 32.767 chars. El auditor concatena el código entero + 3 prompts + 6 ficheros de memoria. Visible en `ps`/`/proc/<pid>/cmdline`. | `cmd.Stdin` (ambos CLIs leen stdin) o fichero 0600. |
| 🔴 | `claude.go:17-119` vs `gemini.go:17-135` | ~90 % duplicado. | `cliRunner` table-driven. |
| 🟠 | `claude.go:52-75`, `gemini.go:65-88` | Spinner a stdout aunque no sea TTY (CI lleno de `\r`); logger a stderr ⇒ interleaving. `close(done); time.Sleep(50ms)` hack de carrera. | `term.IsTerminal`; `WaitGroup`; stderr; `ui.Spinner`. |
| 🟠 | `claude.go:41`, `gemini.go:45` | Sin timeout ni `WaitDelay`: un CLI colgado deja pipes abiertos para siempre. | Deadline en opciones; `WaitDelay = 5s`. |
| 🟠 | `claude.go:46-48`, `gemini.go:59-61` | Stdout totalmente buffered: 20 minutos de spinner y "no cancelar con Ctrl+C". | `io.MultiWriter`; streaming por líneas al logger. |
| 🟠 | `claude.go:27`, `gemini.go:57` | Claude ignora `Model`; Gemini fuerza `GOOGLE_GENAI_USE_VERTEXAI=true` aunque `UseVertexAI == false`. | Honrar opciones; env condicional. |
| 🟠 | `claude.go:33,99`, `gemini.go:33,112` | Error de `LoadAgentContext` tragado: un fichero de memoria corrupto silencia la "memoria". | Warn mínimo. |
| 🟠 | `claude.go:38,80,82`, `gemini.go:38,93,95` | `LogIO` vuelca prompt completo (código fuente, specs) y salida a `~/.specforge/logs/*.log` **en cada ejecución sin importar `--debug`** (ver 13.9). | `--trace-io` opt-in; 0600. |
| 🟠 | — | Cero tests. | Con `CommandRunner` falso, argv/env/stdin/wrapping son triviales. |
| 🟡 | `claude.go:86,89`, `gemini.go:102` | Error con stderr completo (puede ser enorme); mensaje de éxito a stdout desde el adaptador. | Truncar; UI en `cmd/`. |

### 13.2 `adapters/auth`

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🟠 | `gcp_adc.go:124`, `:67,70` | `exec.Command("gcloud")` ignora `ctx`; 1-3 s por llamada, hasta dos veces. | `CommandContext` 10 s o `gcloud config list --format=json`. |
| 🟠 | `gcp_adc.go:76-78` | En fallo devuelve `Project = "default-project"` con `err == nil`; se persiste y se envía a Gemini ⇒ 403 confusos. | `ErrNoProject`; que `init` pregunte. |
| 🟠 | `gcp_adc.go:42-43` | `HasADC = true` solo porque el fichero existe, antes de validarlo. | Desde el SDK o `GetAccessToken`. |
| 🟠 | `gcp_adc.go:98-99` | `GOOGLE_APPLICATION_CREDENTIALS` puede ser una **clave de service account**; se lee igual que ADC de usuario, sin comprobar permisos 0600. | Warn si `type == "service_account"`; no loguear la ruta a Info. |
| 🟠 | `gcp_adc.go:34` + `domain.NewDefaultConfig` | `europe-west1` hardcodeado y duplicado. | Una fuente. |
| 🟠 | — | Cero tests; `getADCFilePath` y `adcJSON` son puros y testables con `t.Setenv`. | Añadir. |
| 🟡 | `gcp_adc.go:57,130` | Sin scopes; heurística `(unset)` muerta (gcloud lo escribe en stderr y `Output()` lo ignora). | Un helper; `--format=value(...)`. |

> Todo este paquete sale del core en Fase 3 (§5.2); arreglar lo mínimo para que no mienta hasta entonces.

### 13.3 `adapters/compiler`

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `dispatcher.go:96-109,111-148` | Copia byte a byte de `quality/process.go:12-25,27-64` (`winCmd`, `runProcessWithPipes`). | `adapters/exec`. |
| 🔴 | `dispatcher.go:75` | `python -m py_compile` **sin ficheros**: lee stdin (`/dev/null`) y sale 0. El build Python siempre aprueba sin compilar. | `python3 -m compileall -q .`; fallback `python3 → python → py`. |
| 🟠 | `dispatcher.go:76-77,91-92` | Stack desconocido ⇒ `echo` con `Success=true`. | `ErrUnsupportedStack`. |
| 🟠 | `dispatcher.go:86` | `npm test -- --run` es Vitest-only; Jest/Mocha/Karma lo rechazan. | Detectar runner en `package.json`. |
| 🟠 | `dispatcher.go:81-94` | `scenario` ignorado. | Filtros por runner. |
| 🟠 | `dispatcher.go:111-148` | Salida de `mvn test` entera en memoria sin tope, concatenada al prompt. | Streaming + ring buffer. |
| 🟠 | `dispatcher.go:131-135` | `ExitCode()` es −1 si lo mata ctx; "binario no encontrado" indistinguible de fallo real: **causa raíz** de los gates fail-open. | `errors.Is(err, exec.ErrNotFound)` ⇒ `ToolMissing`. |
| 🟠 | `dispatcher.go:48-64` | `GenerateTestStubs` construye un prompt en español dentro del compilador y nunca verifica que se creó un test. | App layer. |
| 🟡 | `dispatcher.go:69` | `mvn clean compile test-compile` con `clean` en cada iteración. | Quitar `clean`. |
| 🟡 | `dispatcher_test.go:38-57` | Depende de `echo`/`cmd` del host. | Falso. |

### 13.4 `adapters/doc`

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🟠 | `converter.go:61` | `exec.Command` sin ctx; `ConvertFile` sin parámetro `ctx`. Una conversión colgada bloquea para siempre. | `ctx` + timeout por fichero. |
| 🟠 | `converter.go:56-59` | Sin `markitdown`, fallback `python -m markitdown`; sin `python` ⇒ error críptico. | Probar `markitdown`, `python3 -m`, `python -m`, `uvx`; `ErrMarkitdownMissing` con pista de instalación. |
| 🟠 | `converter.go:78-80,97-99` | `ConvertDirectory` traga todo: Walk errors ⇒ `nil`; errores de `ConvertFile` descartados. 40 PDFs sin markitdown ⇒ `([], nil)`. | `errors.Join`. |
| 🟠 | `converter.go:94-95` | `foo.pdf` y `foo.docx` ⇒ ambos `foo.md`; el segundo pisa al primero. | `foo.pdf.md` o detectar colisión. |
| 🟡 | `converter.go:93,85-87`, `converter_test.go:43-47` | `ext != ".md"` muerto; ignore list duplicada; test que no podía fallar. ✅ PR #2 mueve el test a integración. | Reusar filtro. |

### 13.5 `adapters/e2e` — ver §7.4.

### 13.6 `adapters/ingest`

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🟠 | `scanner.go:27` | Sin `ctx`, sin tope de ficheros/tamaño; un monorepo se recorre entero y el árbol crece sin límite (y va a un prompt). "~20 ms" sin benchmark. | `ctx`, `MaxFiles`, `MaxTreeDepth`, `BenchmarkScanProject`. |
| 🟠 | `scanner.go:51` + `context.go:85-102` | Matching por componente: `docs/build` o `src/**/generated` nunca casan; negaciones descartadas (`:73`); `EqualFold` ignora `Dist`/`Bin`. | Librería gitignore (§11). |
| 🟠 | `scanner.go:83-84,102-120` | Sin detección de binarios; `snippet[:MaxSnippetBytes]` parte runas UTF-8. | `utf8.Valid`; cortar en frontera de runa. |
| 🟡 | `scanner.go:143-145,62,66,40-41` | "Key file" por `Contains("app"\|"api"\|"index"\|"main")` en el path entero (`happy.go`, `rapid.py`); emoji en el árbol que va al LLM; Walk errors ignorados. | Segmentos; sin emoji; propagar. |

### 13.7 `adapters/quality` — ver §7.2.

### 13.8 `adapters/security` — ver §7.3.

### 13.9 `adapters/storage`

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `logger.go:94-109,128-130,83,42,64` | `Log()` escribe **todos los niveles al fichero**; `debugMode`/`verbose` solo filtran stderr. `LogIO` persiste prompts (código), salida de IA y stderr de herramientas en cada ejecución en `~/.specforge/logs/sdd-YYYYMMDD.log` 0644 en dir 0755, sin rotación. | Umbral de fichero; `--trace-io`; 0600/0700; rotación. |
| 🟠 | `logger.go:32-68` | Singleton con dos inits inconsistentes: `GetLogger()` asigna sin `once`; `InitGlobalLogger` usa `once` pero devuelve su propia instancia aunque `once` ya disparó. Racy si el spinner y main lo llaman a la vez. Sin `Close()`. | Un constructor; `ports.Logger` inyectado; `Close()` en root. |
| 🟠 | `config.go:75` | `config.json` 0644 con `SonarQube.Token`, `Account`, `Project`, `env`. No atómico. | 0600; tokens fuera; tmp+rename. |
| 🟠 | `config.go:30`, `logger.go:41` | `~/.specforge` hardcodeado; sin XDG/`%APPDATA%`; logs y config en el mismo dir. | `os.UserConfigDir()`; `SPECFORGE_HOME`. |
| 🟡 | `config.go:43,63`, `logger.go:*` | "Ejecuta 'sdd init'"; `sdd-*.log`; `Save` muta la entrada (`PopulateEnvMap`). Logger sin tests. ✅ PR #2 unifica textos `~/.sdd` → `~/.specforge`. | Renombrar fichero de log; copia; tests con `logDir` inyectable. |

### 13.10 `adapters/system`

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🔴 | `path.go:40-50` | Ruta del ejecutable interpolada en PowerShell entre comillas simples: una `'` (usuario "O'Brien") ejecuta código; `-notlike "*$d*"` trata `[]?*` como comodines; `C:\tools` "presente" si existe `C:\tools2`. | **Eliminar la función.** Si se conserva: `x/sys/windows/registry` + `WM_SETTINGCHANGE`. |
| 🟠 | `path.go:73,83,80` | `export PATH="<dir>:$PATH"` sin escapar: `"` o `$(…)` se ejecutan en cada shell. Idempotencia por substring. | `%q`; bloque marcador `# >>> specforge >>>`. |
| 🟠 | `path.go:74-89,95` | Escribe en `.bashrc` **y** `.zshrc` sin mirar `$SHELL`; ignora `.profile`/`.zprofile`/fish; si no existe ninguno dice "ya está registrado". | Por `$SHELL`; crear si falta; mensaje distinto. |
| 🟠 | `init.go:178` → `path.go:13` | Dotfiles modificados **automáticamente durante `init`** sin preguntar. | Opt-in o prompt. |
| 🟡 | `path.go:52,25,60` | `powershell` sin ctx (y puede ser `pwsh`); `EqualFold` en Unix; `os.Setenv` inútil. Cero tests. | — |

### 13.11 `assets/embed.go` y baseline

| Sev | Dónde | Problema | Mejora |
| :---: | :--- | :--- | :--- |
| 🟠 | `embed.go:59` + `setup.go` con `overwrite=true` | Pisa `pom.xml`/`package.json` del usuario sin aviso. | Backup o diff-prompt. |
| 🟠 | `scaffold/java/.mvn/settings.xml:9-14` | `<username>maven</username>`, `${env.NEXUS_PASSWORD}`, `{{NEXUS_URL}}` que `ExtractDir` nunca templatea (copia bytes). | Templating real o eliminar (va a `contrib/bank`). |
| 🟡 | `embed.go:73,17-22,16` | Fallback `"3.0.0"` vs `VERSION` = `specforge-3.0.0`; `filepath.Join`+`ToSlash` en vez de `path.Join`; `subDir` sin validar `..`. | Un formato; `path.Join`. |

---

## 14. Tests

**Cobertura real (sin tests de integración, tras PR #2): 35,2 %** · `cmd` 26,6 % · `domain` 59,6 % · `compiler` 40 % · `e2e` 26 % · `storage` 30,5 % · `ingest` 85 % · `quality` 14,5 % · `doc` 4,5 % · `agent`/`auth`/`security`/`system` **0 %**.

| Sev | Problema | Mejora | Estado |
| :---: | :--- | :--- | :---: |
| 🔴 | `loop_test.go:45-89`, `setup_test.go:27-114` ejecutan el `rootCmd` **global** con `os.Chdir`; flags globales reseteados a mano o heredados (`flagSetupStack` del test anterior). Cobra retiene `SetArgs` entre `Execute()`. `runLoop` happy path, `runInterview`, `runAudit`, `runE2E`, `Execute`: **0 %**. El producto no tiene un test con agente/compilador falsos. | Constructores con deps; test de tabla sobre compilador scripted cubriendo: RED compila-y-pasa, RED no compila, RED sin cambios, GREEN con test editado, GREEN agota reintentos, REFACTOR con gate Skipped, **agente pregunta en cada fase**. `t.TempDir()`, `t.Chdir()`. | ⬜ |
| 🟠 | `quality/*_test.go` ejecutan `npx jscpd` (red) y `golangci-lint` real (fallaban); `converter_test` hace `return` si falta markitdown; `TestSetupExtractsQualityConfigs`, `TestFindBrowserExecutable` **no asertan**. | Tras `CommandRunner` falso con fixtures; reales en `-tags integration`. | 🔄 PR #2 mueve los de herramientas a integración y hace asertar dos; `TestFindBrowserExecutable` ⬜ |
| 🟠 | `spec_test.go:74`, `loop_test.go:50`, `diagnostic_test.go`, `setup_test.go:39` | Aserciones sobre strings de UI en español y emoji; `"6.6"` de la constitución. | `errors.Is`. | ⬜ |
| 🟠 | `domain` | 0 % en `DetectProject`, `IgnoreFilter`, `RenderMarkdown`, `ExtractSpecSeal`, `RecordStep`, `CurrentScenarioData`; `HasFailures` solo `"critical"`. Sin tests para `### Scenario:`, Outline, And, CRLF, fallback, `AdvanceToNextPhase` con cero escenarios. Sin `Fuzz*`. Tests de dominio tocan el FS (`os.MkdirTemp`) porque el dominio hace I/O. | Tabla + golden `testdata/specs/*.md` + `FuzzParseScenarios`/`FuzzExtractJSON`. | ⬜ |
| 🟡 | Sin `-race` en scripts, sin umbral, sin `var _ ports.X = (*Impl)(nil)`; `interview_test.go:16` codifica el bug de acentos como esperado. | CI con `-race` y umbral creciente (35 → 60 → 75); aserciones de interfaz. | 🔄 `-race` + umbral 35 en PR #2 |

---

## 15. Idioms, contrato de CLI, dependencias y build

| Sev | Problema | Mejora | Estado |
| :---: | :--- | :--- | :---: |
| 🔴 | **71/71 ficheros CRLF**; `gofmt -l .` los lista todos. | `.gitattributes`; `gofmt -w`; CI. | ✅ PR #2 |
| 🟠 | `chromedp` `// indirect` siendo directo (`go.mod:12-14`). | `go mod tidy` + check en CI. | ✅ PR #2 |
| 🟠 | `go 1.26.0`: el plan inicial proponía bajar a 1.24. **Corrección:** lo exigen `chromedp v0.16`, `cdproto` y `x/oauth2`. Se mantiene; README lee de `go.mod`. El scaffold Go escribe `go 1.24.0` (inconsistente). | Alinear scaffold. | 🔄 |
| 🟠 | Versión `3.0.0` hardcodeada en 10+ sitios; build sin `-X`; `version` dice "Zero-Dependencies"/"Pure Static Go"/"6 Phases". | `internal/buildinfo`; `rootCmd.Version`; texto honesto. | ✅ PR #2 |
| 🟠 | Flags `--FailOn`, `--DryRun` (PascalCase) vs `--fail-on`; `flagConsistencyFailOn` parseado y **nunca leído**; `--fail-on` significa severidad en `audit` y `blocker/warning` en `consistency`. | Kebab-case; borrar muertos. | ⬜ |
| 🟠 | Banners a stdout con emoji; `ingest --print` mezcla datos y banners (`> ctx.md` captura el banner); sin `--json`, `--quiet`, `NO_COLOR`, TTY. | Estado a stderr, datos a stdout; `--output json`; `--quiet`. | ⬜ |
| 🟠 | Exit codes: `Execute()` siempre 1; `consistency` 6 sin documentar; `build --dry-run` `os.Exit(0)` tras imprimir. | 0 ok · 1 uso · 2 gate · 3 integridad · 4 toolchain · **5 pregunta pendiente**; mapa único desde errores tipados; `build` `Hidden` o borrado. | ⬜ |
| 🟠 | Mezcla español/inglés en identificadores, comentarios, flags, errores, logs; `diagnostic.go` referenciaba `docs/GUIA_DE_USO_V3.md` (inexistente) y `~/.sdd/logs/`; `root.go:55` vs `diagnostic.go:114` rutas distintas. | Inglés para código/godoc/errores; tabla de mensajes para UI. | 🔄 referencias ✅ PR #2; idioma ⬜ |
| 🟡 | Alias `sdd`/`forge` en el root (Cobra los ignora) mientras los diagnósticos dicen `sdd loop`. `init.go:181-183` habla de "comando 'sdd'". | Un nombre; quitar alias. | ⬜ |
| 🟡 | `e2e.go:44` `MarkFlagRequired("url")` + check manual; `init.go:39,123` default `europe-west1` hace `!= ""` siempre true ⇒ pisa la detección; `doc.go:30` `--recursive` default true solo se apaga con `=false`; `setup.go:201-214`/`init.go:56-63,91-147` leen stdin sin TTY ⇒ CI se cuelga; `setup` sin `--non-interactive`. | `Flags().Changed`; `--no-recursive`; `term.IsTerminal` + `--non-interactive`. | ⬜ |
| 🟡 | `Short` con "blindaje de tuberías Win32"; sin `Example:`; `commands.go` registra `version`; `build_cmd.go` nombra `buildCommand`; `BuildResult.ErrorOut`. | Nombres consistentes; ejemplos. | ⬜ |
| 🟡 | `x/oauth2` + `compute/metadata` solo para ADC en `init`. | Build tag `nogcp` o `contrib/bank`. | ⬜ |
| 🟡 | 4 scripts de build sin `-X`, sin checksums/SBOM. | `Makefile` + `goreleaser` (checksums). | ✅ PR #2 |
| 🟡 | Sin `.golangci.yml` pese a que el gate lo ejecuta contra proyectos de usuarios. | Añadir con `errcheck`, `funlen`, `gocyclo`, `goconst`, `revive`. | ⬜ |

---

## 16. Flujo y arquitectura objetivo

### 16.1 El ciclo completo con sus puertas

```text
specforge init                        agent, language. Nada más.
specforge setup                       bloque gestionado en CLAUDE.md/GEMINI.md · .specify/config.yaml · specs/

specforge spec new "Password reset"   NNNN asignado · spec.md desde plantilla §6.4
specforge spec interview NNNN         bucle por turnos · una pregunta por turno · interview.jsonl
specforge spec clarify NNNN           resuelve [NEEDS CLARIFICATION] uno a uno → decisions.md
specforge spec lint NNNN              §6.4 en verde
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

### 16.2 Árbol objetivo

```text
cmd/specforge/main.go                 NotifyContext · buildinfo · mapa de exit codes
internal/
  app/                                casos de uso (lo que hoy vive en cmd/)
    interview/  spec/  plan/  tddloop/  deliver/  audit/  e2e/
    protocol/   response.go            §3.1: done | needs_clarification | blocked
    review/     gates.go               R0-R4
  domain/                             PURO
    spec/       (gherkin oficial) seal.go lint.go sections.go
    tdd/        state.go outcome.go errors.go
    stack/      profile.go detect.go
    quality/    result.go thresholds.go
    delivery/   report.go trace.go
  ports/        agent exec state_repo spec_repo vcs logger prompter
  adapters/
    agent/      cli_runner.go (claude, gemini; stdin; JSON)
    exec/       runner.go
    vcs/        git.go
    storage/    state_fs.go(atómico) spec_fs.go lessons_fs.go decisions_fs.go config.go
    quality/    jscpd knip stryker linter archunit (JSON reporters)
    testparse/  gojson.go surefire.go vitest.go junit.go
    e2e/        chromedp_driver.go vision_agent.go
    security/   auditor.go (jsonschema)
  buildinfo/    ✅ (PR #2)
  ui/           spinner box prompt messages_{es,en}
assets/
  prompts/{es,en}/   interview plan red green refactor vision audit/*  (text/template, contrato §3.1)
  templates/         spec.md plan.md delivery.md pr_body.md
  baseline/{go,react,java,python}/
contrib/bank/       tekton nexus consistency adc path   (fuera del core)
```

---

## 17. Roadmap por fases y estado

Cada fase se cierra con una *definición de hecho* y una **comprobación de principios**.

### Fase 0 — Higiene y red de seguridad ✅ (PR #2) · P5, P6

- [x] LF + `.gitattributes`; `gofmt -w` (71 → 0 ficheros marcados).
- [x] `go mod tidy` (`chromedp` directo). `go 1.26` se mantiene: lo exigen las dependencias.
- [x] Tests con herramientas externas → `-tags integration`; los que no asertaban ahora asertan.
- [x] `internal/buildinfo` con `-X`; `version`/`--version`; estado y config sin `"3.0.0"` fijo.
- [x] `Makefile` + `.goreleaser.yaml` sustituyen a los 4 scripts.
- [x] CI: `gofmt -l`, `go vet`, `tidy -diff`, `go test -race -cover` con suelo 35 %, build Linux/macOS/Windows.
- [x] Referencias y afirmaciones falsas corregidas en `USER_GUIDE.md`, `diagnostic.go`, `audit.go`.
- [ ] `golangci-lint` en CI con `.golangci.yml` (errcheck, funlen, gocyclo, goconst, revive).
- [ ] Scaffold Go alineado a `go 1.26`.
- **Hecho cuando:** CI verde en Linux/macOS/Windows en un clon limpio. ✅ verificado localmente; pendiente la primera ejecución en GitHub Actions al fusionar.

### Fase 1 — Que las puertas se nieguen y la IA pregunte ✅ (PR #3) · P1, P3, P5

- [x] **Protocolo §3.1** en `internal/app/protocol`; todos los prompts terminan con el contrato; el runtime gestiona los tres estados; `decisions.md`; exit 5.
- [x] **Preguntas sin terminal reanudables**: la pregunta queda en `questions.md`; la respuesta se escribe ahí o en la terminal; `--resume` continúa **el mismo paso** con la línea base guardada (lo que el agente escribió antes de preguntar cuenta) y no llama al agente mientras no haya respuesta. Hallazgo de la ejecución real con Claude Code.
- [x] Bloque gestionado en `CLAUDE.md`/`GEMINI.md` con la regla §3.3 y el estándar del stack (`setup`); el prompt va por stdin.
- [x] `ports.CommandRunner` + `adapters/process` (timeout, `WaitDelay`, búfer de cola, `.cmd/.bat` en Windows); un único adaptador de agente para Claude y Gemini; `acceptEdits`/`auto_edit` (antes el agente headless **no podía escribir**).
- [x] `signal.NotifyContext` en `main`; contexto en todos los pasos; chromedp con contexto y cancelación.
- [x] `TestOutcome` por stack (test2json, Surefire/JUnit XML, Vitest/Jest JSON, pytest JUnit); RED = compila, ejecuta y falla por aserción; snapshot de ficheros antes/después; `files_written` verificado; `ErrUnsupportedStack`.
- [x] Hash de tests tras RED + `TamperingError` en GREEN/REFACTOR.
- [x] `LastFailure` llega al primer GREEN; reintentos con error head+tail; `Attempts` a 0 al reanudar.
- [x] Gates tri-estado con reporters JSON; umbrales en `specforge.yaml`; `strict`; SonarGate eliminado; `npx --no-install`.
- [x] Auditor fail-closed con `jsonschema/v6`; `--end-of-options` y refs validadas; `needs_validation` → pregunta; informes 0600.
- [x] Estado atómico; config de usuario 0600 en `os.UserConfigDir()`; `slog` + lumberjack; `--trace-io`.
- [x] Parser: `cucumber/gherkin/go` sobre bloques ```` ```gherkin ````; `ErrNoScenarios`; preguntas abiertas estructuradas; golden + fuzz.
- [x] E2E: acciones tipadas validadas (selector ∈ snapshot, mismo origen), evidencia verificada en Go, informe por escenario, capturas por paso. *(Adelantado de la Fase 4.)*
- [x] CLI nueva sobre la raíz de composición: `init`, `setup`, `spec new|interview|lint|approve|list`, `loop`, `audit`, `e2e`, `version`; códigos de salida 0-5/130 desde errores tipados. *(Adelantado de las Fases 2 y 3: composition root, `spec new/lint/approve`, contrato de CLI.)*
- [x] Eliminado el código v3 (dominio con I/O, adaptadores duplicados, ADC/GCP, PATH, Tekton, Nexus, `consistency`, `build`, `doc`, `ingest`, scaffolds con dependencias de banco). *(Adelantado de la Fase 3.)*
- **Hecho:** tests de aplicación con agente y runner falsos cubren los casos de §14, incluido "el agente pregunta en cada fase y el runtime pausa"; `cmd` 79 %, total 80 %; ejecución real completa con Claude Code (2 escenarios, RED → GREEN → REFACTOR, exit 0).

### Fase 2 — Spec clara, plan y revisión ✅ (PR #4) · P2, P7, P4

- [x] `internal/app` + raíz de composición; dominio sin I/O; borrados `LoopState`/`StatePhase`/`BDDFeature`/`CalculateSHA256`/`MainframeConfigurator`. *(En PR #3.)*
- [x] Plantilla §6.4 y `spec lint`; `spec new` (NNNN); `spec approve` (R0); `interview` no sella; slug con acentos. *(En PR #3.)*
- [x] **`plan` + R1**: el agente solo puede escribir `plan.md` (cualquier otro fichero ⇒ rechazo); cada marcador de escenario debe tener test planificado; `plan approve` sella; el loop exige el plan aprobado si existe y lo inyecta en todos los prompts.
- [x] Prompts `text/template` en `{es,en}` con bloques compartidos (`contract`, `context`, `turn`); error head+tail. Conversación con el agente extraída a `app/conversation` y compartida por loop y plan.
- [x] **R2 por escenario** (`review: scenario | off`, `--review`): aceptar · escribir el cambio (vuelve a GREEN con la nota) · volver a RED. Sin terminal es una pregunta en `questions.md` (exit 5) que se responde ahí.
- [x] **Commit por escenario** `feat(SDD_NNNN_KKK): título` solo con sus ficheros (pathspec literal, `--only`, respeta hooks y firma); `commit: false` / `--no-commit`; ficheros y commit quedan en el estado para la traza.
- [x] `--scenario k --from red|green|refactor`.
- [x] Sello unificado `sha256-v1` normalizado y anclado; `--resume` revalida y conserva los escenarios sin cambios. *(En PR #3.)*
- [x] **`spec clarify`**: pregunta cada `[NEEDS CLARIFICATION]` y escribe `- **Decided:** pregunta → respuesta (fecha, quién)` en su lugar; también en `decisions.md`.
- [x] **Enmiendas** = editar + `spec approve` de nuevo: `approvals.md` registra cada aprobación con su delta ADDED/MODIFIED/UNCHANGED/REMOVED y el loop rehace solo lo cambiado. *Decisión: sin comando `spec amend` aparte (P6, menos superficie).*
- [x] **Lecciones** escritas por el agente (`lesson` en el contrato) solo tras un intento rechazado; `specs/LESSONS.md` versionado, por stack, sin duplicados, tope 30; se inyectan las del stack actual.
- [x] Glosario e invariantes desde la propia spec. *(En PR #3.)*
- **Hecho:** ejecución real con Claude Code de `plan` (plan mínimo, un test por escenario, riesgo de redondeo señalado en vez de inventado) y del loop con R2 en terminal y un commit por escenario; tests de aplicación y de CLI para cada rama (cambio, vuelta a RED, salto, lección, clarify, delta).

### Fase 3 — Entrega y recorte ✅ (PR #5) · P8, P6

- [x] **`deliver`** (R4): `DELIVERY.md`, `trace.json` y `PR_BODY.md` solo desde artefactos (spec y plan sellados, estado del loop casado por huella de escenario, `decisions.md`, `questions.md`, `LESSONS.md`, `docs/security/findings.json`, `docs/e2e/<spec>/report.json`). Cada fila: tests con sus nombres por marcador, commit, puertas (✓ · ⚠ omitida · ✗) y notas. Lo incompleto se dice en la primera línea. Respeta la plantilla de PR del repo.
- [x] Las preguntas de proceso (revisión R2, verificación de RED) se registran como `REVIEW`/`VERIFY` y no se mezclan con las decisiones de producto en la entrega.
- [x] Recorte: ADC/GCP, PATH, Tekton, Nexus, `consistency`, `build`, `doc`, `ingest`, alias, `openspec`, scaffolds. *Decisión: borrados en lugar de `contrib/bank` (P6); siguen en la historia de git, etiqueta v3.0.0.* *(En PR #3.)*
- [x] Contrato de CLI: kebab-case, estado a stderr y datos a stdout, **`--json`** (datos de `version`, `spec list`, `deliver` y errores como `{exit,title,cause,action}`), `--quiet`, códigos de salida, `--non-interactive`.
- [x] `os.UserConfigDir()`, `SPECFORGE_HOME`, un nombre, godoc en inglés. *(En PR #3.)*
- [x] **`.golangci.yml`** (errcheck, errorlint, gocyclo ≤ 25, revive, staticcheck, unparam, unconvert, gofmt) en CI con `golangci-lint-action` compilado con la Go de `go.mod`; 0 avisos. Refactor de `red()` (turno + verificación) y de `Diagnose` (clasificadores por familia) para cumplirlo, sin subir umbrales.
- [x] Test que comprueba que toda clave de mensaje usada en el código existe en el catálogo.
- **Hecho:** `deliver` sobre el proyecto real construido por Claude Code: 2/2 escenarios, cada fila con su test y su commit reales.

### Fase 4 — Entrevista por turnos y E2E real ✅ (PR #6) · P1, P2, P3

- [x] **`spec interview` propiedad de la herramienta**: cada turno es una llamada headless con contrato (`question`, `context` = por qué importa, `section`, `unknowns`); el agente escribe la respuesta anterior en la spec y devuelve la siguiente pregunta o `done`. Transcripción en `specs/NNNN/interview.jsonl` y respuestas en `decisions.md`. Solo puede cambiar la spec (otro fichero ⇒ error). Termina **solo** cuando el lint no encuentra `TODO` ni estructura ausente: un `done` prematuro se devuelve con los problemas (máx. 3 rondas). Lo que no se sabe queda como `[NEEDS CLARIFICATION]`, nunca inventado. Sin terminal: pregunta en `questions.md`, exit 5, y la siguiente ejecución continúa. `--chat` conserva la conversación libre con el agente.
- [x] E2E por escenario con `then_index`, evidencia verificada en Go, `report.json`, capturas por paso, `--min-pass-rate`; selectores sin mutar el DOM; certificados opt-in; `CHROME_PATH`. *(Adelantado en PR #3.)*
- [x] **Demo**: `docs/DEMO.md` con las sesiones reales con Claude Code (entrevista, plan, loop con revisión, entrega) y la tabla de defectos que destaparon.
- [ ] **v4.0.0**: publicar la release es decisión del mantenedor (etiqueta + `goreleaser`); todo lo necesario está listo.
- **Hecho:** entrevista real en modo CI: 4 preguntas ⇒ spec completa con 8 invariantes, cada una con su escenario de fallo, 12 escenarios, contratos, errores, fuera de alcance y supuestos con quién los confirmó; `spec approve` sin un solo aviso.

---

## 18. Métricas: v3 → v4 (medidas en la rama de la Fase 4)

| Métrica | v3 | Objetivo | v4 medido | Principio |
| :--- | :---: | :---: | :---: | :---: |
| Fases en las que el agente puede preguntar | 1 de 7 | 7 de 7 | 7 de 7 (entrevista, plan, RED, GREEN, REFACTOR, E2E, audit) ✅ | P1 |
| Puertas de revisión humana registradas | 0 | 5 (R0-R4) | 5: `spec approve`, `plan approve`, revisión por escenario, preguntas, `deliver` ✅ | P2 |
| Puertas que aprueban sin ejecutarse | 5 | 0 | 0: una herramienta ausente es ⚠ *skipped*, y bloquea con `strict` ✅ | P3 |
| Imports de `os`/`os/exec` en `domain` | 7 ficheros | 0 | 0 ✅ | P4 |
| Funciones > 60 líneas | 12 | 0 | 8 (orquestadores de casos de uso y el catálogo de mensajes) 🔄 | P4, P5 |
| `_ =` sobre escrituras | ~25 | 0 | 0 (solo limpiezas best effort comentadas) ✅ | P5 |
| Cobertura total / `cmd` / ejecución de procesos | 35 / 27 / 0 % | ≥ 75 / 70 / 60 % | 77 / 81 / 90 % ✅ | P5 |
| Avisos de `golangci-lint` | sin configurar | 0 | 0, en CI ✅ | P5 |
| Ficheros CRLF / `gofmt -l` | 71 | 0 | 0 ✅ | P5 |
| Parsers/matchers/loggers caseros con librería madura disponible | 6 | 0 | 0 (Gherkin oficial, jsonschema, slog + lumberjack, x/term, x/text) ✅ | P6 |
| Comandos del CLI | 11 | 8 | 8 de trabajo + `version` ✅ | P6 |
| Reglas de lint de spec | 0 | 12 secciones | 5 bloqueantes + 3 consejos (secciones, invariantes, varios `When`) ✅ | P7 |
| Memoria inyectada | ilimitada | 30 lecciones + decisiones | 30 lecciones por stack + decisiones + plan ✅ | P7 |
| Artefactos de entrega | 0 | 3 | 3 (`DELIVERY.md`, `trace.json`, `PR_BODY.md`) ✅ | P8 |
| Referencias al banco en el core | ~40 % | 0 | 0 ✅ | P6 |
| Afirmaciones del README/guía no reproducibles | 12 | 0 | 0: cada salida de la documentación procede de una ejecución real ✅ | P8 |
| Código Go (producción / tests) | 5.628 / 1.242 | — | 12.685 / 5.601 | — |

--- | :---: | :---: | :---: |
| Fases en las que el agente puede preguntar | 1 de 7 | 7 de 7 | P1 |
| Puertas de revisión humana registradas | 0 | 5 (R0-R4) | P2 |
| Puertas que aprueban sin ejecutarse | 5 | 0 | P3 |
| Falsos "YAGNI" / falsos "superado" reproducibles | 4 | 0 | P3 |
| Imports de `os`/`exec`/`filepath` en `domain` | 7 ficheros | 0 | P4 |
| Funciones > 60 líneas | 12 | 0 | P4, P5 |
| `_ =` sobre escrituras | ~25 | 0 | P5 |
| Cobertura total / `cmd` / adaptadores exec | 35 / 27 / 0 % | ≥ 75 / 70 / 60 % | P5 |
| Ficheros CRLF / `gofmt -l` | ~~71~~ 0 ✅ | 0 | P5 |
| Parsers/matchers/loggers caseros con librería madura disponible | 6 | 0 | P6 |
| Comandos del CLI | 11 | 8 | P6 |
| Scripts de build | ~~4~~ Makefile + goreleaser ✅ | — | P6 |
| Secciones obligatorias de spec con lint | 0 | 12 | P7 |
| Memoria inyectada | ilimitada | 30 lecciones + decisiones | P7 |
| Artefactos de entrega | 0 | 3 | P8 |
| Referencias al banco en el core | ~40 % | 0 | P6 |
| Afirmaciones del README/guía no reproducibles | ~~12~~ 0 ✅ | 0 | P8 |

---

## 19. Las diez cosas que haría mañana, en orden

1. **Protocolo `done | needs_clarification | blocked`** en todos los prompts y el runtime que pausa y pregunta (§3). ~150 líneas. Sin esto, todo lo demás sigue inventando.
2. **Hash de tests entre fases** + regla en el bloque gestionado (`loop.go`). 40 líneas.
3. **`TestOutcome` y RED "falla por aserción"** (`dispatcher.go`, `loop.go:199-254`).
4. **Gates tri-estado** y borrar SonarGate (`quality/*`).
5. **Auditor fail-closed** + `--end-of-options` (`cloudflare_auditor.go:108-114,134`).
6. **Prompt por stdin** y `cliRunner` único (`agent/*`). Sin esto no funciona en proyectos reales.
7. **`state.Save` atómico y propagado** (`state.go`, `loop.go`).
8. **`cucumber/gherkin/go`** en lugar del parser casero (`spec.go`). Desaparecen tres bugs ✔.
9. **`spec approve`** (R0) y que `interview` no selle (`interview.go:194-212`).
10. ~~LF + `go mod tidy` + CI + goreleaser.~~ ✅ Hecho en PR #2. Nuevo 10: **`CommandRunner` + `internal/app/tddloop`** con tests scripted. Desbloquea todo lo demás.

---

## Anexo A · Reproducción de los bloqueantes verificados

Todos sobre `main` (`71334e4`), binario compilado con `go build -o sf .`.

**A.1 Parser mutila el español y colapsa el formato de la plantilla**
```go
s,_ := ParseScenarios("Scenario: X\nDado un saldo de 100\nCuando se retira 30\nEntonces el saldo es 70\n")
// GIVEN=["un saldo de 100"] WHEN=["o se retira 30"] THEN=["ces el saldo es 70"]
s2,_ := ParseScenarios("### Scenario: Y\nGiven a\nWhen b\nThen c\n")
// len=1 title="Requerimiento General de la Especificación"
```

**A.2 Bloqueo por `[NEEDS CLARIFICATION]` (funciona) y sello roto (funciona)**
```bash
specforge loop --spec specs/0001-password-reset.md     # con una línea [NEEDS CLARIFICATION]
# 🛑 BLOQUEO DURO … 1 cuestión(es) abierta(s)
sed -i 's/30 minutes/24 hours/' specs/0001-password-reset.md
specforge loop --spec specs/0001-password-reset.md
# Error: el sello criptográfico no coincide (esperado: f15155…, actual: be9548…)
```

**A.3 jscpd aprueba sin ejecutarse** — `internal/adapters/quality/jscpd.go:37-40`
```go
if err != nil && res.ExitCode == 1 && res.Output == "" {
    return true, "jscpd: Omitido (npx/jscpd no disponible en este entorno).", nil
}
```

**A.4 Auditor convierte un parse error en informe vacío** — `cloudflare_auditor.go:108-114`
```go
report, err := domain.ValidateFindingsAgainstSchema([]byte(rawValidation), nil)
if err != nil {
    logger.Warn("Aviso parseando esquema estricto: %v. Generando estructura de recuperación.", err)
    report = &domain.SecurityReport{Findings: make([]domain.SecurityFinding, 0)}
}
```

**A.5 Logger escribe todos los niveles al fichero** — `storage/logger.go:94-109`
```go
if err := l.ensureFile(); err == nil && l.file != nil { _, _ = io.WriteString(l.file, line) }   // siempre
if (level == LevelDebug && l.debugMode) || … { fmt.Fprint(os.Stderr, line) }                     // solo stderr filtra
```

**A.6 Tests de calidad fallan en un clon limpio** (antes de PR #2)
```text
--- FAIL: TestLinterGateGo — Go language version (go1.25) used to build golangci-lint is lower than the targeted Go version (1.26.0)
--- FAIL: TestCompositeQualityGateGo — idem
```

**A.7 Métricas de código**
```text
LOC producción 5.628 · tests 1.242 · cobertura 38,4 % (con integración) / 35,2 % (sin)
runLoop 265 líneas · runSetup 218 · runInterview 170 · runInit 146 · RunVisualSpec 136 · DiagnoseError 135
gofmt -l . → 71 ficheros (CRLF)   →   0 tras PR #2
```

---

## Anexo B · Índice de hallazgos por fichero

| Fichero | Secciones |
| :--- | :--- |
| `main.go` | 12.3 |
| `console_windows.go` | 12.3 |
| `go.mod` | 15 |
| `cmd/root.go` | 12.3, 15 |
| `cmd/loop.go` | 1, 3, 7.1, 8, 9, 12.1, 12.3, 14 |
| `cmd/interview.go` | 6.1, 6.2, 12.1 |
| `cmd/setup.go` | 5.2, 8, 9, 12.3, 15 |
| `cmd/init.go` | 5.2, 12.2, 13.10, 15 |
| `cmd/audit.go` | 7.3, 12.3, 15 |
| `cmd/e2e.go` | 7.4, 15 |
| `cmd/ingest.go`, `cmd/doc.go` | 12.1, 15 |
| `cmd/consistency.go`, `cmd/build_cmd.go` | 5.2, 12.1, 12.3, 15 |
| `cmd/commands.go` | 15 (✅ PR #2) |
| `internal/domain/spec.go` | 6.2, 6.3, 12.2 |
| `internal/domain/state.go` | 7.1, 12.2, 12.3 |
| `internal/domain/config.go`, `project.go`, `seal.go`, `context.go`, `agent_memory.go`, `diagnostic.go`, `security.go` | 7.3, 9, 12.1, 12.2, 13.6 |
| `internal/ports/*` | 12.1 |
| `internal/adapters/agent/*` | 8, 13.1 |
| `internal/adapters/auth/*` | 13.2 |
| `internal/adapters/compiler/*` | 7.1, 13.3 |
| `internal/adapters/doc/*` | 13.4 |
| `internal/adapters/e2e/*` | 7.4 |
| `internal/adapters/ingest/*` | 13.6 |
| `internal/adapters/quality/*` | 7.2 |
| `internal/adapters/security/*` | 7.3 |
| `internal/adapters/storage/*` | 13.9 |
| `internal/adapters/system/*` | 13.10 |
| `assets/embed.go`, `assets/baseline/**` | 5.2, 9, 13.11 |
| `USER_GUIDE.md`, `README.md` | 1, 15 (✅ PR #1, #2) |

---

## Anexo C · Estado de PRs

| PR | Contenido | Fase | Estado |
| :--- | :--- | :---: | :--- |
| [#1](https://github.com/jefmonjor/specforge/pull/1) | README rediseñado + este plan | — | Abierta |
| [#2](https://github.com/jefmonjor/specforge/pull/2) | Higiene y red de seguridad | 0 | Abierta · fusionar **antes** que cualquier otra |
| #3 | Núcleo verificable, CLI nueva y recorte del código v3 | 1 | Abierta, apilada sobre #2 |
| #4 | Plan (R1), revisión (R2), commit por escenario, clarify, historial de aprobaciones, lecciones | 2 | Abierta, apilada sobre #3 |
| #5 | `deliver`, `--json`, golangci-lint | 3 | Abierta, apilada sobre #4 |
| #6 | Entrevista por turnos, demo real y cierre del plan | 4 | Abierta, apilada sobre #5 |

---

*Generado a partir de la lectura completa del repositorio en `main` (commit `71334e4`), ejecución de `go build`, `go vet`, `go test -cover`, y reproducción manual de los fallos marcados con ✔. Los ocho principios de §0 son la vara con la que debe medirse cualquier PR futura, incluidas las que salgan de este plan.*
