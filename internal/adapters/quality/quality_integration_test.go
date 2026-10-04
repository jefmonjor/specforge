//go:build integration

package quality

import (
	"context"
	"os"
	"testing"

	"specforge/internal/domain"
)

// These tests shell out to real tools (golangci-lint, go vet, npx jscpd) and
// depend on the host. Run them with: go test -tags integration ./...

func TestCompositeQualityGateGo(t *testing.T) {
	p := domain.ProjectInfo{Type: domain.ProjectGo, RootPath: "."}
	gate := NewCompositeQualityGate(p)

	passed, report, err := gate.RunStaticAnalysis(context.Background(), p)
	if err != nil {
		t.Fatalf("quality gate error: %v", err)
	}
	if !passed {
		t.Errorf("expected the Go gate chain to pass on this repo: %s", report)
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
		t.Errorf("go vet / linter should pass on this repo: %s", report)
	}
}

func TestJSCPDGateCleanDirectory(t *testing.T) {
	gate := NewJSCPDGate()
	tmpDir := t.TempDir()
	_ = os.WriteFile(tmpDir+"/package.json", []byte(`{"name":"x"}`), 0o644)

	p := domain.ProjectInfo{RootPath: tmpDir, Type: domain.ProjectReact}
	passed, report, err := gate.RunStaticAnalysis(context.Background(), p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !passed {
		t.Errorf("a clean directory must have no clones: %s", report)
	}
}
