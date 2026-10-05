package delivery

import (
	"fmt"
	"strings"
)

type labels struct {
	delivery, spec, plan, noPlan, approvedBy, on, notApproved, scenarios, summary string
	finished, loop, satisfiedN, pendingN                                          string
	cols                                                                          [6]string
	satisfied, pending, reviewed, rejected                                        string
	decisions, open, lessons, checks, outOfScope, none                            string
	testsLine, gatesLine, secLine, secNone, e2eLine, e2eNone                      string
	prSummary, prScenarios, prDecisions, prChecks, prNotDone, prImplements        string
	incomplete                                                                    string
	known, baseLine, baseClean                                                    string
	risk                                                                          string
	size, sizeOver, slices, sliceLine, oversized, lines                           string
	reviewNote, followUps, branchLine, branchNone, openFindings                   string
	verifyNote, verifySkipped, verifyLine, verifyOpen                             string
}

var catalog = map[string]labels{
	"en": {
		delivery: "Delivery", spec: "Specification", plan: "Plan", noPlan: "no plan (the loop worked from the specification)",
		approvedBy: "approved by", on: "on", notApproved: "not approved", scenarios: "Scenarios", summary: "Summary",
		finished: "%d/%d finished", loop: "%d through RED → GREEN → REFACTOR", satisfiedN: "%d already satisfied", pendingN: "%d pending",
		cols:      [6]string{"#", "Scenario", "Tests", "Commit", "Gates", "Notes"},
		satisfied: "already satisfied", pending: "⏳ not finished", reviewed: "%d change(s) requested in review", rejected: "%d rejected attempt(s)",
		decisions: "Decisions taken during development", open: "⚠ Questions still open", lessons: "Lessons recorded",
		checks: "Checks", outOfScope: "Out of scope", none: "none",
		testsLine: "Tests: every scenario test was fingerprinted after RED and verified unchanged through GREEN and REFACTOR.",
		gatesLine: "Quality gates: per scenario above. ✓ passed · ⚠ skipped (the tool did not run) · ✗ failed.",
		secLine:   "Security audit: %d confirmed · %d to validate · %d rejected.", secNone: "Security audit: not run.",
		e2eLine: "E2E: %d/%d scenarios verified in a browser (%.0f%%).", e2eNone: "E2E: not run.",
		prSummary: "Summary", prScenarios: "Scenarios", prDecisions: "Decisions", prChecks: "Checks", prNotDone: "Not done",
		prImplements:  "Implements specification %s · %s (`%s`), built test-first with SpecForge.",
		incomplete:    "⚠ This delivery is incomplete: see the pending items below.",
		known:         "Known failures (already failing before the loop; not counted against it)",
		baseLine:      "Baseline: %d test(s) already failed before the loop (`%s`); listed above, they never blocked a scenario.",
		baseClean:     "Baseline: the whole suite passed before the loop (`%s`); any new failure blocked.",
		risk:          "risk %s: %s",
		size:          "%d authored line(s) · budget %d",
		sizeOver:      "%d authored line(s), over the budget of %d: see the suggested slices",
		slices:        "Suggested slices (stacked pull requests, in order)",
		sliceLine:     "Slice %d · scenario(s) %s · %d line(s) · `git branch specforge/slice-%d %s`",
		oversized:     " · ⚠ larger than the budget on its own: review it whole, it is never cut",
		lines:         "%d line(s)",
		reviewNote:    "review: %d lens(es) · %d corrected · %d follow-up(s) · %d discarded",
		followUps:     "Review follow-ups (not blocking: pre-existing, or accepted by the developer)",
		branchLine:    "Branch review: %d lens(es) · %d reported · %d still open · %d follow-up(s) · %d discarded (every finding checked against the diff).",
		branchNone:    "Branch review: not run.",
		openFindings:  "⚠ Review findings still open (they block)",
		verifyNote:    "verify: %d met · %d unmet · %d unverified",
		verifySkipped: "verify skipped: %s",
		verifyLine:    "Independent verification of the specification: %d met · %d unmet · %d unverified (probes derived from the specification, run in a copy).",
		verifyOpen:    "⚠ Requirements the verifier showed broken",
	},
	"es": {
		delivery: "Entrega", spec: "Especificación", plan: "Plan", noPlan: "sin plan (el ciclo trabajó desde la especificación)",
		approvedBy: "aprobada por", on: "el", notApproved: "sin aprobar", scenarios: "Escenarios", summary: "Resumen",
		finished: "%d/%d terminados", loop: "%d por RED → GREEN → REFACTOR", satisfiedN: "%d ya satisfechos", pendingN: "%d pendientes",
		cols:      [6]string{"#", "Escenario", "Tests", "Commit", "Puertas", "Notas"},
		satisfied: "ya satisfecho", pending: "⏳ sin terminar", reviewed: "%d cambio(s) pedidos en revisión", rejected: "%d intento(s) rechazados",
		decisions: "Decisiones tomadas durante el desarrollo", open: "⚠ Preguntas aún abiertas", lessons: "Lecciones registradas",
		checks: "Comprobaciones", outOfScope: "Fuera de alcance", none: "ninguna",
		testsLine: "Tests: cada test de escenario se fijó por huella tras RED y se verificó sin cambios en GREEN y REFACTOR.",
		gatesLine: "Puertas de calidad: por escenario arriba. ✓ pasada · ⚠ omitida (la herramienta no se ejecutó) · ✗ fallida.",
		secLine:   "Auditoría de seguridad: %d confirmados · %d por validar · %d descartados.", secNone: "Auditoría de seguridad: no ejecutada.",
		e2eLine: "E2E: %d/%d escenarios verificados en navegador (%.0f%%).", e2eNone: "E2E: no ejecutado.",
		prSummary: "Resumen", prScenarios: "Escenarios", prDecisions: "Decisiones", prChecks: "Comprobaciones", prNotDone: "Sin hacer",
		prImplements:  "Implementa la especificación %s · %s (`%s`), construida con tests primero mediante SpecForge.",
		incomplete:    "⚠ Esta entrega está incompleta: mira los pendientes más abajo.",
		known:         "Fallos conocidos (ya fallaban antes del loop; no cuentan contra él)",
		baseLine:      "Línea base: %d test(s) ya fallaban antes del loop (`%s`); listados arriba, nunca bloquearon un escenario.",
		baseClean:     "Línea base: la suite completa pasaba antes del loop (`%s`); cualquier fallo nuevo bloqueó.",
		risk:          "riesgo %s: %s",
		size:          "%d línea(s) escritas · presupuesto %d",
		sizeOver:      "%d línea(s) escritas, por encima del presupuesto de %d: mira los cortes propuestos",
		slices:        "Cortes propuestos (pull requests apiladas, en orden)",
		sliceLine:     "Corte %d · escenario(s) %s · %d línea(s) · `git branch specforge/slice-%d %s`",
		oversized:     " · ⚠ supera el presupuesto por sí solo: revísalo entero, nunca se corta",
		lines:         "%d línea(s)",
		reviewNote:    "revisión: %d lente(s) · %d corregidos · %d seguimiento(s) · %d descartados",
		followUps:     "Seguimientos de la revisión (no bloquean: ya existían o los aceptó el desarrollador)",
		branchLine:    "Revisión de la rama: %d lente(s) · %d informados · %d aún abiertos · %d seguimiento(s) · %d descartados (cada hallazgo contrastado con el diff).",
		branchNone:    "Revisión de la rama: no ejecutada.",
		openFindings:  "⚠ Hallazgos de revisión aún abiertos (bloquean)",
		verifyNote:    "verificación: %d cumplidos · %d incumplidos · %d sin verificar",
		verifySkipped: "verificación omitida: %s",
		verifyLine:    "Verificación independiente de la especificación: %d cumplidos · %d incumplidos · %d sin verificar (sondeos derivados de la especificación, ejecutados en una copia).",
		verifyOpen:    "⚠ Requisitos que el verificador demostró incumplidos",
	},
}

