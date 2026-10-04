package security

import (
	"fmt"
	"strings"
)

type reportLabels struct {
	title, generated, confirmed, needsValidation, rejected, noneConfirmed,
	confirmedHeading, toValidate, file, class, description, impact, remediation, decision string
}

var labels = map[string]reportLabels{
	"es": {"Informe de auditoría de seguridad", "Generado", "Confirmadas", "Por validar", "Descartadas",
		"No hay vulnerabilidades confirmadas en el código auditado.", "Vulnerabilidades confirmadas",
		"Requieren validación", "Fichero", "Clase de ataque", "Descripción", "Prueba de impacto", "Remediación", "Decisión del desarrollador"},
	"en": {"Security audit report", "Generated", "Confirmed", "Needs validation", "Rejected",
		"No confirmed vulnerabilities in the audited code.", "Confirmed vulnerabilities",
		"Needing validation", "File", "Attack class", "Description", "Proof of impact", "Remediation", "Developer decision"},
}

// Markdown renders the report for docs/security/REPORT.md.
func (r *Report) Markdown(lang string) string {
	l, ok := labels[lang]
	if !ok {
		l = labels["en"]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n- **%s:** %s\n- **%s:** %d\n- **%s:** %d\n- **%s:** %d\n\n",
		l.title, l.generated, r.GeneratedAt, l.confirmed, r.TotalConfirmed, l.needsValidation, r.TotalNeedsValidation, l.rejected, r.TotalRejected)

	section := func(heading string, status Status) {
		first := true
		for _, f := range r.Findings {
			if f.Status != status {
				continue
			}
			if first {
				fmt.Fprintf(&b, "## %s\n\n", heading)
				first = false
			}
			fmt.Fprintf(&b, "### [%s] %s (%s)\n\n- **%s:** `%s:%d`\n- **%s:** `%s`\n- **%s:** %s\n- **%s:** %s\n",
				f.ID, f.Title, strings.ToUpper(string(f.Severity)), l.file, f.File, f.Line, l.class, f.AttackClass,
				l.description, f.Description, l.impact, f.ProofOfImpact)
			if f.Remediation != "" {
				fmt.Fprintf(&b, "- **%s:** %s\n", l.remediation, f.Remediation)
			}
			if f.Decision != "" {
				fmt.Fprintf(&b, "- **%s:** %s\n", l.decision, f.Decision)
			}
			b.WriteString("\n")
		}
	}
	if r.TotalConfirmed == 0 {
		b.WriteString(l.noneConfirmed + "\n\n")
	}
	section(l.confirmedHeading, Confirmed)
	section(l.toValidate, NeedsValidation)
	return b.String()
}
