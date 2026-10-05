# SpecForge 6 · Plan: proporcionalidad, revisión verificable y seguridad operativa

> **Estado: ejecutado.** Los doce puntos están implementados y probados con Claude Code; el detalle de lo hecho y de lo que encontraron las pruebas reales está en [IMPROVEMENT_PLAN.md](IMPROVEMENT_PLAN.md) (Fase 6). Diferencias con este diseño: la guardia vive en `app/guardhook` sin puerto propio (es JSON sobre `ports.Files`); el puerto `VCS` se dividió en `DiffSource`, `Recorder` y `Measurer`; los escenarios en paralelo usan sandboxes (copias con su propio git) en lugar de `git worktree`, porque así incluyen el trabajo sin commit; el verificador de un escenario responde por las invariantes que el escenario nombra, y `verify: feature` por todas.

> Continúa [IMPROVEMENT_PLAN.md](IMPROVEMENT_PLAN.md) (Fases 0–5). Nace del análisis de [gentle-shell](https://github.com/Gentleman-Programming/gentle-shell) (v4.0.0, 4 de octubre de 2026): 803 ficheros, 64k líneas de TypeScript, 4.428 tests, prompts de orquestación muy trabajados y una revisión por lentes que vive en un binario Go externo. De ahí salen doce ideas. Este plan dice cómo integrarlas **sin perder lo que nos distingue**: SpecForge comprueba lo que la IA hace; no se fía de lo que dice.

- [0. Principio rector y reglas del plan](#0-principio-rector-y-reglas-del-plan)
- [1. Resumen: los doce puntos](#1-resumen-los-doce-puntos)
- [2. Arquitectura objetivo](#2-arquitectura-objetivo)
- [3. Fase 6.1 · Proporcionalidad y entorno (puntos 1–7)](#3-fase-61--proporcionalidad-y-entorno-puntos-17)
- [4. Fase 6.2 · Revisión verificable y seguridad (puntos 8–10)](#4-fase-62--revisión-verificable-y-seguridad-puntos-810)
- [5. Fase 6.3 · Paralelismo y revisión ciega (puntos 11–12)](#5-fase-63--paralelismo-y-revisión-ciega-puntos-1112)
- [6. Cambios transversales](#6-cambios-transversales)
- [7. Orden de ejecución, PRs y calendario](#7-orden-de-ejecución-prs-y-calendario)
- [8. Riesgos y mitigaciones](#8-riesgos-y-mitigaciones)
- [9. Lo que no se hace](#9-lo-que-no-se-hace)
- [10. Métricas de éxito](#10-métricas-de-éxito)
- [Anexo A · De dónde sale cada idea](#anexo-a--de-dónde-sale-cada-idea)
- [Anexo B · Contratos JSON nuevos](#anexo-b--contratos-json-nuevos)

---

## 0. Principio rector y reglas del plan

**Lo que verificamos nosotros, lo seguimos verificando.** gentle-shell tiene mejores prompts y una revisión más rica, pero su disciplina TDD es solo texto: nadie comprueba que el RED falló. Nosotros sí. Cada idea que importamos entra con una pregunta obligatoria: *¿qué comprueba SpecForge con código, y qué pasa cuando el agente miente?* Si la respuesta es "nada", la idea se queda en un prompt y la documentación lo dice así.

Reglas, heredadas de los principios P1–P8 del plan anterior:

1. **Si está en el README, está en el código.** Ninguna sección de este plan promete algo que no tenga un test que lo demuestre.
2. **Fail-closed.** Una respuesta del agente que no cumple el esquema no es "casi bien": se reintenta una vez y después para con exit 2.
3. **Dominio puro.** Riesgo, diff, hallazgos, guardia de comandos: todo en `internal/domain`, sin E/S, testeable con tablas.
4. **El agente puede subir la exigencia, nunca bajarla.** Riesgo, revisión, verificación: solo se escalan.
5. **Proporcional por defecto, estricto si se pide.** Un cambio de documentación no paga la revisión de un cambio de autenticación. `--strict` y la configuración siempre pueden exigir más.
6. **Una ronda de corrección.** Como en gentle-shell: una corrección acotada, una validación dirigida, y si sigue fallando, decide la persona. Sin bucles.
7. **Sin vocabulario nuevo.** Hallazgo, prueba, lente, riesgo, verificador. Nada de "authority", "receipt" ni "lineage".
8. **Cada PR deja la CI verde y la demo real al día.** Las pruebas con Claude Code son parte de "hecho".

---

## 1. Resumen: los doce puntos

| # | Punto | Fase | Coste | Principios | Lo que SpecForge comprueba con código |
| :---: | :--- | :---: | :---: | :--- | :--- |
| 1 | Fallos preexistentes como evidencia | 6.1 | 2 d | P3, P7 | Nombres de los tests que fallan en la línea base, leídos del informe del runner; solo bloquean los nuevos |
| 2 | Riesgo por nivel, determinista | 6.1 | 2 d | P3, P4 | Nivel calculado de rutas y líneas; el agente solo puede subirlo |
| 3 | `doctor` | 6.1 | 1 d | P8 | Cada herramienta, con versión y cómo instalarla; exit 4 si falta algo necesario |
| 4 | Preguntas con opciones derivadas | 6.1 | 0,5 d | P1 | Pregunta sin opciones en loop/plan/legacy → rechazada y repetida |
| 5 | Superficies de edición desde el plan | 6.1 | 2 d | P2, P3 | Escrituras fuera de los componentes del plan → pregunta o rechazo |
| 6 | Presupuesto de entrega | 6.1 | 1 d | P8 | Líneas autorizadas por escenario desde git; cortes de PR propuestos |
| 7 | Modelo por fase | 6.1 | 1 d | P6 | El modelo de cada fase llega al agente (test con agente guionizado) |
| 8 | Revisión por lentes (R3) con hallazgos causales | 6.2 | 8 d | P1, P3, P5 | Cada `proof_ref` apunta a un hunk real del diff; esquema JSON; presupuesto de corrección |
| 9 | Verificador independiente desde la spec | 6.2 | 5 d | P3, P7 | Veredicto por cada INV y escenario; proyecto intacto (hash); comandos con salida observada |
| 10 | Guardia de comandos destructivos | 6.2 | 3 d | P3 | Tokenizador puro con tabla de casos; hook instalado y comprobado por `doctor` |
| 11 | Escenarios en paralelo | 6.3 | 10 d | P4 | Superficies disjuntas comprobadas antes de lanzar; un worktree por escenario |
| 12 | Revisión ciega doble | 6.3 | 3 d | P3 | Dos lentes independientes; solo lo que ambas prueban bloquea sin refutador |

Total estimado: **6.1 ≈ 2 semanas · 6.2 ≈ 3–4 semanas · 6.3 cuando 6.1 y 6.2 estén en producción.**

---

## 2. Arquitectura objetivo

Se mantiene la hexagonal de v4/v5. Lo nuevo, por capa:

```text
internal/
  domain/
    tdd/        Outcome.Failures (nombres) · Baseline · SurfaceCheck
    risk/       NUEVO · Tier, Classify, Rules, Escalate
    review/     NUEVO · Diff (hunks), Finding, ProofRef, Verify,
                CorrectionBudget, Ledger
    guard/      NUEVO · Recognize (tokenizador de comandos)
    delivery/   AuthoredLines, Slices
    spec/       PlanComponents (superficies desde el plan)
  app/
    tddloop/    fases: RED → GREEN → REFACTOR → REVIEW → (human) → COMMIT
    review/     NUEVO · lentes, refutador, corrección, validación
    verify/     NUEVO · verificador en copia de trabajo
    doctor/     NUEVO · comprobaciones del entorno
    deliver/    presupuesto y cortes
    setup/      instala el hook de la guardia
  ports/        Scratch (copia de trabajo), Hooks (instalación), DiffSource
  adapters/
    testrun/    nombres de tests fallidos por runner
    scratch/    NUEVO · copia del proyecto para el verificador
    hooks/      NUEVO · settings de Claude Code / Gemini CLI
    vcs/        numstat por commit, diff -U0 por rango
assets/
  prompts/{en,es}/  review_<lens>.md, refute.md, correct.md,
                    validate.md, verify.md
  review/           schema.json (hallazgos), verify-schema.json
```

Flujo del loop en v6, por escenario:

```text
RED → GREEN → REFACTOR (suite − línea base, gates)
   → riesgo = clasificar(ficheros, líneas, escalado del agente)
   → REVIEW  (pasivo: ninguna · medio: 1 lente · alto: 4 lentes)
        hallazgos → verificar proof_refs contra el diff
        inferenciales graves → refutador
        bloqueantes → 1 corrección (presupuesto) → validación dirigida
   → VERIFY  (solo alto, o bajo demanda)
   → revisión humana R2 (según review: scenario | risk | off)
   → commit feat(SDD_…)
```

Códigos de salida: sin cambios. Un bloqueo de revisión o de verificación es `2`, como los gates.

---

## 3. Fase 6.1 · Proporcionalidad y entorno (puntos 1–7)

### 3.1 Fallos preexistentes como evidencia, no como bloqueo

**Problema.** REFACTOR exige que pase toda la suite. Un repositorio con un test roto antes de empezar no puede usar SpecForge, y el agente acaba intentando arreglar lo que no es suyo.

**Diseño.**

- `tdd.Outcome` gana `Failures []TestRef{Name, File string}`. Cada runner los extrae de su informe: `go test -json` (acción `fail` por `Test`), Surefire/JUnit XML (`testcase` con hijo `failure`/`error`: `classname#name`), Vitest/Jest JSON (`assertionResults[].fullName` con `status: failed`), pytest JUnit XML. Con `Exact=false` (`npm test` plano) no hay nombres y todo sigue como hoy.
- `tdd.State` gana `Baseline *Baseline{At time.Time; Command string; Failures []TestRef}`. Se calcula al arrancar el loop (y con `--restart`), ejecutando la suite completa una vez antes del primer RED. Se muestra: `⚠ 3 test(s) already fail on this branch and will not block: …`.
- Regla de REFACTOR: `fallos(suite) − Baseline.Failures = ∅`. Un test de la línea base que sigue fallando es un aviso; uno nuevo bloquea como hoy. Un test de la línea base que empieza a pasar se retira de ella (se persiste).
- Prompt (`context.md`): sección `## Known failures on this branch (not yours to fix)` con la lista, para que el agente no los persiga.
- `deliver`: sección *Known failures* en DELIVERY.md y campo `baseline` en trace.json.

**Lo que SpecForge comprueba.** Los nombres salen del informe del runner, no de la palabra del agente. La resta es determinista.

**Tests.** Fixtures por runner con dos fallos nombrados; `tddloop`: un fallo preexistente no bloquea REFACTOR, uno nuevo sí, uno preexistente que pasa se retira; `Exact=false` no crea línea base.

**Hecho cuando** la demo de Go arranca con un test roto a propósito, el loop completa los escenarios y DELIVERY.md lo lista como conocido.

### 3.2 Riesgo por nivel, determinista

**Problema.** Todo escenario paga la misma ceremonia. gentle-shell clasifica por rutas y líneas, sin modelo, y con eso decide cuánta revisión aplicar.

**Diseño.**

- `internal/domain/risk` (puro):

  ```go
  type Tier string // passive | medium | high
  type Change struct{ Path string; Added, Deleted int; Binary bool }
  type Rules struct {
      MaxLines  int      // default 400
      HighPaths []string // regex sobre segmentos de ruta
      DocsOnly  []string // extensiones y rutas pasivas
  }
  type Assessment struct{ Tier Tier; Lines int; Reasons []string }
  func Classify(changes []Change, r Rules) Assessment
  func Escalate(a Assessment, to Tier, why string) Assessment // solo sube
  ```

  Reglas por defecto: **alto** si `Lines > MaxLines` o algún segmento de ruta casa con `auth|authn?|authz|security|secret|credential|token|password|payment|billing|permission|acl|shell|process|exec|migration|infra|deploy|ci|Dockerfile|go.mod|package.json|pom.xml|build.gradle|pyproject.toml`; **pasivo** si todos los ficheros son documentación (`.md`, `.txt`, `.rst`, `docs/`) o comentarios; **medio** el resto. `Reasons` explica el nivel ("pom.xml changed", "612 lines > 400").
- Se evalúa tras GREEN sobre los ficheros del escenario (`git diff --numstat` desde el inicio del escenario) y se guarda en `ScenarioRef.Risk`.
- Escalado por el agente: `protocol.Response` gana `risk: "high"` y `risk_reason`. Solo sube. Se registra en `decisions.md` como decisión del agente.
- Efectos, todos configurables en `specforge.yaml`:
  - `review: scenario | risk | off`. Nuevo valor `risk`: revisión humana R2 solo en medio y alto; un escenario pasivo se acepta con una línea en la salida.
  - Lentes (3.8): pasivo 0 · medio 1 · alto 4.
  - Verificador (3.9): solo alto, salvo `verify: always`.
  - Gates lentos (Stryker): `gates.mutation_from: medium`.
- `deliver` muestra el nivel por escenario y sus razones; trace.json también.

**Lo que SpecForge comprueba.** El nivel es una función pura de git; el agente no lo elige.

**Tests.** Tabla de clasificación (20 casos: rutas, líneas, binarios, docs); `Escalate` nunca baja; `tddloop` con `review: risk` salta R2 en pasivo y la exige en medio.

**Hecho cuando** un escenario que solo toca `README.md` termina sin revisión humana ni lentes, y uno que toca `auth/` exige ambas.

### 3.3 `doctor`

**Diseño.** `specforge doctor [--json]` en `internal/app/doctor` con un puerto de sondas (para tests con fakes):

| Comprobación | Necesaria | Pista si falta |
| :--- | :---: | :--- |
| Agente configurado (`claude`/`gemini`) en PATH, con versión y sesión iniciada | sí | `specforge init`, instalar el CLI |
| `git` y `user.name` | sí | — |
| Runner del stack: `go`, `mvn`/`mvnw`, `gradle`/`gradlew`, `node`+`npm`, `pytest` (del `.venv` primero) | sí | comando de instalación por SO |
| Gates: `golangci-lint`, `ruff`, `jscpd`, `knip`, Stryker configurado, `maven-pmd-plugin` en el pom | no | el mismo texto que hoy muestra el gate saltado |
| Chrome/Chromium/Edge o `CHROME_PATH` | no | solo para `e2e` |
| `specforge.yaml` válido; `migration.legacy` existe | sí si está | — |
| Hook de la guardia instalado (3.10) | no | `specforge setup` |
| Versión del binario vs. última release (sin red: solo la propia) | no | — |

Salida: una línea por comprobación con ✓ / ⚠ / ✗ y la pista. Exit `4` si falta algo necesario; `0` con avisos. Reutiliza `process.Available`, `process.VenvBin`, `browser.discover`, `stack.Detect`.

**Tests.** Sondas falsas; cada estado produce el exit y la pista esperados; `--json`.

### 3.4 Preguntas siempre con opciones derivadas

**Problema.** El agente a veces pregunta "¿qué ficheros debo tocar?" o "¿qué comando ejecuto?" en texto libre. gentle-shell lo prohíbe: la lista candidata la deriva el agente y la persona aprueba o recorta.

**Diseño.**

- `contract.md` (en/es): "Cuando la duda sea sobre ficheros, rutas, comandos, nombres o alternativas técnicas, deriva tú la lista candidata y ponla en `options`; la persona elige, aprueba o recorta. El texto libre es solo para decisiones de producto."
- Comprobación: en las fases de loop, plan y legacy, una `needs_clarification` sin al menos dos `options` se rechaza con el retry del contrato ("derive the options"). La entrevista queda exenta (sus preguntas son de negocio). Se implementa en `conversation.Talk` con una opción `RequireOptions` por fase.
- `clarify.Asker` ya muestra opciones numeradas y acepta texto libre salvo `Strict`; sin cambios.

**Tests.** Pregunta sin opciones en GREEN → un retry → segunda sin opciones → `ErrNoContract`; en la entrevista no se exige.

### 3.5 Superficies de edición desde el plan

**Problema.** Hoy el alcance se comprueba después (snapshot). El plan ya enumera los ficheros en `## Components`; podemos declararlos antes y exigirlos.

**Diseño.**

- `spec.PlanComponents(plan string) []Surface{Path string; New bool}`: rutas entre comillas invertidas de `## Components` más los ficheros de test de `## Tests per scenario`. `LintPlan` avisa de un componente sin ruta parseable.
- Superficies permitidas = componentes + tests del plan + ficheros nuevos bajo los directorios de los componentes marcados *new* + `specs/NNNN-slug/` (decisiones).
- Prompt: `## Allowed edit surfaces` en RED/GREEN/REFACTOR con la lista exacta, en la forma de gentle-shell (una ruta por línea).
- Comprobación tras cada turno: `FilesWritten` fuera de las superficies → según `plan.surfaces` en `specforge.yaml`:
  - `ask` (defecto): pregunta a la persona "El agente cambió `X`, que no está en el plan: aceptar (se añade a las superficies de esta spec) · rechazar (vuelve a GREEN con el aviso)". Sin terminal, `questions.md` y exit 5. Las aceptaciones se guardan en el estado y en `decisions.md`; el plan sellado no se toca.
  - `strict`: rechazo con feedback, cuenta como intento.
  - `off`: como hoy.
- Sin plan: sin cambios.

**Lo que SpecForge comprueba.** Las superficies salen del plan aprobado y sellado; la escritura real sale del snapshot. Es previo (el agente lo sabe) y posterior (se comprueba).

**Tests.** Parser de componentes; loop con plan: escritura fuera → pregunta → aceptar continúa, rechazar vuelve a GREEN; `strict` rechaza; sin plan nada cambia.

### 3.6 Presupuesto de entrega

**Diseño.**

- `delivery.AuthoredLines(commit) int`: `git show --numstat` del commit del escenario, excluyendo generados (`*.lock`, `package-lock.json`, `pnpm-lock.yaml`, `go.sum`, `testdata/golden/**`, `vendor/`). Por escenario y total en DELIVERY.md y trace.json.
- `delivery.budget_lines` en `specforge.yaml` (defecto 400). Si el total lo supera, DELIVERY.md y PR_BODY.md ganan `## Suggested slices`: agrupación voraz de escenarios consecutivos ≤ presupuesto, cada corte con su rango de commits y el comando para crear la rama (`git branch slice-1 <sha>`), más el tamaño de cada uno. `deliver --slices` escribe `PR_BODY-1.md … PR_BODY-n.md`.
- Nunca se corta un escenario por la mitad: un escenario que por sí solo supera el presupuesto se señala, no se recorta (ni se "adelgaza" el código para caber, como insiste gentle-shell).

**Tests.** Algoritmo de cortes puro (tabla); exclusión de generados; `deliver` con 3 escenarios de 250 líneas → 2 cortes.

### 3.7 Modelo por fase

**Diseño.**

- `specforge.yaml`:

  ```yaml
  model: claude-sonnet-5-5        # por defecto para todo
  models:                         # opcional, por fase
    interview: claude-opus-5-5
    plan: claude-opus-5-5
    red: claude-sonnet-5-5
    green: claude-haiku-4-5
    review: claude-opus-5-5
    verify: claude-opus-5-5
  ```

  Fases: `interview, plan, legacy, red, green, refactor, review, refute, verify, audit, e2e`. Resolución: flag `--model` > `models.<fase>` > `model` > configuración de usuario.
- `config.Settings.ModelFor(phase) string`; los `Options` de `tddloop`, `planning`, `interview`, `migrate`, `review`, `verify`, `audit`, `e2erun` reciben una función en lugar de una cadena.
- `deliver` registra qué modelo hizo cada fase de cada escenario (trace.json), porque el riesgo de un modelo pequeño cuenta (3.9: un escritor "pequeño" sube el nivel de verificación, como en gentle-shell).

**Tests.** Resolución; el agente guionizado recibe `req.Model` distinto por fase.

---

## 4. Fase 6.2 · Revisión verificable y seguridad (puntos 8–10)

### 4.1 (8) Gate de revisión con lentes y hallazgos causales verificables

**Idea de gentle-shell.** Cuatro lentes de solo lectura (riesgo, fiabilidad, legibilidad, resiliencia) que devuelven hallazgos con `severity`, `evidence_class`, `causal_disposition` y `proof_refs`. Los inferenciales pasan por un refutador. Una corrección con presupuesto. Allí la autoridad está en Go externo y la verificación de las pruebas también; aquí la hacemos nosotros.

**Diseño.**

*Dominio `internal/domain/review` (puro):*

```go
type Hunk struct{ Path string; NewStart, NewEnd int; Added bool } // de git diff -U0
func ParseDiff(unified string) ([]Hunk, error)

type Severity string      // BLOCKER | CRITICAL | WARNING | SUGGESTION
type Evidence string      // deterministic | inferential | insufficient
type Causal string        // introduced | behavior-activated | worsened | pre-existing | base-only | unknown
type ProofRef struct{ Kind string; Path string; Line int } // changed-hunk | new-file | before-after
type Finding struct {
    ID, Lens string; Severity Severity; Location ProofRef; Claim string
    Evidence Evidence; Causal Causal; Proof []ProofRef
}
type Verdict struct{ Accepted, FollowUps, Discarded []Finding; Blocking []Finding; Escalate []Finding }
func Verify(findings []Finding, hunks []Hunk) Verdict
func CorrectionBudget(authoredLines int) int // min(200, ceil(lines/2))
```

Reglas de `Verify`, todas deterministas:
- Todo `proof_ref` `changed-hunk` debe caer dentro de un hunk del diff del escenario; `new-file` debe ser un fichero añadido. Un hallazgo sin ninguna prueba válida se **descarta** con razón (igual que una cita legacy inventada).
- Bloquea solo `BLOCKER|CRITICAL` con `Causal ∈ {introduced, behavior-activated, worsened}` y `Evidence ≠ insufficient`.
- `pre-existing|base-only` → seguimiento (follow-up), nunca bloquea.
- Grave con `Causal = unknown` o `Evidence = insufficient` → **escala** a la persona.
- `WARNING|SUGGESTION` → informativos.

*Aplicación `internal/app/review`:*
1. **Selección por riesgo** (3.2): pasivo ninguna lente; medio la dominante (riesgo si hay ruta sensible, fiabilidad si hay tests/API, si no legibilidad); alto las cuatro. `review.lenses: auto | off | [list]`.
2. **Lentes**: un turno de agente de solo lectura por lente, con el diff del escenario, la spec y el plan; respuesta JSON validada contra `assets/review/schema.json` con `jsonschema/v6`, como `audit`. Inválida → un retry → `StepError` (exit 2). Nunca un informe vacío por error.
3. **Refutador**: los bloqueantes `inferential` van juntos a un turno de solo lectura que devuelve `confirmed|refuted` con prueba por ID. Solo los confirmados bloquean. Los deterministas no lo necesitan.
4. **Corrección**: una sola. Turno GREEN con los hallazgos como feedback y presupuesto `CorrectionBudget(líneas del escenario)`; tras corregir, `git numstat` de la corrección: si supera el presupuesto → pregunta a la persona. Después, tests del escenario + suite − línea base + gates (la maquinaria de REFACTOR). Las lentes **no** se vuelven a ejecutar.
5. **Validación dirigida**: un turno que solo responde por los IDs corregidos: `resolved|regression`. Una regresión → **pregunta a la persona** (aceptar con seguimiento · parar). Nunca una segunda corrección automática.
6. **Registro**: `specs/NNNN-slug/review/<marker>.json` (hallazgos, descartes con razón, refutaciones, corrección, validación) y resumen en el estado; `deliver` muestra `review: 4 lenses · 2 fixed · 1 follow-up · 0 escalated` por escenario, con los seguimientos listados.
7. **Dónde corre**: fase `REVIEW` del loop entre REFACTOR y R2, por escenario (el candidato es el conjunto de ficheros del escenario, que luego es su commit). Además, `specforge review [spec] [--base <ref>]` sobre una rama entera, para código que no salió del loop; su resultado también entra en `deliver`.

*Relación con `audit`.* `audit` sigue siendo la auditoría de seguridad profunda (reconocimiento, cazador, validador) sobre una rama; la lente de riesgo es su versión ligera por escenario. Comparten el esquema de hallazgos y el motor fail-closed; unificar comandos se decide después de medir el coste real.

**Lo que SpecForge comprueba.** Esquema, pruebas contra el diff real, causalidad para bloquear, presupuesto desde git, validación solo de los IDs corregidos. Un hallazgo que no señala una línea cambiada no existe.

**Tests.** `ParseDiff` con fixtures (ficheros nuevos, renombrados, binarios); `Verify` con 15 casos (prueba fuera de hunk descartada, pre-existing a seguimiento, unknown escala); esquema inválido → retry → `StepError`; loop completo con agente falso: hallazgo → corrección → validación; presupuesto superado → pregunta; `review` sobre rama en `cmd`.

**Hecho cuando** la demo real: un escenario de riesgo alto pasa por las cuatro lentes, una lente inventa una prueba fuera del diff y SpecForge la descarta, un hallazgo real se corrige en una ronda y la validación lo cierra. Todo en `docs/DEMO.md`.

### 4.2 (9) Verificador independiente desde la spec

**Idea de gentle-shell** (`gentle-ai-verify`): "verifica la petición, no el trabajo del escritor; el test del escritor puede fijar un bug". Deriva sus propias pruebas de la spec, las ejecuta en una copia aislada y devuelve un veredicto por requisito.

**Diseño.**

- Puerto `ports.Scratch`: `Make(root, baseRef) (dir, cleanup)`. Adaptador `adapters/scratch`: copia los ficheros versionados a `mktemp -d` (`cp -a --reflink=auto` cuando existe), enlaza en solo lectura los directorios de dependencias (`node_modules`, `.venv`, `target`, `vendor`) para no copiarlos, y crea una segunda copia en el commit base (`git worktree` temporal, luego copiado y retirado) para comparar comportamiento previo. Límite de tamaño configurable; por encima, el verificador se salta con ⚠ y razón.
- `internal/app/verify`: un turno `docturn` con `Dir = scratch`, sin `ReadDirs`. Prompt `verify.md`: derivar pruebas de cada escenario e invariante (positivas, negativas, límites, mensajes y códigos de salida exactos), hashear los datos antes y después de una operación rechazada, ejecutar los ejemplos de la spec, comparar con la copia base los comandos que ya existían. Respuesta JSON (`assets/review/verify-schema.json`): por cada `INV-NN` y escenario `met|unmet|unverified{reason, command}`, bloqueantes con `command` y `observed` (salida real), avisos, y tests de regresión propuestos (`path`, `content`, `covers`).
- Comprobaciones de SpecForge: esquema; **todos** los INV y escenarios de la spec tienen veredicto (los IDs se cruzan con la spec parseada); cada bloqueante trae comando y salida no vacía; el proyecto real está intacto (snapshot antes/después, como el legacy); la copia se borra.
- Flujo: tras REVIEW en escenarios de riesgo alto (`verify: high`, defecto; `always`, `off`), y al final del loop sobre toda la spec si `verify: feature`. También `specforge verify <spec>` bajo demanda. Bloqueantes → **una** corrección (turno GREEN con los bloqueantes) → re-verificación solo de esos IDs → si siguen, pregunta a la persona. Los tests de regresión propuestos se ofrecen: aceptados → se añaden en un paso `VERIFY` autorizado (sus hashes entran en `TestHashes`) y se commitean con el escenario.
- Los agentes corren con escritura en la copia (no necesitamos modo solo lectura del CLI): lo que escriban ahí no importa, y el proyecto real se comprueba.

**Lo que SpecForge comprueba.** Cobertura de veredictos contra la spec, salida observada, proyecto intacto, una sola corrección.

**Tests.** Agente falso: veredicto incompleto rechazado; bloqueante sin salida rechazado; proyecto modificado → error; flujo corrección → re-verificación → pregunta; copia borrada; límite de tamaño.

**Hecho cuando** la demo planta un bug que el test del escritor fija (p. ej. redondeo) y el verificador lo detecta con un comando reproducible.

### 4.3 (10) Guardia de comandos destructivos

**Idea de gentle-shell.** Un tokenizador (no un intérprete) reconoce `rm -r`, `find -delete`, `DROP/TRUNCATE/DELETE sin WHERE`, `git reset --hard`, `clean -f`, `push --force`, `branch -D`, `stash drop`, desenvolviendo `sudo`, `env`, `xargs`, `timeout`, `sh -c`. Hard-deny para `rm -r /`, `~`, `.`. Los subagentes no pueden confirmar: se bloquea.

**Diseño.**

- `internal/domain/guard` (puro): puerto a Go del reconocedor, con la misma tabla de casos. `Recognize(cmd string) []Match{Kind fs|git|db|sensitive; HardDeny bool; Reason; Segment}`. Añade rutas sensibles como argumento (`.env`, `.ssh/`, `*.pem`, `id_rsa`, `credentials`).
- `specforge guard --hook claude|gemini`: lee por stdin el JSON del hook (`tool_name`, `tool_input.command`), escribe la razón por stderr y sale `2` para bloquear (semántica de Claude Code: exit 2 bloquea y el modelo ve el motivo) o `0`. Modo por configuración:

  ```yaml
  guard:
    mode: block          # block | confirm (solo con terminal) | off
    allow: ["git push --force-with-lease origin claude/*"]
  ```

  En el loop (sin terminal para el agente) todo lo reconocido se bloquea, como los hijos de gentle-shell.
- `setup` instala el hook en `.claude/settings.json` del proyecto (`hooks.PreToolUse`, matcher `Bash`), fusionando sin tocar el resto del fichero e idempotente; en Gemini CLI, si la versión instalada expone hooks equivalentes; si no, `doctor` lo marca ⚠ y la guía lo dice. Nada de esto depende de que el hook exista: la verificación del loop sigue igual sin él.
- Fuera del loop (uso interactivo del agente por la persona), `confirm` muestra la razón y pregunta.

**Límites, escritos en la guía como los escribe gentle-shell:** reconocimiento léxico, no sandbox; scripts, programas y expansiones no se inspeccionan; solo cubre la herramienta Bash del agente. Para contención real: contenedor.

**Tests.** Tabla de 60+ comandos (positivos, negativos, envueltos, con comillas, heredocs, SQL con comentarios); hook con stdin y códigos de salida; `setup` fusiona y es idempotente; `doctor` lo detecta.

---

## 5. Fase 6.3 · Paralelismo y revisión ciega (puntos 11–12)

### 5.1 (11) Escenarios en paralelo

Requisito previo: superficies por escenario (3.5) y riesgo (3.2). Diseño a validar antes de implementar:

- El plan declara los componentes por escenario; `spec.PlanComponents` devuelve superficies **por marcador**. Dos escenarios son paralelizables si sus superficies son disjuntas (comprobación pura, glob-aware, como `writer-surfaces.ts`).
- Cada escenario paralelo corre en un `git worktree` propio sobre la misma rama base; RED/GREEN/REFACTOR como hoy; al terminar, se integran en orden de escenario (`cherry-pick`), y un REFACTOR final corre la suite completa una vez ("seam check" de gentle-shell).
- Límite `loop.parallel: 1` por defecto; la revisión humana R2 sigue siendo secuencial.
- Riesgos: suites que escriben en rutas compartidas, dependencias instaladas por worktree (coste), conflictos en ficheros de registro (`decisions.md`). Se empieza con Go y Python; Node y Java después.

### 5.2 (12) Revisión ciega doble

Variante de 4.1 para riesgo alto (`review.blind: true`): dos pases independientes de las mismas lentes (modelos distintos si `models.review2` existe, si no el mismo con semillas distintas). Los hallazgos que ambos pases prueban sobre el mismo hunk son `deterministic` sin refutador; los que solo encuentra uno pasan por el refutador. Coste ×2, solo en alto. Reusa todo 4.1; ~3 días.

---

## 6. Cambios transversales

- **`specforge.yaml`** gana `review`, `models`, `plan.surfaces`, `delivery.budget_lines`, `verify`, `guard`, `risk`. Claves desconocidas siguen siendo error. La plantilla de `setup` se actualiza con todo comentado.
- **Estado del loop**: `Version` sube; el estado v5 se lee y se migra (línea base vacía, riesgo sin evaluar).
- **Prompts** en/es: `review_risk|reliability|readability|resilience.md`, `refute.md`, `correct.md`, `validate.md`, `verify.md`; `contract.md` con la regla de opciones y el campo `risk`; `context.md` con fallos conocidos y superficies.
- **Guía de usuario**: §7 gana la fase REVIEW y las superficies; §9 "gates por riesgo"; secciones nuevas `doctor`, `review`, `verify`, `guard`; §14 ficheros nuevos (`review/`, línea base en el estado); §15 recetas (aceptar una escritura fuera del plan, forzar verificación, desactivar la guardia para un comando).
- **README**: "Nothing moves forward on the agent's word" se mantiene; se añade una fila por gate nuevo en la tabla de verificación con lo que comprueba cada uno.
- **DEMO**: una sesión real nueva (§8) con un escenario pasivo, uno medio y uno alto con lentes, refutador, corrección, verificador y guardia bloqueando un `git reset --hard`.
- **CI**: sin cambios de forma; suelo de cobertura sube a 75 %.
- **Release**: `v6.0.0` con el workflow de la v5; las notas en `docs/releases/v6.0.0.md`.

---

## 7. Orden de ejecución, PRs y calendario

PRs apiladas, cada una con CI verde y tests reales donde aplique:

| PR | Contenido | Fase | Días |
| :---: | :--- | :---: | :---: |
| A | Nombres de tests fallidos en los runners; línea base; REFACTOR con resta; prompt y deliver | 3.1 | 2 |
| B | Dominio `risk`; `review: risk`; escalado en el contrato; modelo por fase | 3.2, 3.7 | 3 |
| C | `doctor`; opciones obligatorias | 3.3, 3.4 | 1,5 |
| D | Superficies desde el plan; presupuesto de entrega y cortes | 3.5, 3.6 | 3 |
| — | **Demo real 6.1 y release 5.1.0** | | 1 |
| E | Dominio `review` (diff, hallazgos, verify); lentes; refutador; corrección; validación; `specforge review` | 4.1 | 8 |
| F | Scratch; verificador; regresiones propuestas | 4.2 | 5 |
| G | Dominio `guard`; hook; `setup`; `doctor` | 4.3 | 3 |
| H | Docs, demo real 6.2, `v6.0.0` | 6 | 2 |

Calendario: 6.1 en dos semanas (PR A–D, release **5.1.0** intermedia para que la proporcionalidad llegue pronto); 6.2 en tres o cuatro (PR E–H, **6.0.0**); 6.3 se planifica con lo aprendido midiendo 6.2 en uso real.

Orden dentro de cada PR: dominio con tests → aplicación con agente falso → `cmd` → prompts en/es → guía → prueba real con Claude Code → `golangci-lint` 0 avisos.

---

## 8. Riesgos y mitigaciones

| Riesgo | Mitigación |
| :--- | :--- |
| Coste en tokens: cuatro lentes + refutador + verificador por escenario | Solo en riesgo alto por defecto; pasivo no paga nada; `review.lenses: off` y `verify: off` existen; `deliver` muestra tokens y modelo por fase para que el coste se vea |
| Clasificación de riesgo equivocada (falso pasivo) | Reglas configurables; el agente puede subir; `--strict` lo fuerza todo a alto; nunca se baja |
| Copias de trabajo pesadas para el verificador | Enlaces a dependencias, límite de tamaño con ⚠, se salta con razón, nunca se "simula" |
| Nombres de tests distintos entre runners (clases Java, `fullName` de Vitest) | Un `TestRef` normalizado por runner con fixtures reales; con `Exact=false` no hay línea base |
| Hooks que cambian entre versiones de Claude Code y Gemini CLI | `doctor` comprueba que el hook responde (`specforge guard --hook claude --selftest`); la verificación del loop no depende del hook |
| El plan enumera mal los componentes | `LintPlan` avisa; `plan.surfaces: ask` pregunta en vez de rechazar |
| Más fases = loops más largos | Cada fase se ve en la salida con su tiempo; `--resume` cubre todas; estado atómico |
| Complejidad del código (el fichero de 10k líneas de gentle-shell es el aviso) | Cada dominio nuevo es un paquete puro con tests de tabla; `gocyclo ≤ 25` ya está en el lint |

---

## 9. Lo que no se hace

- Telemetría de ningún tipo.
- Interfaz de terminal (barra lateral, temas, Vim, paleta).
- Dependencias de un binario externo para la revisión: todo en el mismo binario Go.
- Vocabulario de "autoridad", "recibos" y "linajes".
- Skills ligadas a la organización (etiquetas `type:*`, issue obligatoria).
- Memoria persistente externa: `LESSONS.md` y `decisions.md` siguen siendo la memoria, versionada en el repo.
- Prometer en el README lo que solo esté en un prompt.

---

## 10. Métricas de éxito

Medidas en la demo real y en la CI:

1. Un repositorio con tests rotos completa un loop; los fallos conocidos aparecen en DELIVERY.md.
2. Un escenario pasivo cuesta **0** llamadas de revisión y verificación.
3. El **100 %** de los hallazgos aceptados señalan un hunk real; los demás aparecen como descartados con su razón.
4. El verificador detecta el bug plantado de la demo con un comando reproducible.
5. `git reset --hard` dentro del loop se bloquea y el agente ve el motivo.
6. `doctor` en una máquina limpia dice exactamente qué instalar, y en una completa sale `0`.
7. Cobertura ≥ 75 %, `golangci-lint` 0 avisos, CI verde en tres SO.
8. README y guía siguen leyéndose en móvil sin desplazamiento lateral (misma comprobación que en v5).

---

## Anexo A · De dónde sale cada idea

| Punto | Fichero de gentle-shell | Qué cambia al traerlo |
| :---: | :--- | :--- |
| 1 | `assets/agents/gentle-ai-worker.md` (`## Known environmental failures`) | Allí lo declara el padre en el prompt; aquí lo mide SpecForge con el runner |
| 2 | `lib/review-risk.ts`, `lib/review-risk-assessment.ts` | Mismas heurísticas; aquí decide gates, lentes, verificación y R2 |
| 3 | `/gentle:doctor` en `extensions/gentle-ai.ts` | Igual idea, con códigos de salida y pistas de instalación |
| 4 | `gentle-ai-worker.md` (`interaction_required`, "never ask the human to author paths") | Allí es prompt; aquí se rechaza la pregunta sin opciones |
| 5 | `lib/bounded-writer-admission.ts`, `assets/orchestrator-writer.md` | Allí solo admisión; aquí admisión + comprobación posterior desde el plan sellado |
| 6 | `skills/chained-pr`, `work-unit-commits`, presupuesto de 400 líneas | Allí lo decide el modelo; aquí lo cuenta git y lo propone `deliver` |
| 7 | Perfiles por rol (`lib/agent-profiles.ts`) | Simplificado a un mapa por fase |
| 8 | `assets/agents/review-*.md`, `skills/_shared/review-ledger-contract.md`, `lib/review-risk.ts` | La autoridad estaba en Go externo; aquí el dominio `review` verifica las pruebas contra el diff |
| 9 | `assets/agents/gentle-ai-verify.md` | Mismas reglas de sondeo; aquí se exige cobertura de veredictos y proyecto intacto |
| 10 | `lib/destructive-command-guard.ts`, `docs/yolo-mode.md` | Portado a Go como dominio puro; instalado como hook del agente |
| 11 | `lib/writer-surfaces.ts`, `lib/agents-runner.ts` | Procesos y worktrees por escenario, cuando 5 exista |
| 12 | `skills/judgment-day`, `assets/agents/jd-judge-*.md` | Dos pases de las mismas lentes; intersección = determinista |

## Anexo B · Contratos JSON nuevos

**Hallazgo de una lente** (`assets/review/schema.json`, extracto):

```json
{
  "lens": "reliability",
  "findings": [{
    "id": "REL-001",
    "severity": "CRITICAL",
    "location": {"path": "internal/pay/net.go", "line": 42},
    "claim": "A negative bonus lowers the net below zero; INV-03 says net ≥ 0.",
    "evidence_class": "deterministic",
    "causal_disposition": "introduced",
    "proof_refs": [{"kind": "changed-hunk", "path": "internal/pay/net.go", "line": 42}]
  }],
  "evidence": ["Read the diff of 3 files; ran no commands."]
}
```

**Veredicto del verificador** (`assets/review/verify-schema.json`, extracto):

```json
{
  "verdicts": [
    {
      "id": "INV-03",
      "status": "unmet",
      "command": "go run . pay --gross -5",
      "observed": "net: -5.00"
    },
    {
      "id": "SDD_0001_002",
      "status": "met",
      "command": "go test -run SDD_0001_002 ./...",
      "observed": "ok"
    }
  ],
  "blockers": [
    {
      "id": "INV-03",
      "command": "go run . pay --gross -5",
      "observed": "net: -5.00",
      "expected": "error NEGATIVE_GROSS"
    }
  ],
  "advisories": [],
  "regression_tests": [
    {"path": "internal/pay/net_test.go", "covers": ["INV-03"], "content": "…"}
  ]
}
```

**Escalado de riesgo en el contrato del agente** (añadido a `contract.md`):

```json
{
  "status": "done",
  "files_written": ["…"],
  "summary": "…",
  "risk": "high",
  "risk_reason": "touches the password reset token"
}
```