func lbl(lang string) labels {
	if l, ok := catalog[lang]; ok {
		return l
	}
	return catalog["en"]
}

// Markdown renders DELIVERY.md.
func (t Trace) Markdown(lang string) string {
	l := lbl(lang)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s · %s %s\n\n", l.delivery, t.ID, t.Title)
	if !t.Complete() {
		b.WriteString(l.incomplete + "\n\n")
	}
	fmt.Fprintf(&b, "**%s** %s\n\n", l.spec, approval(l, t.Spec))
	if t.Plan != nil {
		fmt.Fprintf(&b, "**%s** %s\n\n", l.plan, approval(l, *t.Plan))
	} else {
		fmt.Fprintf(&b, "**%s** %s\n\n", l.plan, l.noPlan)
	}
	fmt.Fprintf(&b, "**%s** %s\n\n", l.scenarios, t.counts(l))
	if line := t.sizeLine(l); line != "" {
		b.WriteString(line + "\n\n")
	}
	b.WriteString(t.table(l, true))
	if len(t.Slices) > 0 {
		section(&b, l.slices, t.sliceLines(l), "")
	}
	if t.Baseline != nil && len(t.Baseline.Failures) > 0 {
		section(&b, l.known, code(t.Baseline.Failures), "")
	}
	if r := t.Checks.BranchReview; r != nil && len(r.Open) > 0 {
		section(&b, l.openFindings, r.Open, "")
	}
	if v := t.Checks.Verify; v != nil && len(v.Open) > 0 {
		section(&b, l.verifyOpen, v.Open, "")
	}
	if f := t.followUps(); len(f) > 0 {
		section(&b, l.followUps, f, "")
	}
	section(&b, l.decisions, t.Decisions, l.none)
	if len(t.Pending) > 0 {
		section(&b, l.open, t.Pending, "")
	}
	if len(t.Lessons) > 0 {
		section(&b, l.lessons, t.Lessons, "")
	}
	fmt.Fprintf(&b, "\n## %s\n\n", l.checks)
	for _, line := range t.checkLines(l) {
		b.WriteString("- " + line + "\n")
	}
	if t.OutOfScope != "" {
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", l.outOfScope, t.OutOfScope)
	}
	return b.String()
}

