package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type FindingStatus string

const (
	StatusConfirmed       FindingStatus = "confirmed"
	StatusNeedsValidation FindingStatus = "needs_validation"
	StatusRejected        FindingStatus = "rejected"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// SecurityFinding representa una vulnerabilidad identificada o descartada
type SecurityFinding struct {
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Severity      Severity      `json:"severity"`
	Status        FindingStatus `json:"status"`
	File          string        `json:"file"`
	Line          int           `json:"line"`
	AttackClass   string        `json:"attack_class"`
	Description   string        `json:"description"`
	ProofOfImpact string        `json:"proof_of_impact"`
	Remediation   string        `json:"remediation,omitempty"`
}

// SecurityReport agrupa los resultados de la auditoría conforme a report-schema.json
type SecurityReport struct {
	GeneratedAt          string            `json:"generated_at"`
	TotalConfirmed       int               `json:"total_confirmed"`
	TotalNeedsValidation int               `json:"total_needs_validation"`
	TotalRejected        int               `json:"total_rejected"`
	Findings             []SecurityFinding `json:"findings"`
}

// CoverageLedgerUnit representa una unidad de ataque/código mapeada en Reconnaissance
type CoverageLedgerUnit struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	File          string   `json:"file"`
	EntryPoints   []string `json:"entry_points"`
	TrustBoundary string   `json:"trust_boundary"`
	AttackSurface []string `json:"attack_surface"`
	Notes         string   `json:"notes,omitempty"`
}

// CoverageLedger representa el libro mayor de cobertura de auditoría
type CoverageLedger struct {
	Units []CoverageLedgerUnit `json:"units"`
}

var jsonBlockRegex = regexp.MustCompile("(?s)```(?:json)?\\s*([{\\[].*?[}\\]])\\s*```")

// ExtractJSONFromMarkdown extrae el bloque JSON limpio de respuestas del LLM con texto o Markdown
func ExtractJSONFromMarkdown(raw string) []byte {
	trimmed := strings.TrimSpace(raw)

	// 1. Si ya es JSON puro
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		return []byte(trimmed)
	}

	// 2. Extraer de bloque de código Markdown
	matches := jsonBlockRegex.FindStringSubmatch(trimmed)
	if len(matches) > 1 {
		return []byte(strings.TrimSpace(matches[1]))
	}

	// 3. Fallback: buscar primera llave y última llave
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return []byte(trimmed[start : end+1])
	}

	return []byte(trimmed)
}

// ValidateFindingsAgainstSchema valida estrictamente en Go puro que el JSON cumple report-schema.json
func ValidateFindingsAgainstSchema(jsonData []byte, schemaData []byte) (*SecurityReport, error) {
	cleanJSON := ExtractJSONFromMarkdown(string(jsonData))

	var report SecurityReport
	if err := json.Unmarshal(cleanJSON, &report); err != nil {
		return nil, fmt.Errorf("error deserializando JSON de auditoría: %w (muestra: %s)", err, truncateSnippet(string(cleanJSON), 200))
	}

	if report.GeneratedAt == "" {
		report.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	}

	validSeverities := map[Severity]bool{
		SeverityCritical: true,
		SeverityHigh:     true,
		SeverityMedium:   true,
		SeverityLow:      true,
		SeverityInfo:     true,
	}

	validStatuses := map[FindingStatus]bool{
		StatusConfirmed:       true,
		StatusNeedsValidation: true,
		StatusRejected:        true,
	}

	confirmedCount := 0
	needsValCount := 0
	rejectedCount := 0

	for i, f := range report.Findings {
		if strings.TrimSpace(f.ID) == "" {
			return nil, fmt.Errorf("hallazgo #%d carece de 'id'", i+1)
		}
		if strings.TrimSpace(f.Title) == "" {
			return nil, fmt.Errorf("hallazgo %s carece de 'title'", f.ID)
		}
		if !validSeverities[f.Severity] {
			return nil, fmt.Errorf("hallazgo %s tiene severidad inválida '%s'", f.ID, f.Severity)
		}
		if !validStatuses[f.Status] {
			return nil, fmt.Errorf("hallazgo %s tiene status inválido '%s'", f.ID, f.Status)
		}
		if strings.TrimSpace(f.File) == "" {
			return nil, fmt.Errorf("hallazgo %s carece de 'file'", f.ID)
		}
		if strings.TrimSpace(f.AttackClass) == "" {
			return nil, fmt.Errorf("hallazgo %s carece de 'attack_class'", f.ID)
		}
		if strings.TrimSpace(f.Description) == "" {
			return nil, fmt.Errorf("hallazgo %s carece de 'description'", f.ID)
		}
		if strings.TrimSpace(f.ProofOfImpact) == "" {
			return nil, fmt.Errorf("hallazgo %s carece de 'proof_of_impact'", f.ID)
		}

		switch f.Status {
		case StatusConfirmed:
			confirmedCount++
		case StatusNeedsValidation:
			needsValCount++
		case StatusRejected:
			rejectedCount++
		}
	}

	report.TotalConfirmed = confirmedCount
	report.TotalNeedsValidation = needsValCount
	report.TotalRejected = rejectedCount

	return &report, nil
}

