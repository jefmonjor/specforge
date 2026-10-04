package ports

import (
	"context"
	"specforge/internal/domain"
)

// E2EOptions parametriza la sesión de ejecución visual autónoma
type E2EOptions struct {
	Headless       bool
	MaxSteps       int
	ScreenshotPath string
	Agent          string
}

// E2EEngine define el contrato para el motor de pruebas E2E autónomo
type E2EEngine interface {
	RunVisualSpec(ctx context.Context, spec *domain.Spec, targetURL string, opts E2EOptions) error
}
