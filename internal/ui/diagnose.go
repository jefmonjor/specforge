package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"specforge/internal/app/audit"
	"specforge/internal/app/clarify"
	"specforge/internal/app/doctor"
	"specforge/internal/app/e2erun"
	"specforge/internal/app/interview"
	"specforge/internal/app/migrate"
	"specforge/internal/app/planning"
	"specforge/internal/app/protocol"
	"specforge/internal/app/reviewer"
	"specforge/internal/app/specs"
	"specforge/internal/app/tddloop"
	"specforge/internal/config"
	"specforge/internal/domain/spec"
	"specforge/internal/domain/tdd"
	"specforge/internal/ports"
)

// Exit codes. They are part of the CLI contract: scripts and CI rely on
// them, so they only ever gain new values.
const (
	ExitOK          = 0
	ExitError       = 1   // unexpected error or bad usage
	ExitGate        = 2   // a gate said no: tests, quality, security, E2E
	ExitSpec        = 3   // the specification or the loop state needs attention
	ExitEnvironment = 4   // a tool, browser or setting is missing
	ExitQuestion    = 5   // a question awaits the developer (non-interactive run)
	ExitInterrupted = 130 // Ctrl-C
)

// Diagnosis explains an error to the developer.
type Diagnosis struct {
	Code   int    `json:"exit"`
	Title  string `json:"title"`
	Cause  string `json:"cause"`
	Action string `json:"action,omitempty"`
}

// Diagnose classifies err.
func Diagnose(lang string, err error) Diagnosis {
	d := Diagnosis{Code: ExitError, Cause: err.Error()}
	key := ""
	if errors.Is(err, context.Canceled) {
		d.Code, key = ExitInterrupted, "interrupted"
	} else {
		for _, classify := range []classifier{questionErrors, specErrors, gateErrors, environmentErrors} {
			if code, k := classify(lang, err, &d); k != "" {
				d.Code, key = code, k
				break
			}
		}
	}
	if key != "" {
		d.Title = T(lang, "diag."+key+".title")
		if d.Action == "" {
			d.Action = T(lang, "diag."+key+".action")
		}
	}
	return d
}

// A classifier recognises a family of errors. It returns the exit code and
// the message key, and may refine the cause and the action; key is "" when
// err is not in its family.
type classifier func(lang string, err error, d *Diagnosis) (code int, key string)

func questionErrors(lang string, err error, d *Diagnosis) (int, string) {
	var pending *clarify.PendingQuestionError
	if errors.As(err, &pending) {
		d.Cause = pending.Question
		d.Action = T(lang, "diag.pending.action", pending.File)
		return ExitQuestion, "pending"
	}
	return 0, ""
}

func specErrors(_ string, err error, d *Diagnosis) (int, string) {
	var (
		tamper   *tdd.TamperingError
		tampered *spec.TamperedError
		openQ    *tdd.OpenQuestionsError
		lint     *specs.LintError
		ambig    *specs.AmbiguousError
		notFound *specs.NotFoundError
	)
	switch {
	case errors.As(err, &tamper):
		return ExitSpec, "tampering"
	case errors.As(err, &tampered):
		return ExitSpec, "tampered"
	case errors.Is(err, spec.ErrNotSealed):
		return ExitSpec, "notsealed"
	case errors.As(err, &openQ):
		d.Cause = strings.Join(openQ.Questions, "\n")
		return ExitSpec, "openquestions"
	case errors.As(err, &lint):
		lines := []string{lint.Path}
		for _, i := range lint.Issues {
			lines = append(lines, i.String())
		}
		d.Cause = strings.Join(lines, "\n")
		return ExitSpec, "lint"
	case errors.As(err, &ambig):
		return ExitSpec, "ambiguous"
	case errors.As(err, &notFound), errors.Is(err, specs.ErrNoSpecs):
		return ExitSpec, "nospec"
	case errors.Is(err, spec.ErrNoScenarios):
		return ExitSpec, "noscenarios"
	case errors.Is(err, tddloop.ErrPlanNotApproved), errors.Is(err, tddloop.ErrPlanOutdated),
		errors.Is(err, specs.ErrNoPlan), errors.Is(err, specs.ErrPlanNeedsApprovedSpec):
		return ExitSpec, "plan"
	case errors.Is(err, tddloop.ErrLoopInProgress), errors.Is(err, tddloop.ErrNothingToResume), errors.Is(err, tdd.ErrStateMismatch):
		return ExitSpec, "state"
	}
	return 0, ""
}

