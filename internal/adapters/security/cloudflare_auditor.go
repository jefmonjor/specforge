package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"specforge/assets"
	"specforge/internal/adapters/ingest"
	"specforge/internal/adapters/storage"
	"specforge/internal/domain"
	"specforge/internal/ports"
)

// CloudflareAdversarialAuditor orquesta la auditoría de 6 fases inspirada en Cloudflare
type CloudflareAdversarialAuditor struct{}

func NewCloudflareAdversarialAuditor() ports.SecurityAuditor {
	return &CloudflareAdversarialAuditor{}
}

func (a *CloudflareAdversarialAuditor) Audit(ctx context.Context, projectDir string, opts ports.SecurityAuditOptions, runner ports.AgentRunner, agentOpts ports.AgentOptions) (*domain.SecurityReport, error) {
	logger := storage.GetLogger()

	// 1. Cargar prompts metodológicos embebidos
	reconPrompt, err := assets.ReadEmbeddedFile("security-audit/RECONNAISSANCE.md")
	if err != nil {
		return nil, fmt.Errorf("error cargando RECONNAISSANCE.md: %w", err)
	}

	huntingPrompt, err := assets.ReadEmbeddedFile("security-audit/HUNTING.md")
	if err != nil {
		return nil, fmt.Errorf("error cargando HUNTING.md: %w", err)
	}

	validationPrompt, err := assets.ReadEmbeddedFile("security-audit/VALIDATION.md")
	if err != nil {
		return nil, fmt.Errorf("error cargando VALIDATION.md: %w", err)
	}

	attackClasses, err := assets.ReadEmbeddedFile("security-audit/ATTACK-CLASSES.md")
	if err != nil {
		return nil, fmt.Errorf("error cargando ATTACK-CLASSES.md: %w", err)
	}

	// 2. Extraer contexto de código (Full Scan o Diff Incremental)
	var codeContext string
	if opts.DiffScan {
		codeContext = getGitDiffContext(projectDir, opts.DiffTarget)
		if strings.TrimSpace(codeContext) == "" {
			logger.Info("Git diff vacío, escaneando cambios en el árbol de trabajo...")
			codeContext = getWorkingTreeChanges(projectDir)
		}
	} else {
		scanner := ingest.NewProjectScanner()
		projCtx, err := scanner.ScanProject(projectDir)
		if err == nil {
			codeContext = projCtx.RenderMarkdown()
		}
	}

	if strings.TrimSpace(codeContext) == "" {
		codeContext = "No se detectaron archivos de código fuente relevantes."
	}

	securityDir := filepath.Join(projectDir, "docs", "security")
	_ = os.MkdirAll(securityDir, 0755)

	// PASO 1: Reconnaissance (Mapeo de Attack Surface & Trust Boundaries)
	logger.Info("[Security Audit] Paso 1: Reconnaissance adversarial...")
	reconPayload := fmt.Sprintf("%s\n\nCÓDIGO / SUPERFICIE A AUDITAR:\n%s", string(reconPrompt), codeContext)
	rawRecon, err := runner.RunHeadless(ctx, reconPayload, agentOpts)
	if err != nil {
		logger.Warn("Aviso en Reconnaissance: %v", err)
	}

	cleanReconJSON := domain.ExtractJSONFromMarkdown(rawRecon)
	ledgerFile := filepath.Join(securityDir, "coverage-ledger.json")
	if len(cleanReconJSON) > 0 {
		_ = os.WriteFile(ledgerFile, cleanReconJSON, 0644)
	}

	// PASO 2: Hunting (Red Team - Caza de candidatos con Attack Classes)
	logger.Info("[Security Audit] Paso 2: Hunting (Red Team)...")
	huntingPayload := fmt.Sprintf("%s\n\n%s\n\nCOVERAGE LEDGER:\n%s\n\nCÓDIGO FUENTE:\n%s",
		string(huntingPrompt), string(attackClasses), string(cleanReconJSON), codeContext)
	rawHunting, err := runner.RunHeadless(ctx, huntingPayload, agentOpts)
	if err != nil {
		return nil, fmt.Errorf("fallo en fase de caza (Hunting): %w", err)
	}

	cleanHuntingJSON := domain.ExtractJSONFromMarkdown(rawHunting)

	// PASO 3: Validation (Blue Team - Intento activo de refutación de candidatos)
	logger.Info("[Security Audit] Paso 3: Validation (Blue Team Verifier)...")
	validationPayload := fmt.Sprintf("%s\n\nHALLAZGOS CANDIDATOS DEL HUNTER:\n%s\n\nCÓDIGO PARA CONTRA-VERIFICACIÓN:\n%s",
		string(validationPrompt), string(cleanHuntingJSON), codeContext)
	rawValidation, err := runner.RunHeadless(ctx, validationPayload, agentOpts)
	if err != nil {
		return nil, fmt.Errorf("fallo en fase de validación (Verifier): %w", err)
	}

	// PASO 4: Validación de Schema y Persistencia de Resultados
	report, err := domain.ValidateFindingsAgainstSchema([]byte(rawValidation), nil)
	if err != nil {
		logger.Warn("Aviso parseando esquema estricto: %v. Generando estructura de recuperación.", err)
		report = &domain.SecurityReport{
			Findings: make([]domain.SecurityFinding, 0),
		}
	}

	findingsFile := filepath.Join(securityDir, "findings.json")
	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(findingsFile, reportJSON, 0644)

	reportMarkdownFile := filepath.Join(securityDir, "REPORT.md")
	_ = os.WriteFile(reportMarkdownFile, []byte(report.RenderMarkdown()), 0644)

	logger.Info("[Security Audit] Auditoría completada. Confirmadas: %d, Needs Validation: %d, Descartadas: %d",
		report.TotalConfirmed, report.TotalNeedsValidation, report.TotalRejected)

	return report, nil
}

func getGitDiffContext(dir, target string) string {
	if target == "" {
		target = "HEAD~1"
	}

	cmd := exec.Command("git", "diff", target)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmdFallback := exec.Command("git", "diff", "--staged")
		cmdFallback.Dir = dir
		if fbOut, fbErr := cmdFallback.CombinedOutput(); fbErr == nil && len(fbOut) > 0 {
			return string(fbOut)
		}
		return ""
	}

	return string(out)
}

func getWorkingTreeChanges(dir string) string {
	cmd := exec.Command("git", "diff")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil && len(out) > 0 {
		return string(out)
	}
	return ""
}
