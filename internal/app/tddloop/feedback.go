package tddloop

import "fmt"

// feedback is what the next prompt tells the agent about a rejected
// attempt, in the prompt's language.
var feedbackText = map[string]map[Rejection]string{
	"en": {
		RejectNoContract:   "Your answer did not end with the JSON status object. Do the task again and end with the contract.",
		RejectFalseClaim:   "You listed files that did not change: %s. List only the files you actually wrote.",
		RejectNoTest:       "No test file changed. Write the test for this scenario.",
		RejectNoMarker:     "None of the test files you changed names a test with the marker %s. Rename the test so its name contains it.",
		RejectNotCompiled:  "The tests do not compile or cannot be loaded. Make them compile and fail on an assertion; add minimal production stubs if the code they call does not exist yet.",
		RejectNothingRan:   "No test whose name contains %s ran. Make the marker part of the test name the runner reports.",
		RejectStillFailing: "The test still fails. The output above shows why.",
		RejectPremature:    "The test passed before any implementation. Write a test that fails until the behaviour of this scenario exists.",
		RejectUnconfirmed:  "The developer did not accept the previous test as a valid RED: %s",
		RejectReviewChange: "The developer reviewed this scenario and asked for a change: %s. Change the implementation accordingly; the tests stay as they are.",
		RejectReviewRed:    "The developer reviewed this scenario and sent it back to RED: the test does not express the scenario. Rewrite the test so it checks exactly what the scenario says.",
		RejectNewFailures:  "These tests did not fail before the loop and fail now:\n%s",
		RejectOutsidePlan:  "You changed files that are not in the approved plan: %s. Put them back exactly as they were and do the task within the plan's files; if the plan is wrong, ask (needs_clarification) instead.",
		RejectNoOptions:    "Your question had no options. Derive the candidate answers yourself (files, paths, commands, names, alternatives) and put at least two in \"options\"; the developer picks or trims them.",
	},
	"es": {
		RejectReviewChange: "El desarrollador revisó este escenario y pidió un cambio: %s. Cambia la implementación en consecuencia; los tests se quedan como están.",
		RejectReviewRed:    "El desarrollador revisó este escenario y lo devolvió a RED: el test no expresa el escenario. Reescribe el test para que compruebe exactamente lo que dice el escenario.",
		RejectNoContract:   "Tu respuesta no terminaba con el objeto JSON de estado. Repite la tarea y termina con el contrato.",
		RejectFalseClaim:   "Incluiste ficheros que no cambiaron: %s. Lista solo los ficheros que escribiste.",
		RejectNoTest:       "No cambió ningún fichero de test. Escribe el test de este escenario.",
		RejectNoMarker:     "Ningún fichero de test que cambiaste tiene un test cuyo nombre contenga el marcador %s. Renombra el test para que lo contenga.",
		RejectNotCompiled:  "Los tests no compilan o no se pueden cargar. Haz que compilen y fallen en una aserción; añade stubs mínimos de producción si el código al que llaman aún no existe.",
		RejectNothingRan:   "No se ejecutó ningún test cuyo nombre contenga %s. Haz que el marcador forme parte del nombre que reporta el runner.",
		RejectStillFailing: "El test sigue fallando. La salida de arriba muestra por qué.",
		RejectPremature:    "El test pasó antes de existir la implementación. Escribe un test que falle hasta que el comportamiento de este escenario exista.",
		RejectUnconfirmed:  "El desarrollador no aceptó el test anterior como un RED válido: %s",
		RejectNewFailures:  "Estos tests no fallaban antes del loop y fallan ahora:\n%s",
		RejectOutsidePlan:  "Cambiaste ficheros que no están en el plan aprobado: %s. Déjalos exactamente como estaban y haz la tarea dentro de los ficheros del plan; si el plan está mal, pregunta (needs_clarification).",
		RejectNoOptions:    "Tu pregunta no tenía opciones. Deriva tú las respuestas candidatas (ficheros, rutas, comandos, nombres, alternativas) y pon al menos dos en \"options\"; el desarrollador elige o recorta.",
	},
}

func feedback(lang string, r Rejection, args ...any) string {
	cat, ok := feedbackText[lang]
	if !ok {
		cat = feedbackText["en"]
	}
	text := cat[r]
	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	return text
}
