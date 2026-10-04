package domain

import (
	"os"
	"testing"
)

func TestLoopStateSaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-state-test-*")
	if err != nil {
		t.Fatalf("error creando directorio temporal: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	state := NewLoopState(tmpDir, "implementar calculo de intereses")
	state.RecordCheckpoint(PhaseSpecify, "spec_creation", "SUCCESS", "spec creada")
	state.RecordCheckpoint(PhaseRedTest, "failing_tests", "SUCCESS", "tests en rojo listos")

	if err := state.Save(tmpDir); err != nil {
		t.Fatalf("error guardando estado: %v", err)
	}

	loaded, err := LoadState(tmpDir)
	if err != nil {
		t.Fatalf("error cargando estado: %v", err)
	}

	if loaded.CurrentPhase != PhaseRedTest {
		t.Errorf("fase esperada %s, obtenida: %s", PhaseRedTest, loaded.CurrentPhase)
	}
	if loaded.CurrentStep != "failing_tests" {
		t.Errorf("paso esperado 'failing_tests', obtenido: %s", loaded.CurrentStep)
	}
	if len(loaded.Checkpoints) != 2 {
		t.Errorf("se esperaban 2 checkpoints, obtenidos: %d", len(loaded.Checkpoints))
	}
}