// PRBody renders PR_BODY.md. When the repository has a pull request
// template, its headings are kept: SpecForge's summary goes under the first
// one and the trace after the template, so nothing the team asks for is
// dropped.
func (t Trace) PRBody(lang, template string) string {
	l := lbl(lang)
	var summary strings.Builder
	fmt.Fprintf(&summary, l.prImplements+"\n\n", t.ID, t.Title, t.Spec.Path)
	if !t.Complete() {
		summary.WriteString(l.incomplete + "\n\n")
	}
	summary.WriteString(t.counts(l) + "\n")
	if line := t.sizeLine(l); line != "" {
		summary.WriteString("\n" + line + "\n")
	}

	var trace strings.Builder
	fmt.Fprintf(&trace, "\n## %s\n\n%s", l.prScenarios, t.table(l, false))
	section(&trace, l.prDecisions, t.Decisions, l.none)
	fmt.Fprintf(&trace, "\n## %s\n\n", l.prChecks)
	for _, line := range t.checkLines(l) {
		trace.WriteString("- " + line + "\n")
	}
	var notDone []string
	for _, sc := range t.Scenarios {
		if sc.Status == Pending {
			notDone = append(notDone, fmt.Sprintf("%s · %s", sc.Marker, sc.Title))
		}
	}
	notDone = append(notDone, t.Pending...)
	if len(notDone) > 0 {
		section(&trace, l.prNotDone, notDone, "")
	}

	if strings.TrimSpace(template) == "" {
		return fmt.Sprintf("## %s\n\n%s%s", l.prSummary, summary.String(), trace.String())
	}
	return mergeTemplate(template, summary.String()) + trace.String()
}