func gateErrors(lang string, err error, d *Diagnosis) (int, string) {
	if code, key := reviewErrors(lang, err, d); key != "" {
		return code, key
	}
	var (
		gates    *tddloop.GatesError
		blocked  *tdd.AgentBlockedError
		scope    *planning.ScopeError
		partial  *planning.IncompleteError
		secBlock *audit.BlockedError
		stepErr  *audit.StepError
		e2eBelow *e2erun.BelowThresholdError
		iScope   *interview.ScopeError
		iPartial *interview.IncompleteError
	)
	switch {
	case errors.As(err, &iScope):
		return ExitGate, "interview"
	case errors.As(err, &iPartial):
		lines := []string{err.Error()}
		for _, i := range iPartial.Issues {
			lines = append(lines, i.String())
		}
		d.Cause = strings.Join(lines, "\n")
		return ExitGate, "interview"
	case errors.Is(err, tdd.ErrPrematureGreen):
		return ExitGate, "premature"
	case errors.Is(err, tdd.ErrAttemptsExhausted):
		return ExitGate, "attempts"
	case errors.As(err, &gates):
		d.Cause = gates.Report.Explain(gates.Strict)
		if gates.SuiteFailure != "" {
			d.Cause = err.Error() + "\n" + gates.SuiteFailure
		}
		return ExitGate, "gates"
	case errors.As(err, &blocked):
		d.Cause, d.Action = blocked.Reason, blocked.SuggestedAction
		return ExitGate, "blocked"
	case errors.Is(err, protocol.ErrNoContract), errors.Is(err, tddloop.ErrTooManyQuestions):
		return ExitGate, "contract"
	case errors.As(err, &scope):
		return ExitGate, draftKey(scope.Step)
	case errors.As(err, &partial):
		d.Cause = strings.Join(append([]string{err.Error()}, partial.Problems...), "\n")
		return ExitGate, draftKey(partial.Step)
	case errors.As(err, &secBlock):
		var lines []string
		for _, f := range secBlock.Findings {
			lines = append(lines, fmt.Sprintf("[%s] %s (%s, %s) %s:%d", f.ID, f.Title, f.Severity, f.Status, f.File, f.Line))
		}
		d.Cause = strings.Join(lines, "\n")
		return ExitGate, "security"
	case errors.As(err, &stepErr):
		return ExitGate, "auditstep"
	case errors.As(err, &e2eBelow):
		return ExitGate, "e2e"
	}
	return 0, ""
}

// reviewErrors classify what the review lenses and the plan's edit
// surfaces stop: each is a gate saying no.
func reviewErrors(_ string, err error, _ *Diagnosis) (int, string) {
	var (
		blocked  *reviewer.BlockedError
		step     *reviewer.StepError
		readOnly *reviewer.ReadOnlyError
		surfaces *tddloop.SurfaceError
	)
	switch {
	case errors.As(err, &blocked), errors.Is(err, tddloop.ErrReviewStopped):
		return ExitGate, "review"
	case errors.As(err, &step), errors.As(err, &readOnly):
		return ExitGate, "reviewstep"
	case errors.As(err, &surfaces):
		return ExitGate, "surfaces"
	}
	return 0, ""
}

// draftKey names the diagnosis of a document turn by its step.
func draftKey(step string) string {
	switch {
	case step == "plan":
		return "plandraft"
	case strings.Contains(step, "legacy code is read-only"):
		return "legacyreadonly"
	}
	return "legacydraft"
}

func environmentErrors(_ string, err error, _ *Diagnosis) (int, string) {
	var missing *doctor.MissingError
	switch {
	case errors.As(err, &missing):
		return ExitEnvironment, "doctor"
	case errors.Is(err, migrate.ErrNoLegacy):
		return ExitEnvironment, "nolegacy"
	case errors.Is(err, ports.ErrToolNotFound), errors.Is(err, tdd.ErrUnsupportedStack):
		return ExitEnvironment, "tool"
	case errors.Is(err, config.ErrNoAgent):
		return ExitEnvironment, "noagent"
	case errors.Is(err, ports.ErrTimeout):
		return ExitEnvironment, "timeout"
	}
	return 0, ""
}

// PrintDiagnosis writes the box for err to w.
func PrintDiagnosis(w io.Writer, lang string, d Diagnosis) {
	title := d.Title
	if title == "" {
		title = T(lang, "diag.generic.title")
	}
	fmt.Fprintf(w, "\n✗ %s\n", title)
	for _, line := range strings.Split(strings.TrimSpace(d.Cause), "\n") {
		fmt.Fprintf(w, "    %s\n", line)
	}
	if d.Action != "" {
		fmt.Fprintf(w, "  → %s\n", d.Action)
	}
	fmt.Fprintf(w, "  (exit %d)\n", d.Code)
}

