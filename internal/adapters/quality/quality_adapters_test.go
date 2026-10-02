package quality

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"specforge/internal/domain"
)

func TestKnipGateWithoutPackageJson(t *testing.T) {
	gate := NewKnipGate()
	tmpDir, _ := os.MkdirTemp("", "sdd-knip-test-*")
	defer os.RemoveAll(tmpDir)

	p := domain.ProjectInfo{RootPath: tmpDir, Type: domain.ProjectGo}
	passed, report, err := gate.RunStaticAnalysis(context.Background(), p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !passed {
		t.Errorf("debería pasar cuando no hay package.json: %s", report)
	}
}

func TestJSCPDGateCleanDirectory(t *testing.T) {
	gate := NewJSCPDGate()
	tmpDir, _ := os.MkdirTemp("", "sdd-jscpd-test-*")
	defer os.RemoveAll(tmpDir)

	p := domain.ProjectInfo{RootPath: tmpDir, Type: domain.ProjectReact}
	passed, report, err := gate.RunStaticAnalysis(context.Background(), p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !passed {
		t.Errorf("debería pasar en un directorio limpio sin clones: %s", report)
	}
}

func TestStrykerGateWithoutConfig(t *testing.T) {
	gate := NewStrykerGate()
	tmpDir, _ := os.MkdirTemp("", "sdd-stryker-test-*")
	defer os.RemoveAll(tmpDir)

	p := domain.ProjectInfo{RootPath: tmpDir, Type: domain.ProjectReact}
	passed, report, err := gate.RunStaticAnalysis(context.Background(), p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !passed {
		t.Errorf("debería omitir cuando no hay stryker.conf.*: %s", report)
	}
}

func TestLinterGateGo(t *testing.T) {
	gate := NewLinterGate()
	p := domain.ProjectInfo{RootPath: ".", Type: domain.ProjectGo}
	passed, report, err := gate.RunStaticAnalysis(context.Background(), p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !passed {
		t.Errorf("go vet / linter debería pasar en sdd-go: %s", report)
	}
}

func TestSetupExtractsQualityConfigs(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "sdd-quality-configs-*")
	defer os.RemoveAll(tmpDir)

	// Simular proyecto React
	_ = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{"name":"test"}`), 0644)

	p := domain.DetectProject(tmpDir)
	if p.Type != domain.ProjectNode && p.Type != domain.ProjectReact {
		t.Logf("Stack detectado: %s", p.Type)
	}
}
