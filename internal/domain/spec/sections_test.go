package spec

import "testing"

func TestSection(t *testing.T) {
	md := "# Spec\n\n## 2. Lenguaje Ubicuo (Glosario)\n- **Saldo**: dinero disponible.\n\n### Notas\nsub-section stays\n\n## 3. Invariantes del Dominio\n- INV-01: saldo >= 0\n\n```gherkin\n# not a heading\n```\n\n## 4. Otra\nx\n"
	if got := Section(md, GlossaryTitle); got != "- **Saldo**: dinero disponible.\n\n### Notas\nsub-section stays" {
		t.Errorf("glossary = %q", got)
	}
	if got := Section(md, InvariantsTitle); got != "- INV-01: saldo >= 0\n\n```gherkin\n# not a heading\n```" {
		t.Errorf("invariants = %q", got)
	}
	if got := Section("# none\n", GlossaryTitle); got != "" {
		t.Errorf("missing section = %q", got)
	}
}
