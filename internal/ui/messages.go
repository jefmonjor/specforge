package ui

import "fmt"

// T returns the message for key in lang (falling back to English).
func T(lang, key string, args ...any) string {
	cat, ok := messages[lang]
	if !ok {
		cat = messages["en"]
	}
	text, ok := cat[key]
	if !ok {
		text = messages["en"][key]
	}
	if text == "" {
		text = key
	}
	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	return text
}

var messages = map[string]map[string]string{
	"en": {
		"agent.working":          "%s is working (%s)",
		"tests.running":          "running %s",
		"loop.title":             "SpecForge · Red → Green → Refactor · %s",
		"loop.stack":             "stack %s · agent %s · %d scenario(s)",
		"loop.amended":           "the specification changed: %d scenario(s) to (re)do: %s",
		"loop.scenario":          "Scenario %d/%d · %s · %s",
		"loop.accepted":          "%s accepted",
		"loop.satisfied":         "scenario %d marked as already satisfied by the developer",
		"loop.answered":          "answer recorded in the decisions log",
		"loop.finished":          "all %d scenario(s) passed RED → GREEN → REFACTOR",
		"loop.rejected":          "attempt rejected: %s",
		"gate.line":              "%s · %s · %s",
		"reject.no-contract":     "the agent did not end with the JSON status",
		"reject.false-claim":     "the agent listed files it did not change: %s",
		"reject.no-test":         "no test file changed",
		"reject.no-marker":       "no changed test carries the marker %s",
		"reject.not-compiled":    "the tests do not compile",
		"reject.nothing-ran":     "no test named %s ran",
		"reject.still-failing":   "the test still fails",
		"reject.premature-green": "the test passed before any implementation",
		"reject.unconfirmed":     "the developer rejected the RED: %s",
		"audit.title":            "SpecForge · security audit",
		"audit.target.diff":      "changes since %s · %d chunk(s)",
		"audit.target.full":      "full scan · %d chunk(s)",
		"audit.step":             "chunk %d/%d · %s",
		"audit.retry":            "%s answered without the required JSON: retrying once",
		"audit.empty":            "no changes to audit",
		"audit.passed":           "no finding blocks the merge (threshold %s) · confirmed %d · to validate %d · rejected %d",
		"audit.report":           "report: %s",
		"e2e.title":              "SpecForge · E2E · %s",
		"e2e.scenario":           "Scenario %d/%d · %s",
		"e2e.step":               "step %d · %s %s → %s",
		"e2e.result":             "%s · %d/%d Then verified",
		"e2e.passed":             "pass rate %.0f%% · report in %s",
	},
	"es": {
		"agent.working":          "%s está trabajando (%s)",
		"tests.running":          "ejecutando %s",
		"loop.title":             "SpecForge · Red → Green → Refactor · %s",
		"loop.stack":             "stack %s · agente %s · %d escenario(s)",
		"loop.amended":           "la especificación cambió: %d escenario(s) por (re)hacer: %s",
		"loop.scenario":          "Escenario %d/%d · %s · %s",
		"loop.accepted":          "%s aceptado",
		"loop.satisfied":         "escenario %d marcado como ya satisfecho por el desarrollador",
		"loop.answered":          "respuesta registrada en el log de decisiones",
		"loop.finished":          "los %d escenario(s) pasaron RED → GREEN → REFACTOR",
		"loop.rejected":          "intento rechazado: %s",
		"gate.line":              "%s · %s · %s",
		"reject.no-contract":     "el agente no terminó con el JSON de estado",
		"reject.false-claim":     "el agente listó ficheros que no cambió: %s",
		"reject.no-test":         "no cambió ningún fichero de test",
		"reject.no-marker":       "ningún test cambiado lleva el marcador %s",
		"reject.not-compiled":    "los tests no compilan",
		"reject.nothing-ran":     "no se ejecutó ningún test llamado %s",
		"reject.still-failing":   "el test sigue fallando",
		"reject.premature-green": "el test pasó antes de existir implementación",
		"reject.unconfirmed":     "el desarrollador rechazó el RED: %s",
		"audit.title":            "SpecForge · auditoría de seguridad",
		"audit.target.diff":      "cambios desde %s · %d bloque(s)",
		"audit.target.full":      "escaneo completo · %d bloque(s)",
		"audit.step":             "bloque %d/%d · %s",
		"audit.retry":            "%s respondió sin el JSON requerido: un reintento",
		"audit.empty":            "no hay cambios que auditar",
		"audit.passed":           "ningún hallazgo bloquea el merge (umbral %s) · confirmadas %d · por validar %d · descartadas %d",
		"audit.report":           "informe: %s",
		"e2e.title":              "SpecForge · E2E · %s",
		"e2e.scenario":           "Escenario %d/%d · %s",
		"e2e.step":               "paso %d · %s %s → %s",
		"e2e.result":             "%s · %d/%d Then verificados",
		"e2e.passed":             "tasa de éxito %.0f%% · informe en %s",
	},
}
