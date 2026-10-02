package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"specforge/internal/adapters/agent"
	"specforge/internal/adapters/security"
	"specforge/internal/adapters/storage"
	"specforge/internal/domain"
	"specforge/internal/ports"
)

var (
	flagAuditFull   bool
	flagAuditDiff   bool
	flagAuditTarget string
	flagAuditFailOn string
	flagAuditAgent  string
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Auditoría de seguridad adversarial basada en el arnés de Cloudflare",
	Long: `Ejecuta una auditoría de seguridad adversarial en 6 fases:
1. Reconnaissance: Mapeo de superficies de ataque y fronteras de confianza (coverage-ledger.json).
2. Hunting (Red Team): Detección de vulnerabilidades según catálogo de ataques (ATTACK-CLASSES.md).
3. Validation (Blue Team): Intento activo e independiente de refutar falsos positivos.
4. Reporting: Generación de findings.json estricto y reporte legible en docs/security/REPORT.md.`,
	RunE: runAudit,
}

func init() {
	auditCmd.Flags().BoolVar(&flagAuditFull, "full", false, "escaneo completo del código del proyecto")
	auditCmd.Flags().BoolVar(&flagAuditDiff, "diff", false, "escaneo incremental (solo cambios del git diff)")
	auditCmd.Flags().StringVar(&flagAuditTarget, "target", "", "referencia git para el diff (por defecto HEAD~1)")
	auditCmd.Flags().StringVar(&flagAuditFailOn, "fail-on", "confirmed", "umbral para bloquear con error ('confirmed', 'critical', 'high', 'medium')")
	auditCmd.Flags().StringVar(&flagAuditAgent, "agent", "", "agente de desarrollo ('gemini' o 'claude')")

	rootCmd.AddCommand(auditCmd)
}

func runAudit(cmd *cobra.Command, args []string) error {
	logger := storage.GetLogger()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error obteniendo directorio actual: %w", err)
	}

	store := storage.NewConfigStorage(cfgFile)
	cfg, err := store.Load()
	if err != nil {
		cfg = domain.NewDefaultConfig()
	}

	agentName := cfg.Agent
	if flagAuditAgent != "" {
		agentName = strings.ToLower(flagAuditAgent)
	}

	isDiff := flagAuditDiff || !flagAuditFull

	fmt.Println("==================================================================")
	fmt.Println("🛡️ SDD-Free v3.0 — Auditoría Adversarial de Seguridad")
	fmt.Println("==================================================================")
	fmt.Printf("  ✓ Motor:        Arnés Adversarial de Cloudflare (Recon -> Hunter -> Verifier)\n")
	fmt.Printf("  ✓ Agente:       %s\n", agentName)
	if isDiff {
		fmt.Printf("  ✓ Modo:         Incremental Diff (Gate Pre-Merge)\n")
	} else {
		fmt.Printf("  ✓ Modo:         Full Scan (Diagnóstico de Entrada)\n")
	}
	fmt.Printf("  ✓ Umbral Bloqueo: %s\n\n", flagAuditFailOn)

	var runner ports.AgentRunner
	if agentName == "claude" {
		runner = agent.NewClaudeAgentRunner()
	} else {
		runner = agent.NewGeminiAgentRunner()
	}

	agentOpts := ports.AgentOptions{
		Project:    cfg.Auth.Project,
		Location:   cfg.Auth.Location,
		WorkingDir: cwd,
		Debug:      debug,
	}

	auditOpts := ports.SecurityAuditOptions{
		FullScan:   flagAuditFull,
		DiffScan:   isDiff,
		DiffTarget: flagAuditTarget,
		FailOn:     flagAuditFailOn,
	}

	auditor := security.NewCloudflareAdversarialAuditor()
	report, err := auditor.Audit(context.Background(), cwd, auditOpts, runner, agentOpts)
	if err != nil {
		return fmt.Errorf("error ejecutando auditoría de seguridad: %w", err)
	}

	hasFailures, violating := report.HasFailures(flagAuditFailOn)

	if hasFailures {
		fmt.Println("")
		fmt.Println("==================================================================")
		fmt.Println("🚨 DIAGNÓSTICO DE SEGURIDAD AUTOMÁTICO (Cloudflare Adversarial)")
		fmt.Println("==================================================================")
		for _, f := range violating {
			fmt.Printf("  • Vulnerabilidad:  [%s] %s (Status: %s, Severidad: %s)\n",
				f.ID, f.Title, strings.ToUpper(string(f.Status)), strings.ToUpper(string(f.Severity)))
			fmt.Printf("  • Archivo:         %s:%d (Clase: %s)\n", f.File, f.Line, f.AttackClass)
			fmt.Println("  • Detalle:         Revisa docs/security/REPORT.md para remediación.")
			fmt.Println("  ------------------------------------------------------------------")
		}
		fmt.Println("==================================================================")
		logger.Error("Auditoría de seguridad bloqueada: %d vulnerabilidades detectadas", len(violating))
		return fmt.Errorf("Security Gate Bloqueado: Se detectaron %d vulnerabilidades confirmadas que violan el umbral '%s'", len(violating), flagAuditFailOn)
	}

	fmt.Println("\n==================================================================")
	fmt.Printf("✓ Auditoría de seguridad superada. Cero vulnerabilidades bajo el umbral '%s'.\n", flagAuditFailOn)
	fmt.Printf("  ✓ Confirmadas:         %d\n", report.TotalConfirmed)
	fmt.Printf("  ✓ Por Validar:         %d\n", report.TotalNeedsValidation)
	fmt.Printf("  ✓ Falsos Positivos:    %d\n", report.TotalRejected)
	fmt.Println("  ✓ Reporte detallado:   docs/security/REPORT.md")
	fmt.Println("  ✓ Libro de cobertura:  docs/security/coverage-ledger.json")
	fmt.Println("==================================================================")

	return nil
}
