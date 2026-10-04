package quality

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"specforge/internal/adapters/storage"
	"specforge/internal/ports"
)

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
		logger.LogIO("QUALITY_PROCESS", "STDERR", stderrStr)
	}

	return ports.BuildResult{
		Success:  success,
		ExitCode: exitCode,
		Output:   stdoutStr,
		ErrorOut: stderrStr,
	}, err
}