// HasFailures determina si el reporte contiene vulnerabilidades que deban bloquear según failOn
func (r *SecurityReport) HasFailures(failOn string) (bool, []SecurityFinding) {
	threshold := strings.ToLower(strings.TrimSpace(failOn))
	if threshold == "" {
		threshold = "confirmed"
	}

	var violating []SecurityFinding
	for _, f := range r.Findings {
		if f.Status != StatusConfirmed {
			continue
		}

		switch threshold {
		case "confirmed":
			violating = append(violating, f)
		case "critical":
			if f.Severity == SeverityCritical {
				violating = append(violating, f)
			}
		case "high":
			if f.Severity == SeverityCritical || f.Severity == SeverityHigh {
				violating = append(violating, f)
			}
		case "medium":
			if f.Severity == SeverityCritical || f.Severity == SeverityHigh || f.Severity == SeverityMedium {
				violating = append(violating, f)
			}
		}
	}

	return len(violating) > 0, violating
}

// RenderMarkdown genera un reporte legible en Markdown para docs/security/REPORT.md
func (r *SecurityReport) RenderMarkdown() string {
	var sb strings.Builder

	sb.WriteString("# 🛡️ Informe de Auditoría de Seguridad Adversarial (Cloudflare Harness)\n\n")
	sb.WriteString(fmt.Sprintf("- **Fecha de Generación:** %s\n", r.GeneratedAt))
	sb.WriteString(fmt.Sprintf("- **Vulnerabilidades Confirmadas:** %d\n", r.TotalConfirmed))
	sb.WriteString(fmt.Sprintf("- **Sospechas por Validar (Needs Validation):** %d\n", r.TotalNeedsValidation))
	sb.WriteString(fmt.Sprintf("- **Falsos Positivos Descartados:** %d\n\n", r.TotalRejected))

	if r.TotalConfirmed == 0 {
		sb.WriteString("## ✅ Estado de Seguridad: Aprobado\n")
		sb.WriteString("No se identificaron vulnerabilidades confirmadas en la superficie de código auditada.\n\n")
	} else {
		sb.WriteString("## 🚨 Vulnerabilidades Confirmadas\n\n")
		for _, f := range r.Findings {
			if f.Status == StatusConfirmed {
				sb.WriteString(fmt.Sprintf("### [%s] %s (Severidad: %s)\n", f.ID, f.Title, strings.ToUpper(string(f.Severity))))
				sb.WriteString(fmt.Sprintf("- **Archivo:** `%s:%d`\n", f.File, f.Line))
				sb.WriteString(fmt.Sprintf("- **Clase de Ataque:** `%s`\n", f.AttackClass))
				sb.WriteString(fmt.Sprintf("- **Descripción:** %s\n", f.Description))
				sb.WriteString(fmt.Sprintf("- **Prueba de Impacto:** %s\n", f.ProofOfImpact))
				if f.Remediation != "" {
					sb.WriteString(fmt.Sprintf("- **Remediación:** %s\n", f.Remediation))
				}
				sb.WriteString("\n---\n\n")
			}
		}
	}

	if r.TotalNeedsValidation > 0 {
		sb.WriteString("## ⚠️ Puntos que Requieren Validación Dinámica (Sandbox)\n\n")
		for _, f := range r.Findings {
			if f.Status == StatusNeedsValidation {
				sb.WriteString(fmt.Sprintf("* **[%s] %s** en `%s:%d` (Clase: %s)\n  %s\n",
					f.ID, f.Title, f.File, f.Line, f.AttackClass, f.Description))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func truncateSnippet(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
