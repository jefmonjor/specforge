package quality

import (
	"testing"

	"specforge/internal/domain"
)

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
