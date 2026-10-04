package spec

import (
	"reflect"
	"strings"
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

func TestResolveQuestion(t *testing.T) {
	md := "## 12. Open questions\n\n```\n- [NEEDS CLARIFICATION]: Which channel?\n```\n  - [NEEDS CLARIFICATION]: Which channel?\n- [NEEDS CLARIFICATION]: Expiry?\n"
	out, ok := ResolveQuestion(md, "Which channel?", "email", "2026-10-04, Ana")
	if !ok {
		t.Fatal("not resolved")
	}
	if !strings.Contains(out, "  - **Decided:** Which channel? → email (2026-10-04, Ana)") || !strings.Contains(out, "```\n- [NEEDS CLARIFICATION]: Which channel?\n```") {
		t.Fatalf("got:\n%s", out)
	}
	if qs := OpenQuestions(out); len(qs) != 1 || qs[0] != "Expiry?" {
		t.Fatalf("open questions left: %v", qs)
	}
	if _, ok := ResolveQuestion(md, "Unknown?", "x", ""); ok {
		t.Fatal("an unknown question must not resolve")
	}
}
