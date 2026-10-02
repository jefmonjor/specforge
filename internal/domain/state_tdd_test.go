package domain

import (
	"os"
	"testing"
)

func TestTDDStateTransitions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-tdd-state-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	scenarios := []Scenario{
		{Index: 1, Title: "Escenario 1"},
		{Index: 2, Title: "Escenario 2"},
	}

	state := NewTDDState(tmpDir, "specs/0001.md", "hash123", scenarios)

	// 1. Fase inicial RED
	if state.CurrentPhase != TDDPhaseRed {
		t.Errorf("fase inicial esperada RED, obtenida: %s", state.CurrentPhase)
	}
	if state.CurrentScenario != 0 {
		t.Errorf("escenario inicial esperado 0, obtenido: %d", state.CurrentScenario)
	}

	// 2. RED -> GREEN
	state.AdvanceToNextPhase()
	if state.CurrentPhase != TDDPhaseGreen {
		t.Errorf("fase esperada GREEN, obtenida: %s", state.CurrentPhase)
	}

	// 3. GREEN -> REFACTOR
	state.AdvanceToNextPhase()
	if state.CurrentPhase != TDDPhaseRefactor {
		t.Errorf("fase esperada REFACTOR, obtenida: %s", state.CurrentPhase)
	}

	// 4. REFACTOR -> RED (Siguiente escenario 2)
	state.AdvanceToNextPhase()
	if state.CurrentPhase != TDDPhaseRed {
		t.Errorf("fase esperada RED para escenario 2, obtenida: %s", state.CurrentPhase)
	}
	if state.CurrentScenario != 1 {
		t.Errorf("escenario actual esperado 1, obtenido: %d", state.CurrentScenario)
	}

	// 5. Completar escenario 2: RED -> GREEN -> REFACTOR -> COMPLETED
	state.AdvanceToNextPhase() // GREEN
	state.AdvanceToNextPhase() // REFACTOR
	state.AdvanceToNextPhase() // COMPLETED

	if state.CurrentPhase != TDDPhaseCompleted {
		t.Errorf("fase final esperada COMPLETED, obtenida: %s", state.CurrentPhase)
	}

	// 6. Test Guardado y Carga
	if err := state.Save(tmpDir); err != nil {
		t.Fatalf("error guardando TDDState: %v", err)
	}

	loaded, err := LoadTDDState(tmpDir)
	if err != nil {
		t.Fatalf("error cargando TDDState: %v", err)
	}

	if loaded.SpecHash != "hash123" {
		t.Errorf("SpecHash esperado 'hash123', obtenido: %s", loaded.SpecHash)
	}
	if loaded.CurrentPhase != TDDPhaseCompleted {
		t.Errorf("CurrentPhase esperada COMPLETED, obtenida: %s", loaded.CurrentPhase)
	}
}
