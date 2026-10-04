package compiler

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"specforge/internal/adapters/storage"
	"specforge/internal/domain"
	"specforge/internal/ports"
)

// DispatcherCompiler implementa ports.Compiler seleccionando las herramientas adecuadas por stack
type DispatcherCompiler struct{}

func NewDispatcherCompiler() ports.Compiler {
	return &DispatcherCompiler{}
}

func (c *DispatcherCompiler) Build(ctx context.Context, project domain.ProjectInfo) (ports.BuildResult, error) {
	cmdName, args := resolveBuildCommand(project)
	return runProcessWithPipes(ctx, project.RootPath, cmdName, args...)
}

func (c *DispatcherCompiler) Test(ctx context.Context, project domain.ProjectInfo) (ports.BuildResult, error) {
	cmdName, args := resolveTestCommand(project, "")
	return runProcessWithPipes(ctx, project.RootPath, cmdName, args...)
}

func (c *DispatcherCompiler) RunTests(ctx context.Context, project domain.ProjectInfo, scenario string) (bool, string, error) {
	cmdName, args := resolveTestCommand(project, scenario)
	res, err := runProcessWithPipes(ctx, project.RootPath, cmdName, args...)

	logger := storage.GetLogger()
	logger.Debug("Test execution: cmd=%s args=%v success=%v exitCode=%d", cmdName, args, res.Success, res.ExitCode)

	combinedOutput := res.Output
	if res.ErrorOut != "" {
		combinedOutput += "\n--- STDERR ---\n" + res.ErrorOut
	}

	return res.Success, combinedOutput, err
}

func (c *DispatcherCompiler) GenerateTestStubs(ctx context.Context, project domain.ProjectInfo, scenario domain.Scenario, runner ports.AgentRunner, opts ports.AgentOptions) error {
	logger := storage.GetLogger()

	var prompt strings.Builder
	prompt.WriteString("MISIÓN: TDD FASE RED (GENERAR TEST QUE DEBE FALLAR)\n\n")
	prompt.WriteString(fmt.Sprintf("Stack del proyecto: %s\n", project.Type))
	prompt.WriteString("Regla de Oro: Solo genera el archivo de prueba unitario/integración que valida el siguiente escenario BDD.\n")
	prompt.WriteString("PROHIBIDO implementar la lógica de negocio productiva en este paso (YAGNI).\n")
	prompt.WriteString("El test DEBE compilar pero FALLAR al ejecutarse (por ausencia del comportamiento esperado).\n\n")

	prompt.WriteString(fmt.Sprintf("ESCENARIO BDD:\n%s\n\n", scenario.RawContent))
	prompt.WriteString("Crea o actualiza el archivo de test correspondiente en el directorio de pruebas estándar del proyecto.\n")

	logger.Info("Solicitando a la IA generación de test RED para escenario '%s'", scenario.Title)
	_, err := runner.RunHeadless(ctx, prompt.String(), opts)
	return err
}

func resolveBuildCommand(project domain.ProjectInfo) (string, []string) {
	switch project.Type {
	case domain.ProjectJavaSpring, domain.ProjectJavaLegacy:
		return winCmd("mvn"), []string{"-B", "clean", "compile", "test-compile"}
	case domain.ProjectReact, domain.ProjectAngular, domain.ProjectNode:
		return winCmd("npm"), []string{"run", "build", "--if-present"}
	case domain.ProjectGo:
		return "go", []string{"build", "./..."}
	case domain.ProjectPython:
		return "python", []string{"-m", "py_compile"}
	default:
		return "echo", []string{"build no configurado"}
	}
}

func resolveTestCommand(project domain.ProjectInfo, scenario string) (string, []string) {
	switch project.Type {
	case domain.ProjectJavaSpring, domain.ProjectJavaLegacy:
		return winCmd("mvn"), []string{"-B", "test"}
	case domain.ProjectReact, domain.ProjectAngular, domain.ProjectNode:
		return winCmd("npm"), []string{"test", "--", "--run"}
	case domain.ProjectGo:
		return "go", []string{"test", "-v", "./..."}
	case domain.ProjectPython:
		return "pytest", []string{"-q"}
	default:
		return "echo", []string{"tests no soportados"}
	}
}

func winCmd(cmd string) string {
	if runtime.GOOS == "windows" {
		if path, err := exec.LookPath(cmd + ".cmd"); err == nil {
			return path
		}
		if path, err := exec.LookPath(cmd + ".exe"); err == nil {
			return path
		}
		if path, err := exec.LookPath(cmd + ".bat"); err == nil {
			return path
		}
	}
	return cmd
}

func runProcessWithPipes(ctx context.Context, workDir, cmdName string, args ...string) (ports.BuildResult, error) {
	logger := storage.GetLogger()

	cmd := exec.CommandContext(ctx, cmdName, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	exitCode := 0
	success := true
	if err != nil {
		success = false
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	if stderrStr != "" {
		logger.LogIO("PROCESS", "STDERR", stderrStr)
	}

	return ports.BuildResult{
		Success:  success,
		ExitCode: exitCode,
		Output:   stdoutStr,
		ErrorOut: stderrStr,
	}, err
}
