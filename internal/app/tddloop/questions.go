package tddloop

import (
	"context"
	"fmt"
	"strings"

	"github.com/jefmonjor/specforge/v6/internal/domain/tdd"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// choice is a localized question with fixed options.
type choice struct {
	text    string
	options []string
}

var choices = map[string]map[string]choice{
	"en": {
		"premature": {
			text:    "The test for scenario %d passed before any implementation. Is this behaviour already implemented?",
			options: []string{"Yes: mark the scenario as already satisfied", "No: ask the agent for a stricter test", "Stop the loop"},
		},
		"review": {
			text:    "Review scenario %d (%s): do you accept it? Type what should change to send it back to GREEN.",
			options: []string{"Accept", "Back to RED: the test does not express the scenario"},
		},
		"raised": {
			text: "The agent asked for more scrutiny of scenario %d",
		},
		"escalated": {
			text:    "Scenario %d · review finding %s at %s:%d, severe but its cause or evidence is unclear: %s What should happen to it?",
			options: []string{"Keep it as a follow-up", "Correct it now", "Stop the loop"},
		},
		"budget": {
			text:    "Scenario %d: the correction changed %d lines, over its budget of %d: that is a redesign rather than a fix. Accept it?",
			options: []string{"Accept the larger correction", "Stop the loop"},
		},
		"regression": {
			text:    "Scenario %d · finding %s is not resolved after the correction: %s There is no second automatic correction. What now?",
			options: []string{"Keep it as a follow-up", "Stop the loop"},
		},
		"unmet": {
			text:    "Scenario %d · %s is still broken after the correction: `%s` prints `%s`; the specification expects: %s. There is no second automatic correction. What now?",
			options: []string{"Keep it as a follow-up", "Stop the loop"},
		},
		"regression-tests": {
			text:    "Scenario %d: the verifier proposes regression tests: %s. Add them to the project (the whole suite must still pass)?",
			options: []string{"Add them", "Skip them"},
		},
		"surfaces": {
			text:    "Scenario %d: the agent changed files that are not in the approved plan: %s. Accept them for this specification, or refuse them (the agent has to put them back)?",
			options: []string{"Accept them", "Refuse them"},
		},
		"inexact": {
			text:    "The %s runner gives no report, so SpecForge cannot tell why the test fails. Is this a valid RED (the test fails on its assertion)? The output is below.",
			options: []string{"Yes: accept it", "No: ask the agent to fix the test"},
		},
	},
	"es": {
		"raised": {
			text: "El agente pidió más escrutinio para el escenario %d",
		},
		"escalated": {
			text:    "Escenario %d · hallazgo de revisión %s en %s:%d, grave pero con causa o evidencia poco clara: %s ¿Qué hacemos con él?",
			options: []string{"Dejarlo como seguimiento", "Corregirlo ahora", "Detener el ciclo"},
		},
		"budget": {
			text:    "Escenario %d: la corrección cambió %d líneas, por encima de su presupuesto de %d: es un rediseño más que un arreglo. ¿La aceptas?",
			options: []string{"Aceptar la corrección mayor", "Detener el ciclo"},
		},
		"regression": {
			text:    "Escenario %d · el hallazgo %s no quedó resuelto tras la corrección: %s No hay una segunda corrección automática. ¿Y ahora?",
			options: []string{"Dejarlo como seguimiento", "Detener el ciclo"},
		},
		"unmet": {
			text:    "Escenario %d · %s sigue roto tras la corrección: `%s` imprime `%s`; la especificación espera: %s. No hay una segunda corrección automática. ¿Y ahora?",
			options: []string{"Dejarlo como seguimiento", "Detener el ciclo"},
		},
		"regression-tests": {
			text:    "Escenario %d: el verificador propone tests de regresión: %s. ¿Los añado al proyecto (la suite completa debe seguir pasando)?",
			options: []string{"Añadirlos", "Omitirlos"},
		},
		"surfaces": {
			text:    "Escenario %d: el agente cambió ficheros que no están en el plan aprobado: %s. ¿Los aceptas para esta especificación o los rechazas (el agente tendrá que deshacerlos)?",
			options: []string{"Aceptarlos", "Rechazarlos"},
		},
		"premature": {
			text:    "El test del escenario %d pasó antes de existir implementación. ¿Este comportamiento ya está implementado?",
			options: []string{"Sí: marcar el escenario como ya satisfecho", "No: pedir al agente un test más estricto", "Detener el ciclo"},
		},
		"review": {
			text:    "Revisa el escenario %d (%s): ¿lo aceptas? Escribe qué debe cambiar para devolverlo a GREEN.",
			options: []string{"Aceptar", "Volver a RED: el test no expresa el escenario"},
		},
		"inexact": {
			text:    "El runner %s no ofrece informe, así que SpecForge no puede saber por qué falla el test. ¿Es un RED válido (el test falla en su aserción)? La salida está debajo.",
			options: []string{"Sí: aceptarlo", "No: pedir al agente que corrija el test"},
		},
	},
}

func question(lang, key string) choice {
	cat, ok := choices[lang]
	if !ok {
		cat = choices["en"]
	}
	return cat[key]
}

// pick returns the index of the option the developer chose, or -1. The
// prompter returns an option's text when the developer types its number.
func pick(answer string, options []string) int {
	a := strings.ToLower(strings.TrimSpace(answer))
	for i, o := range options {
		if a == strings.ToLower(o) {
			return i
		}
	}
	return -1
}

// choose asks one of SpecForge's own fixed-choice questions about scenario
// sc and returns the option picked. The question's text takes sc.Index
// first, then args.
func (s *Service) choose(ctx context.Context, r *run, sc tdd.ScenarioRef, origin, key string, args ...any) (int, error) {
	q := question(r.o.Language, key)
	text := fmt.Sprintf(q.text, append([]any{sc.Index}, args...)...)
	answer, err := s.d.Asker.Ask(ctx, s.originAs(r, sc, origin), ports.Question{Text: text, Options: q.options, Strict: true})
	if err != nil {
		return -1, err
	}
	return pick(answer, q.options), nil
}
