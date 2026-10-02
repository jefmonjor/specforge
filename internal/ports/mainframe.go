package ports

import "context"

// MainframeConfigurator define las operaciones de configuración y análisis para entornos IBM i / AS400
type MainframeConfigurator interface {
	ConfigureVSCode(projectDir string) error
	RunCobolSonarBatch(ctx context.Context, projectDir string) error
}
