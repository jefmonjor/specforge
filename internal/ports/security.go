package ports

import (
	"context"
	"specforge/internal/domain"
)

// SecurityAuditOptions define las opciones de configuración para la auditoría
type SecurityAuditOptions struct {
	FullScan   bool
	DiffScan   bool
	DiffTarget string
	FailOn     string // "confirmed", "critical", "high", "medium"
}

// SecurityAuditor define el contrato del motor de auditoría adversarial (Cloudflare Harness)
type SecurityAuditor interface {
	Audit(ctx context.Context, projectDir string, opts SecurityAuditOptions, runner AgentRunner, agentOpts AgentOptions) (*domain.SecurityReport, error)
}
