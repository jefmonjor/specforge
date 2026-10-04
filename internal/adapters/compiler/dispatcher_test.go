package compiler

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"specforge/internal/domain"
)

func TestResolveCommands(t *testing.T) {
	cases := []struct {
		projType         domain.ProjectType
		expectedBuildCmd string
		expectedTestCmd  string
	}{
		{domain.ProjectJavaSpring, "mvn", "mvn"},
		{domain.ProjectReact, "npm", "npm"},
		{domain.ProjectGo, "go", "go"},
		{domain.ProjectPython, "python", "pytest"},
	}

	for _, c := range cases {
		p := domain.ProjectInfo{Type: c.projType}
		bCmd, _ := resolveBuildCommand(p)
		tCmd, _ := resolveTestCommand(p, "")

		if !strings.Contains(strings.ToLower(bCmd), c.expectedBuildCmd) {
			t.Errorf("para %s build esperado %s, obtenido: %s", c.projType, c.expectedBuildCmd, bCmd)
		}
		if !strings.Contains(strings.ToLower(tCmd), c.expectedTestCmd) {
			t.Errorf("para %s test esperado %s, obtenido: %s", c.projType, c.expectedTestCmd, tCmd)
		}
	}
}

func TestRunProcessWithPipes(t *testing.T) {
	ctx := context.Background()
	cmdName := "echo"
	args := []string{"sdd-framework-test"}
	if runtime.GOOS == "windows" {
		cmdName = "cmd"
		args = []string{"/c", "echo", "sdd-framework-test"}
	}
	res, err := runProcessWithPipes(ctx, "", cmdName, args...)
	if err != nil {
		t.Fatalf("error ejecutando proceso: %v", err)
	}

	if !res.Success {
		t.Errorf("se esperaba éxito")
	}
	if !strings.Contains(res.Output, "sdd-framework-test") {
		t.Errorf("salida inesperada: %s", res.Output)
	}
}
