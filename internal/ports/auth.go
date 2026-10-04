package ports

import (
	"context"
	"specforge/internal/domain"
)

// ADCDetector define el contrato para la detección transparente de credenciales de Google Cloud (Zero-Keys)
type ADCDetector interface {
	DetectCredentials(ctx context.Context) (*domain.AuthConfig, error)
	GetAccessToken(ctx context.Context) (string, error)
}
