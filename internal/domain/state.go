package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"specforge/internal/buildinfo"
	"time"
)

// StatePhase define las fases dentro del ciclo SDD TDD / Loop
type StatePhase string

const (
	PhaseIdle      StatePhase = "IDLE"
	PhaseSpecify   StatePhase = "SPECIFY"
	PhasePlan      StatePhase = "PLAN"
	PhaseGate1     StatePhase = "GATE_1_BUSINESS_APPROVAL"
	PhaseGate2     StatePhase = "GATE_2_TECH_PLAN_APPROVAL"
	PhaseRedTest   StatePhase = "TDD_RED_FAILING_TESTS"
	PhaseGreenCode StatePhase = "TDD_GREEN_IMPLEMENTATION"
	PhaseRefactor  StatePhase = "TDD_REFACTOR_CLEAN_CODE"
	PhaseSonarGate StatePhase = "GATE_SONARQUBE"
	PhaseGate3     StatePhase = "GATE_3_FINAL_APPROVAL"
	PhaseCompleted StatePhase = "COMPLETED"
	PhaseFailed    StatePhase = "FAILED"
)

// TDDPhase define el estado estricto dentro del ciclo Red-Green-Refactor
type TDDPhase string

const (
	TDDPhaseRed       TDDPhase = "RED"
	TDDPhaseGreen     TDDPhase = "GREEN"
	TDDPhaseRefactor  TDDPhase = "REFACTOR"
	TDDPhaseCompleted TDDPhase = "COMPLETED"
)

// Checkpoint registra una sub-etapa superada para reanudación exacta
type Checkpoint struct {
	Timestamp time.Time  `json:"timestamp"`
	Phase     StatePhase `json:"phase"`
	Step      string     `json:"step"`
	Status    string     `json:"status"` // "SUCCESS", "PENDING", "FAILED"
	Details   string     `json:"details,omitempty"`
}

// LoopState representa el estado general persistido en .sdd-state.json
type LoopState struct {
	Version      string                 `json:"version"`
	UpdatedAt    time.Time              `json:"updated_at"`
	ProjectRoot  string                 `json:"project_root"`
	CurrentPhase StatePhase             `json:"current_phase"`
	CurrentStep  string                 `json:"current_step"`
	Intent       string                 `json:"intent"`
	SpecPath     string                 `json:"spec_path,omitempty"`
	PlanPath     string                 `json:"plan_path,omitempty"`
	Checkpoints  []Checkpoint           `json:"checkpoints"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TDDState representa el estado determinista del ciclo TDD con control criptográfico
type TDDState struct {
	Version         string       `json:"version"`
	UpdatedAt       time.Time    `json:"updated_at"`
	ProjectRoot     string       `json:"project_root"`
	SpecPath        string       `json:"spec_path"`
	SpecHash        string       `json:"spec_hash"`
	Scenarios       []Scenario   `json:"scenarios"`
	CurrentScenario int          `json:"current_scenario"` // Índice 0-based del escenario actual
	CurrentPhase    TDDPhase     `json:"current_phase"`
	RetryCount      int          `json:"retry_count"`
	LastError       string       `json:"last_error,omitempty"`
	Checkpoints     []Checkpoint `json:"checkpoints"`
}

// NewTDDState inicializa una nueva máquina de estados TDD
func NewTDDState(projectRoot, specPath, specHash string, scenarios []Scenario) *TDDState {
	return &TDDState{
		Version:         buildinfo.Version,
		UpdatedAt:       time.Now().UTC(),
		ProjectRoot:     projectRoot,
		SpecPath:        specPath,
		SpecHash:        specHash,
		Scenarios:       scenarios,
		CurrentScenario: 0,
		CurrentPhase:    TDDPhaseRed,
		RetryCount:      0,
		Checkpoints:     make([]Checkpoint, 0),
	}
}

// Save persiste atómicamente el estado en .sdd-state.json
func (s *TDDState) Save(dir string) error {
	filePath := filepath.Join(dir, ".sdd-state.json")
	s.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando TDDState: %w", err)
	}
	return os.WriteFile(filePath, data, 0644)
}

// LoadTDDState carga el estado de TDD desde .sdd-state.json
func LoadTDDState(dir string) (*TDDState, error) {
	filePath := filepath.Join(dir, ".sdd-state.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("no existe archivo de estado previo en %s: %w", filePath, err)
	}

	var state TDDState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("error deserializando %s: %w", filePath, err)
	}

	return &state, nil
}

// RecordStep añade un registro histórico de paso superado
func (s *TDDState) RecordStep(step, status, details string) {
	s.UpdatedAt = time.Now().UTC()
	s.Checkpoints = append(s.Checkpoints, Checkpoint{
		Timestamp: s.UpdatedAt,
		Phase:     StatePhase(s.CurrentPhase),
		Step:      step,
		Status:    status,
		Details:   details,
	})
}

// CurrentScenarioData devuelve el escenario BDD que se está ejecutando actualmente
func (s *TDDState) CurrentScenarioData() *Scenario {
	if s.CurrentScenario >= 0 && s.CurrentScenario < len(s.Scenarios) {
		return &s.Scenarios[s.CurrentScenario]
	}
	return nil
}

// AdvanceToNextPhase gestiona la transición de la máquina de estados Red -> Green -> Refactor -> Red...
func (s *TDDState) AdvanceToNextPhase() {
	s.RetryCount = 0
	s.LastError = ""

	switch s.CurrentPhase {
	case TDDPhaseRed:
		s.CurrentPhase = TDDPhaseGreen
	case TDDPhaseGreen:
		s.CurrentPhase = TDDPhaseRefactor
	case TDDPhaseRefactor:
		if s.CurrentScenario+1 < len(s.Scenarios) {
			s.CurrentScenario++
			s.CurrentPhase = TDDPhaseRed
		} else {
			s.CurrentPhase = TDDPhaseCompleted
		}
	}
}

// NewLoopState inicializa una nueva máquina de estados general (compatibilidad)
func NewLoopState(projectRoot, intent string) *LoopState {
	return &LoopState{
		Version:      buildinfo.Version,
		UpdatedAt:    time.Now().UTC(),
		ProjectRoot:  projectRoot,
		CurrentPhase: PhaseIdle,
		CurrentStep:  "init",
		Intent:       intent,
		Checkpoints:  make([]Checkpoint, 0),
		Metadata:     make(map[string]interface{}),
	}
}

func (s *LoopState) RecordCheckpoint(phase StatePhase, step, status, details string) {
	s.CurrentPhase = phase
	s.CurrentStep = step
	s.UpdatedAt = time.Now().UTC()
	s.Checkpoints = append(s.Checkpoints, Checkpoint{
		Timestamp: s.UpdatedAt,
		Phase:     phase,
		Step:      step,
		Status:    status,
		Details:   details,
	})
}

func (s *LoopState) Save(dir string) error {
	filePath := filepath.Join(dir, ".sdd-state.json")
	s.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando estado: %w", err)
	}
	return os.WriteFile(filePath, data, 0644)
}

func LoadState(dir string) (*LoopState, error) {
	filePath := filepath.Join(dir, ".sdd-state.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var state LoopState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("error deserializando %s: %w", filePath, err)
	}
	return &state, nil
}
