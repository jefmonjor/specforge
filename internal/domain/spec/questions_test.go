package spec

import (
	"reflect"
	"testing"
)

func TestOpenQuestions(t *testing.T) {
	md := `# Spec
La IA no avanzará mientras quede algún [NEEDS CLARIFICATION] abierto.

## Cuestiones abiertas
- [NEEDS CLARIFICATION]: ¿Aplica el tiempo de cortesía en Nochevieja?
* [needs clarification] ¿Se cobra comisión por cancelación tardía?
1. [NEEDS CLARIFICATION] ¿Quién aprueba excepciones?
- [NEEDS CLARIFICATION]: <Anota aquí cualquier ambigüedad antes de aprobar.>
- [NEEDS CLARIFICATION]:

` + "```markdown\n- [NEEDS CLARIFICATION]: inside a code block\n```\n" + `
<!--
- [NEEDS CLARIFICATION]: inside a comment
-->
`
	want := []string{
		"¿Aplica el tiempo de cortesía en Nochevieja?",
		"¿Se cobra comisión por cancelación tardía?",
		"¿Quién aprueba excepciones?",
	}
	if got := OpenQuestions(md); !reflect.DeepEqual(got, want) {
		t.Fatalf("OpenQuestions = %q, want %q", got, want)
	}
}

func TestOpenQuestionsEmpty(t *testing.T) {
	if got := OpenQuestions("# Spec\n\n## Cuestiones abiertas\n\nNinguna.\n"); len(got) != 0 {
		t.Fatalf("want none, got %q", got)
	}
}
