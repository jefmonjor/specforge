package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"specforge/internal/app/audit"
	"specforge/internal/app/clarify"
	"specforge/internal/app/e2erun"
	"specforge/internal/app/protocol"
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
	Code   int
	Title  string
	Cause  string
	Action string
}

// Diagnose classifies err.
func Diagnose(lang string, err error) Diagnosis {
	d := Diagnosis{Code: ExitError, Cause: err.Error()}
	set := func(code int, key string) {
		d.Code = code
		d.Title = T(lang, "diag."+key+".title")
		d.Action = T(lang, "diag."+key+".action")
	}

	var (
		tamper   *tdd.TamperingError
		openQ    *tdd.OpenQuestionsError
		blocked  *tdd.AgentBlockedError
		tampered *spec.TamperedError
		pending  *clarify.PendingQuestionError
		gates    *tddloop.GatesError
		secBlock *audit.BlockedError
		stepErr  *audit.StepError
		e2eBelow *e2erun.BelowThresholdError
	)
	switch {
	case errors.Is(err, context.Canceled):
		set(ExitInterrupted, "interrupted")
	case errors.As(err, &pending):
		set(ExitQuestion, "pending")
		d.Action = T(lang, "diag.pending.action", pending.File)
	case errors.As(err, &tamper):
		set(ExitSpec, "tampering")
	case errors.As(err, &tampered):
		set(ExitSpec, "tampered")
	case errors.Is(err, spec.ErrNotSealed):
		set(ExitSpec, "notsealed")
	case errors.As(err, &openQ):
		set(ExitSpec, "openquestions")
		d.Cause = strings.Join(openQ.Questions, "\n")
	case errors.Is(err, spec.ErrNoScenarios):
		set(ExitSpec, "noscenarios")
	case errors.Is(err, tddloop.ErrLoopInProgress), errors.Is(err, tddloop.ErrNothingToResume), errors.Is(err, tdd.ErrStateMismatch):
		set(ExitSpec, "state")
	case errors.Is(err, tdd.ErrPrematureGreen):
		set(ExitGate, "premature")
	case errors.Is(err, tdd.ErrAttemptsExhausted):
		set(ExitGate, "attempts")
	case errors.As(err, &gates):
		set(ExitGate, "gates")
		if gates.SuiteFailure != "" {
			d.Cause = err.Error() + "\n" + gates.SuiteFailure
		} else {
			d.Cause = gates.Report.Explain(gates.Strict)
		}
	case errors.As(err, &blocked):
		set(ExitGate, "blocked")
		d.Cause = blocked.Reason
		if blocked.SuggestedAction != "" {
			d.Action = blocked.SuggestedAction
		}
	case errors.Is(err, protocol.ErrNoContract), errors.Is(err, tddloop.ErrTooManyQuestions):
		set(ExitGate, "contract")
	case errors.As(err, &secBlock):
		set(ExitGate, "security")
		var lines []string
		for _, f := range secBlock.Findings {
			lines = append(lines, fmt.Sprintf("[%s] %s (%s, %s) %s:%d", f.ID, f.Title, f.Severity, f.Status, f.File, f.Line))
		}
		d.Cause = strings.Join(lines, "\n")
	case errors.As(err, &stepErr):
		set(ExitGate, "auditstep")
	case errors.As(err, &e2eBelow):
		set(ExitGate, "e2e")
	case errors.Is(err, ports.ErrToolNotFound), errors.Is(err, tdd.ErrUnsupportedStack):
		set(ExitEnvironment, "tool")
	case errors.Is(err, config.ErrNoAgent):
		set(ExitEnvironment, "noagent")
	case errors.Is(err, ports.ErrTimeout):
		set(ExitEnvironment, "timeout")
	}
	return d
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
			"diag.generic.title":        "SpecForge stopped",
			"diag.interrupted.title":    "Interrupted",
			"diag.interrupted.action":   "the state was saved: continue with `specforge loop --resume`",
			"diag.pending.title":        "A question needs your answer",
			"diag.pending.action":       "answer it in %s or run the command again in a terminal",
			"diag.tampering.title":      "A test file was modified outside RED",
			"diag.tampering.action":     "revert the test change (git checkout -- <file>) and continue with `specforge loop --resume`",
			"diag.tampered.title":       "The specification changed after approval",
			"diag.tampered.action":      "revert the edit or approve the new version with `specforge spec approve`",
			"diag.notsealed.title":      "The specification is not approved",
			"diag.notsealed.action":     "review it and run `specforge spec approve <spec>`",
			"diag.openquestions.title":  "The specification has open questions",
			"diag.openquestions.action": "resolve every [NEEDS CLARIFICATION] and approve the specification again",
			"diag.noscenarios.title":    "The specification has no Gherkin scenarios",
			"diag.noscenarios.action":   "add the acceptance criteria as Gherkin scenarios in a ```gherkin block",
			"diag.state.title":          "Loop state",
			"diag.state.action":         "use --resume to continue or --restart to start over",
			"diag.premature.title":      "The test passed before any implementation",
			"diag.premature.action":     "decide whether the behaviour already exists and run `specforge loop --resume` in a terminal",
			"diag.attempts.title":       "The agent ran out of attempts",
			"diag.attempts.action":      "look at the last failure, fix or guide it, then `specforge loop --resume`",
			"diag.gates.title":          "Quality gates block the scenario",
			"diag.gates.action":         "fix the findings (or install the missing tools) and `specforge loop --resume`",
			"diag.blocked.title":        "The agent is blocked",
			"diag.blocked.action":       "fix the cause and `specforge loop --resume`",
			"diag.contract.title":       "The agent did not follow the response contract",
			"diag.contract.action":      "check the agent output with --trace-io and try again",
			"diag.security.title":       "Security findings block the merge",
			"diag.security.action":      "fix them or settle the doubtful ones; details in docs/security/REPORT.md",
			"diag.auditstep.title":      "The audit could not complete",
			"diag.auditstep.action":     "the audit fails closed: rerun it, or inspect the output with --trace-io",
			"diag.e2e.title":            "E2E scenarios failed",
			"diag.e2e.action":           "see docs/e2e/<spec>/REPORT.md and the screenshots",
			"diag.tool.title":           "A required tool is missing",
			"diag.tool.action":          "install it or configure the stack in specforge.yaml",
			"diag.noagent.title":        "No coding agent configured",
			"diag.noagent.action":       "run `specforge init`",
			"diag.timeout.title":        "A step timed out",
			"diag.timeout.action":       "raise timeouts.agent or timeouts.tests in specforge.yaml",
		},
		"es": {
			"diag.generic.title":        "SpecForge se detuvo",
			"diag.interrupted.title":    "Interrumpido",
			"diag.interrupted.action":   "el estado quedó guardado: continúa con `specforge loop --resume`",
			"diag.pending.title":        "Una pregunta espera tu respuesta",
			"diag.pending.action":       "respóndela en %s o vuelve a lanzar el comando en una terminal",
			"diag.tampering.title":      "Se modificó un fichero de test fuera de RED",
			"diag.tampering.action":     "revierte el cambio del test (git checkout -- <fichero>) y continúa con `specforge loop --resume`",
			"diag.tampered.title":       "La especificación cambió después de aprobarse",
			"diag.tampered.action":      "revierte la edición o aprueba la nueva versión con `specforge spec approve`",
			"diag.notsealed.title":      "La especificación no está aprobada",
			"diag.notsealed.action":     "revísala y ejecuta `specforge spec approve <spec>`",
			"diag.openquestions.title":  "La especificación tiene preguntas abiertas",
			"diag.openquestions.action": "resuelve cada [NEEDS CLARIFICATION] y vuelve a aprobar la especificación",
			"diag.noscenarios.title":    "La especificación no tiene escenarios Gherkin",
			"diag.noscenarios.action":   "añade los criterios de aceptación como escenarios en un bloque ```gherkin",
			"diag.state.title":          "Estado del ciclo",
			"diag.state.action":         "usa --resume para continuar o --restart para empezar de cero",
			"diag.premature.title":      "El test pasó antes de existir implementación",
			"diag.premature.action":     "decide si el comportamiento ya existe y ejecuta `specforge loop --resume` en una terminal",
			"diag.attempts.title":       "El agente agotó sus intentos",
			"diag.attempts.action":      "revisa el último fallo, corrígelo o guíalo y ejecuta `specforge loop --resume`",
			"diag.gates.title":          "Las puertas de calidad bloquean el escenario",
			"diag.gates.action":         "corrige los hallazgos (o instala las herramientas que faltan) y `specforge loop --resume`",
			"diag.blocked.title":        "El agente está bloqueado",
			"diag.blocked.action":       "corrige la causa y `specforge loop --resume`",
			"diag.contract.title":       "El agente no siguió el contrato de respuesta",
			"diag.contract.action":      "revisa su salida con --trace-io y vuelve a intentarlo",
			"diag.security.title":       "Hallazgos de seguridad bloquean el merge",
			"diag.security.action":      "corrígelos o decide los dudosos; detalle en docs/security/REPORT.md",
			"diag.auditstep.title":      "La auditoría no pudo completarse",
			"diag.auditstep.action":     "la auditoría falla cerrada: repítela o revisa la salida con --trace-io",
			"diag.e2e.title":            "Escenarios E2E fallidos",
			"diag.e2e.action":           "revisa docs/e2e/<spec>/REPORT.md y las capturas",
			"diag.tool.title":           "Falta una herramienta necesaria",
			"diag.tool.action":          "instálala o configura el stack en specforge.yaml",
			"diag.noagent.title":        "No hay agente configurado",
			"diag.noagent.action":       "ejecuta `specforge init`",
			"diag.timeout.title":        "Un paso superó su tiempo máximo",
			"diag.timeout.action":       "aumenta timeouts.agent o timeouts.tests en specforge.yaml",
		},
	} {
		for k, v := range cat {
			messages[lang][k] = v
		}
	}
}
