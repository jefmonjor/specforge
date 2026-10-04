package tddloop

import "strings"

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
		"inexact": {
			text:    "The %s runner gives no report, so SpecForge cannot tell why the test fails. Is this a valid RED (the test fails on its assertion)? The output is below.",
			options: []string{"Yes: accept it", "No: ask the agent to fix the test"},
		},
	},
	"es": {
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
