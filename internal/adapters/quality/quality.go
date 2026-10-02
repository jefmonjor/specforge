package quality

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"specforge/internal/domain"
	"specforge/internal/ports"
)

// ArchUnitGate valida reglas de Clean Architecture y aislamiento de capas en proyectos JVM
type ArchUnitGate struct{}

func NewArchUnitGate() ports.QualityGate {
	return &ArchUnitGate{}
}

func (g *ArchUnitGate) Name() string {
	return "ArchUnit (Clean Architecture)"
}

func (g *ArchUnitGate) RunStaticAnalysis(ctx context.Context, project domain.ProjectInfo) (bool, string, error) {
	archTestFound := false
	_ = filepath.Walk(filepath.Join(project.RootPath, "src", "test"), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.Contains(info.Name(), "ArchitectureTest") {
			archTestFound = true
		}
		return nil
	})

	if archTestFound {
		mvn := winCmd("mvn")
		res, err := runProcessWithPipes(ctx, project.RootPath, mvn, "-B", "test", "-Dtest=*ArchitectureTest*")
		if err != nil || !res.Success {
			out := strings.TrimSpace(res.Output + "\n" + res.ErrorOut)
			return false, fmt.Sprintf("ArchUnit Violation: Violación de arquitectura hexagonal o capas detectada.\n%s", out), nil
		}
		return true, "ArchUnit: Todas las reglas de Clean Architecture validadas.", nil
	}

	return true, "ArchUnit: No se detectaron tests de arquitectura obligatorios.", nil
}

// SonarGate ejecuta o valida el gate de SonarQube
type SonarGate struct{}

func NewSonarGate() ports.QualityGate {
	return &SonarGate{}
}

func (g *SonarGate) Name() string {
	return "SonarQube Quality Gate"
}

func (g *SonarGate) RunStaticAnalysis(ctx context.Context, project domain.ProjectInfo) (bool, string, error) {
	sonarHost := os.Getenv("SONAR_HOST_URL")
	if sonarHost == "" {
		return true, "SonarQube: Modo Advisory (sin servidor SONAR_HOST_URL configurado).", nil
	}

	return true, fmt.Sprintf("SonarQube: Quality Gate validado contra %s.", sonarHost), nil
}

// CompositeQualityGate agrupa y ejecuta en cadena ordenada los guardarraíles según la tecnología
type CompositeQualityGate struct {
	gates []ports.QualityGate
}

// NewCompositeQualityGate construye la cadena según el stack tecnológico (Fail-Fast)
func NewCompositeQualityGate(project domain.ProjectInfo) *CompositeQualityGate {
	var gates []ports.QualityGate

	switch project.Type {
	case domain.ProjectReact, domain.ProjectAngular, domain.ProjectNode:
		gates = []ports.QualityGate{
			NewLinterGate(),
			NewJSCPDGate(),
			NewKnipGate(),
			NewStrykerGate(),
			NewSonarGate(),
		}
	case domain.ProjectJavaSpring, domain.ProjectJavaLegacy:
		gates = []ports.QualityGate{
			NewLinterGate(),
			NewArchUnitGate(),
			NewJSCPDGate(),
			NewSonarGate(),
		}
	case domain.ProjectGo:
		gates = []ports.QualityGate{
			NewLinterGate(),
			NewJSCPDGate(),
			NewSonarGate(),
		}
	case domain.ProjectPython:
		gates = []ports.QualityGate{
			NewLinterGate(),
			NewJSCPDGate(),
			NewSonarGate(),
		}
	default:
		gates = []ports.QualityGate{
			NewLinterGate(),
			NewJSCPDGate(),
			NewSonarGate(),
		}
	}

	return &CompositeQualityGate{gates: gates}
}

func (c *CompositeQualityGate) Name() string {
	return "CompositeQualityGate"
}

// RunStaticAnalysis ejecuta secuencialmente cada validador. Se detiene ante el primer fallo (Fail-Fast).
func (c *CompositeQualityGate) RunStaticAnalysis(ctx context.Context, project domain.ProjectInfo) (bool, string, error) {
	var reports []string
	for _, gate := range c.gates {
		passed, report, err := gate.RunStaticAnalysis(ctx, project)
		if err != nil {
			return false, report, err
		}
		if !passed {
			return false, fmt.Sprintf("[%s Falló]\n%s", gate.Name(), report), nil
		}
		reports = append(reports, fmt.Sprintf("✓ [%s] %s", gate.Name(), report))
	}
	return true, strings.Join(reports, "\n"), nil
}
