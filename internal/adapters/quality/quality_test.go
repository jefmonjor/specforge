package quality

import (
	"context"
	"testing"

	"specforge/internal/domain"
)

func TestCompositeQualityGateGo(t *testing.T) {
	p := domain.ProjectInfo{
		Type:     domain.ProjectGo,
		RootPath: ".",
	}
	gate := NewCompositeQualityGate(p)
	ctx := context.Background()

	passed, report, err := gate.RunStaticAnalysis(ctx, p)
	if err != nil {
		t.Fatalf("error en Quality Gate: %v", err)
	}

	if !passed {
		t.Errorf("se esperaba que pasara para stack Go en entorno actual: %s", report)
	}
}

func TestCompositeQualityGateComposition(t *testing.T) {
	reactProject := domain.ProjectInfo{Type: domain.ProjectReact}
	reactGate := NewCompositeQualityGate(reactProject)
	if len(reactGate.gates) != 5 {
		t.Errorf("se esperaban 5 guardarraíles para React, obtenidos: %d", len(reactGate.gates))
	}

	javaProject := domain.ProjectInfo{Type: domain.ProjectJavaSpring}
	javaGate := NewCompositeQualityGate(javaProject)
	if len(javaGate.gates) != 4 {
		t.Errorf("se esperaban 4 guardarraíles para Java, obtenidos: %d", len(javaGate.gates))
	}

	goProject := domain.ProjectInfo{Type: domain.ProjectGo}
	goGate := NewCompositeQualityGate(goProject)
	if len(goGate.gates) != 3 {
		t.Errorf("se esperaban 3 guardarraíles para Go, obtenidos: %d", len(goGate.gates))
	}
}
