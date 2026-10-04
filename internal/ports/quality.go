package ports

import (
	"context"
	"specforge/internal/domain"
)

// QualityGate define el contrato para verificaciones estáticas (SonarQube, ArchUnit, Linters)
type QualityGate interface {
	Name() string
	RunStaticAnalysis(ctx context.Context, project domain.ProjectInfo) (passed bool, report string, err error)
}