// mergeTemplate puts summary under the first heading of template.
func mergeTemplate(template, summary string) string {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(template), "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			out := append(append(append([]string{}, lines[:i+1]...), "", strings.TrimRight(summary, "\n")), lines[i+1:]...)
			return strings.Join(out, "\n") + "\n"
		}
	}
	return strings.TrimRight(summary, "\n") + "\n\n" + strings.Join(lines, "\n") + "\n"
}

func approval(l labels, a Approval) string {
	s := "`" + a.Path + "` · "
	if a.ApprovedBy == "" {
		return s + l.notApproved
	}
	s += fmt.Sprintf("%s %s %s %s", l.approvedBy, a.ApprovedBy, l.on, dateOf(a.ApprovedAt))
	if a.Seal != "" {
		s += " · `" + shortSeal(a.Seal) + "`"
	}
	return s
}

func (t Trace) counts(l labels) string {
	finished := t.Count(Done) + t.Count(Satisfied)
	parts := []string{fmt.Sprintf(l.finished, finished, len(t.Scenarios)), fmt.Sprintf(l.loop, t.Count(Done))}
	if n := t.Count(Satisfied); n > 0 {
		parts = append(parts, fmt.Sprintf(l.satisfiedN, n))
	}
	if n := t.Count(Pending); n > 0 {
		parts = append(parts, fmt.Sprintf(l.pendingN, n))
	}
	return strings.Join(parts, " · ")
}