func init() {
	for lang, cat := range map[string]map[string]string{
		"en": {
			"diag.generic.title":         "SpecForge stopped",
			"diag.interrupted.title":     "Interrupted",
			"diag.interrupted.action":    "the state was saved: continue with `specforge loop --resume`",
			"diag.pending.title":         "A question needs your answer",
			"diag.pending.action":        "write the answer in place of the placeholder in %s and run the same command again (the loop with --resume), or run it in a terminal and answer there",
			"diag.tampering.title":       "A test file was modified outside RED",
			"diag.tampering.action":      "revert the test change (git checkout -- <file>) and continue with `specforge loop --resume`",
			"diag.tampered.title":        "The specification changed after approval",
			"diag.tampered.action":       "revert the edit or approve the new version with `specforge spec approve`",
			"diag.notsealed.title":       "The specification is not approved",
			"diag.notsealed.action":      "review it and run `specforge spec approve <spec>`",
			"diag.openquestions.title":   "The specification has open questions",
			"diag.openquestions.action":  "resolve every [NEEDS CLARIFICATION] and approve the specification again",
			"diag.interview.title":       "The interview did not finish",
			"diag.interview.action":      "check the specification, then run `specforge spec interview <spec>` again (it continues where it stopped)",
			"diag.plandraft.title":       "The plan draft was not accepted",
			"diag.plandraft.action":      "check the files listed above, then run `specforge plan <spec>` again",
			"diag.legacydraft.title":     "The legacy document was not accepted",
			"diag.legacydraft.action":    "the problems are above; run the same command again (the agent revises its draft)",
			"diag.legacyreadonly.title":  "The agent changed the legacy code",
			"diag.legacyreadonly.action": "the legacy repository is read-only: restore it (git checkout in that repository) and run the same command again",
			"diag.nolegacy.title":        "Legacy repository not found",
			"diag.nolegacy.action":       "pass its path, or set `migration.legacy` in specforge.yaml",
			"diag.plan.title":            "The plan needs attention",
			"diag.plan.action":           "draft it with `specforge plan <spec>`, review it, then `specforge plan approve <spec>`",
			"diag.lint.title":            "The specification is not ready for approval",
			"diag.lint.action":           "fix each line above (`specforge spec lint` shows the advice too) and approve again",
			"diag.ambiguous.title":       "Which specification?",
			"diag.ambiguous.action":      "name it: `specforge <command> 0001` (number, file name or path)",
			"diag.nospec.title":          "Specification not found",
			"diag.nospec.action":         "list them with `specforge spec list` or create one with `specforge spec new \"<title>\"`",
			"diag.noscenarios.title":     "The specification has no Gherkin scenarios",
			"diag.noscenarios.action":    "add the acceptance criteria as Gherkin scenarios in a ```gherkin block",
			"diag.state.title":           "Loop state",
			"diag.state.action":          "use --resume to continue or --restart to start over",
			"diag.premature.title":       "The test passed before any implementation",
			"diag.premature.action":      "decide whether the behaviour already exists and run `specforge loop --resume` in a terminal",
			"diag.attempts.title":        "The agent ran out of attempts",
			"diag.attempts.action":       "look at the last failure, fix or guide it, then `specforge loop --resume`",
			"diag.gates.title":           "Quality gates block the scenario",
			"diag.gates.action":          "fix the findings (or install the missing tools) and `specforge loop --resume`",
			"diag.blocked.title":         "The agent is blocked",
			"diag.blocked.action":        "fix the cause and `specforge loop --resume`",
			"diag.contract.title":        "The agent did not follow the response contract",
			"diag.contract.action":       "check the agent output with --trace-io and try again",
			"diag.security.title":        "Security findings block the merge",
			"diag.security.action":       "fix them or settle the doubtful ones; details in docs/security/REPORT.md",
			"diag.auditstep.title":       "The audit could not complete",
			"diag.auditstep.action":      "the audit fails closed: rerun it, or inspect the output with --trace-io",
			"diag.e2e.title":             "E2E scenarios failed",
			"diag.e2e.action":            "see docs/e2e/<spec>/REPORT.md and the screenshots",
			"diag.tool.title":            "A required tool is missing",
			"diag.tool.action":           "install it or configure the stack in specforge.yaml",
			"diag.noagent.title":         "No coding agent configured",
			"diag.noagent.action":        "run `specforge init`",
			"diag.timeout.title":         "A step timed out",
			"diag.timeout.action":        "raise timeouts.agent or timeouts.tests in specforge.yaml",
		},
		"es": {
			"diag.generic.title":         "SpecForge se detuvo",
			"diag.interrupted.title":     "Interrumpido",
			"diag.interrupted.action":    "el estado quedó guardado: continúa con `specforge loop --resume`",
			"diag.pending.title":         "Una pregunta espera tu respuesta",
			"diag.pending.action":        "escribe la respuesta en lugar del marcador en %s y vuelve a lanzar el mismo comando (el ciclo con --resume), o lánzalo en una terminal y responde allí",
			"diag.tampering.title":       "Se modificó un fichero de test fuera de RED",
			"diag.tampering.action":      "revierte el cambio del test (git checkout -- <fichero>) y continúa con `specforge loop --resume`",
			"diag.tampered.title":        "La especificación cambió después de aprobarse",
			"diag.tampered.action":       "revierte la edición o aprueba la nueva versión con `specforge spec approve`",
			"diag.notsealed.title":       "La especificación no está aprobada",
			"diag.notsealed.action":      "revísala y ejecuta `specforge spec approve <spec>`",
			"diag.openquestions.title":   "La especificación tiene preguntas abiertas",
			"diag.openquestions.action":  "resuelve cada [NEEDS CLARIFICATION] y vuelve a aprobar la especificación",
			"diag.interview.title":       "La entrevista no terminó",
			"diag.interview.action":      "revisa la especificación y vuelve a lanzar `specforge spec interview <spec>` (continúa donde se quedó)",
			"diag.legacydraft.title":     "El documento legacy no se aceptó",
			"diag.legacydraft.action":    "los problemas están arriba; vuelve a lanzar el mismo comando (el agente revisa su borrador)",
			"diag.legacyreadonly.title":  "El agente cambió el código legacy",
			"diag.legacyreadonly.action": "el repositorio legacy es de solo lectura: restáuralo (git checkout en ese repositorio) y vuelve a lanzar el mismo comando",
			"diag.nolegacy.title":        "No se encuentra el repositorio legacy",
			"diag.nolegacy.action":       "pasa su ruta o configura `migration.legacy` en specforge.yaml",
			"diag.plandraft.title":       "El borrador del plan no se aceptó",
			"diag.plandraft.action":      "revisa los ficheros de arriba y vuelve a lanzar `specforge plan <spec>`",
			"diag.plan.title":            "El plan necesita atención",
			"diag.plan.action":           "redáctalo con `specforge plan <spec>`, revísalo y después `specforge plan approve <spec>`",
			"diag.lint.title":            "La especificación no está lista para aprobarse",
			"diag.lint.action":           "corrige cada línea de arriba (`specforge spec lint` muestra también los consejos) y vuelve a aprobar",
			"diag.ambiguous.title":       "¿Qué especificación?",
			"diag.ambiguous.action":      "indícala: `specforge <comando> 0001` (número, nombre de fichero o ruta)",
			"diag.nospec.title":          "Especificación no encontrada",
			"diag.nospec.action":         "lístalas con `specforge spec list` o crea una con `specforge spec new \"<título>\"`",
			"diag.noscenarios.title":     "La especificación no tiene escenarios Gherkin",
			"diag.noscenarios.action":    "añade los criterios de aceptación como escenarios en un bloque ```gherkin",
			"diag.state.title":           "Estado del ciclo",
			"diag.state.action":          "usa --resume para continuar o --restart para empezar de cero",
			"diag.premature.title":       "El test pasó antes de existir implementación",
			"diag.premature.action":      "decide si el comportamiento ya existe y ejecuta `specforge loop --resume` en una terminal",
			"diag.attempts.title":        "El agente agotó sus intentos",
			"diag.attempts.action":       "revisa el último fallo, corrígelo o guíalo y ejecuta `specforge loop --resume`",
			"diag.gates.title":           "Las puertas de calidad bloquean el escenario",
			"diag.gates.action":          "corrige los hallazgos (o instala las herramientas que faltan) y `specforge loop --resume`",
			"diag.blocked.title":         "El agente está bloqueado",
			"diag.blocked.action":        "corrige la causa y `specforge loop --resume`",
			"diag.contract.title":        "El agente no siguió el contrato de respuesta",
			"diag.contract.action":       "revisa su salida con --trace-io y vuelve a intentarlo",
			"diag.security.title":        "Hallazgos de seguridad bloquean el merge",
			"diag.security.action":       "corrígelos o decide los dudosos; detalle en docs/security/REPORT.md",
			"diag.auditstep.title":       "La auditoría no pudo completarse",
			"diag.auditstep.action":      "la auditoría falla cerrada: repítela o revisa la salida con --trace-io",
			"diag.e2e.title":             "Escenarios E2E fallidos",
			"diag.e2e.action":            "revisa docs/e2e/<spec>/REPORT.md y las capturas",
			"diag.tool.title":            "Falta una herramienta necesaria",
			"diag.tool.action":           "instálala o configura el stack en specforge.yaml",
			"diag.noagent.title":         "No hay agente configurado",
			"diag.noagent.action":        "ejecuta `specforge init`",
			"diag.timeout.title":         "Un paso superó su tiempo máximo",
			"diag.timeout.action":        "aumenta timeouts.agent o timeouts.tests en specforge.yaml",
		},
	} {
		for k, v := range cat {
			messages[lang][k] = v
		}
	}
}
