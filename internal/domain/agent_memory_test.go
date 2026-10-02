package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAgentContext(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-mem-test-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	agentDir := filepath.Join(tmpDir, ".sdd", "agent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("error creando agent dir: %v", err)
	}

	_ = os.WriteFile(filepath.Join(agentDir, "agente.md"), []byte("Eres un Software Crafter Senior."), 0644)
	_ = os.WriteFile(filepath.Join(agentDir, "persona.md"), []byte("Tono directo y conciso."), 0644)
	_ = os.WriteFile(filepath.Join(agentDir, "ng-rules.md"), []byte("1. NUNCA uses any."), 0644)
	_ = os.WriteFile(filepath.Join(agentDir, "glossary.md"), []byte("SDD: Spec-Driven Development"), 0644)
	_ = os.WriteFile(filepath.Join(agentDir, "references.md"), []byte("Golden Master de Value Object"), 0644)
	_ = os.WriteFile(filepath.Join(agentDir, "lessons.md"), []byte("- [2026-10-02]: Stryker mutation superado."), 0644)

	ctxMem, err := LoadAgentContext(tmpDir)
	if err != nil {
		t.Fatalf("error cargando memoria: %v", err)
	}

	if !strings.Contains(ctxMem, "CEREBRO CONTEXTUAL Y MEMORIA EXPLÍCITA") {
		t.Errorf("no contiene cabecera principal")
	}
	if !strings.Contains(ctxMem, "Software Crafter Senior") {
		t.Errorf("no contiene agente.md")
	}
	if !strings.Contains(ctxMem, "NUNCA uses any") {
		t.Errorf("no contiene ng-rules.md")
	}
	if !strings.Contains(ctxMem, "Stryker mutation superado") {
		t.Errorf("no contiene lessons.md")
	}
}

func TestRecordLesson(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sdd-record-test-*")
	if err != nil {
		t.Fatalf("error creando temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = RecordLesson(tmpDir, "Linter", "Variable no utilizada x", "Eliminar variable huérfana")
	if err != nil {
		t.Fatalf("error registrando lección: %v", err)
	}

	lessonsPath := filepath.Join(tmpDir, ".sdd", "agent", "lessons.md")
	data, err := os.ReadFile(lessonsPath)
	if err != nil {
		t.Fatalf("no se creó lessons.md: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "[Linter]") || !strings.Contains(content, "Variable no utilizada x") {
		t.Errorf("lessons.md no contiene la lección esperada: %s", content)
	}
}