func (t Trace) table(l labels, notes bool) string {
	var b strings.Builder
	cols := l.cols[:]
	if !notes {
		cols = cols[:4]
	}
	b.WriteString("| " + strings.Join(cols, " | ") + " |\n|" + strings.Repeat(" :--- |", len(cols)) + "\n")
	for _, sc := range t.Scenarios {
		var tests []string
		for _, ts := range sc.Tests {
			cell := "`" + ts.File + "`"
			if len(ts.Names) > 0 {
				cell += " · `" + strings.Join(ts.Names, "`, `") + "`"
			}
			tests = append(tests, cell)
		}
		commit := "—"
		if sc.Commit != "" {
			commit = "`" + short(sc.Commit) + "`"
		}
		row := []string{fmt.Sprint(sc.Index), escape(sc.Title), orDash(strings.Join(tests, "<br>")), commit}
		if notes {
			row = append(row, orDash(gateIcons(sc.Gates)), orDash(strings.Join(sc.notes(l), "; ")))
		}
		b.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
	return b.String()
}

func (sc Scenario) notes(l labels) []string {
	var out []string
	switch sc.Status {
	case Satisfied:
		out = append(out, l.satisfied)
	case Pending:
		out = append(out, l.pending)
	}
	if sc.Risk != nil {
		out = append(out, fmt.Sprintf(l.risk, sc.Risk.Tier, escape(strings.Join(sc.Risk.Reasons, ", "))))
	}
	if sc.Lines > 0 {
		out = append(out, fmt.Sprintf(l.lines, sc.Lines))
	}
	if v := sc.Verify; v != nil && v.Skipped != "" {
		out = append(out, fmt.Sprintf(l.verifySkipped, escape(v.Skipped)))
	} else if v != nil {
		out = append(out, fmt.Sprintf(l.verifyNote, v.Met, v.Unmet, v.Unverified))
	}
	if r := sc.Review; r != nil && len(r.Lenses) > 0 {
		out = append(out, fmt.Sprintf(l.reviewNote, len(r.Lenses), r.Corrected, len(r.FollowUps), r.Discarded))
	}
	if sc.ReviewNotes > 0 {
		out = append(out, fmt.Sprintf(l.reviewed, sc.ReviewNotes))
	}
	if sc.Rejections > 0 {
		out = append(out, fmt.Sprintf(l.rejected, sc.Rejections))
	}
	return out
}

// followUps lists every scenario's and the branch review's follow-ups.
func (t Trace) followUps() []string {
	var out []string
	for _, sc := range t.Scenarios {
		if sc.Review != nil {
			for _, f := range sc.Review.FollowUps {
				out = append(out, sc.Marker+" · "+escape(f))
			}
		}
		if sc.Verify != nil {
			for _, id := range sc.Verify.FollowUps {
				out = append(out, sc.Marker+" · "+id+" · still broken after its correction (verify)")
			}
		}
	}
	if r := t.Checks.BranchReview; r != nil {
		for _, f := range r.FollowUps {
			out = append(out, escape(f))
		}
	}
	return out
}

func (t Trace) sizeLine(l labels) string {
	switch {
	case t.Budget <= 0 || t.Lines() == 0:
		return ""
	case len(t.Slices) > 0:
		return fmt.Sprintf(l.sizeOver, t.Lines(), t.Budget)
	}
	return fmt.Sprintf(l.size, t.Lines(), t.Budget)
}

func (t Trace) sliceLines(l labels) []string {
	out := make([]string, len(t.Slices))
	for i, s := range t.Slices {
		idx := make([]string, len(s.Scenarios))
		for j, n := range s.Scenarios {
			idx[j] = fmt.Sprint(n)
		}
		out[i] = fmt.Sprintf(l.sliceLine, s.N, strings.Join(idx, ", "), s.Lines, s.N, short(s.LastSHA))
		if s.Oversized {
			out[i] += l.oversized
		}
	}
	return out
}

func (t Trace) checkLines(l labels) []string {
	lines := []string{l.testsLine, l.gatesLine}
	if b := t.Baseline; b != nil {
		if len(b.Failures) > 0 {
			lines = append(lines, fmt.Sprintf(l.baseLine, len(b.Failures), b.Command))
		} else {
			lines = append(lines, fmt.Sprintf(l.baseClean, b.Command))
		}
	}
	if s := t.Checks.Security; s != nil {
		lines = append(lines, fmt.Sprintf(l.secLine, s.Confirmed, s.NeedsValidation, s.Rejected))
	} else {
		lines = append(lines, l.secNone)
	}
	if r := t.Checks.BranchReview; r != nil {
		lines = append(lines, fmt.Sprintf(l.branchLine, len(r.Lenses), r.Reported, len(r.Open), len(r.FollowUps), r.Discarded))
	}
	if v := t.Checks.Verify; v != nil {
		lines = append(lines, fmt.Sprintf(l.verifyLine, v.Met, v.Unmet, v.Unverified))
	}
	if e := t.Checks.E2E; e != nil {
		lines = append(lines, fmt.Sprintf(l.e2eLine, e.Passed, e.Scenarios, e.PassRate*100))
	} else {
		lines = append(lines, l.e2eNone)
	}
	return lines
}

func section(b *strings.Builder, title string, items []string, empty string) {
	fmt.Fprintf(b, "\n## %s\n\n", title)
	if len(items) == 0 {
		b.WriteString("- " + empty + "\n")
		return
	}
	for _, it := range items {
		b.WriteString("- " + it + "\n")
	}
}

// gateIcons turns "lint=passed duplication=skipped" into "lint ✓ · duplication ⚠".
func gateIcons(summary string) string {
	var out []string
	for _, f := range strings.Fields(summary) {
		name, status, _ := strings.Cut(f, "=")
		icon := map[string]string{"passed": "✓", "skipped": "⚠", "failed": "✗"}[status]
		if icon == "" {
			icon = status
		}
		out = append(out, name+" "+icon)
	}
	return strings.Join(out, " · ")
}

func dateOf(rfc3339 string) string {
	if len(rfc3339) >= 10 {
		return rfc3339[:10]
	}
	return rfc3339
}

func shortSeal(seal string) string {
	if i := strings.LastIndex(seal, ":"); i >= 0 && len(seal) > i+13 {
		return seal[:i+13] + "…"
	}
	return seal
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func code(items []string) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = "`" + it + "`"
	}
	return out
}

func escape(s string) string { return strings.ReplaceAll(s, "|", "\\|") }
