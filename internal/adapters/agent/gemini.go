package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"specforge/internal/adapters/storage"
	"specforge/internal/domain"
	"specforge/internal/ports"
)

type GeminiAgentRunner struct{}

func NewGeminiAgentRunner() ports.AgentRunner {
	return &GeminiAgentRunner{}
}

func (g *GeminiAgentRunner) Name() string {
	return "gemini"
}

func (g *GeminiAgentRunner) RunHeadless(ctx context.Context, prompt string, opts ports.AgentOptions) (string, error) {
	logger := storage.GetLogger()

	// Inyectar Cerebro Contextual (Memoria Explícita) si existe en el proyecto
	effectivePrompt := prompt
	if opts.WorkingDir != "" {
		if ctxMem, err := domain.LoadAgentContext(opts.WorkingDir); err == nil && ctxMem != "" {
			effectivePrompt = fmt.Sprintf("<system_instruction>\n%s\n</system_instruction>\n\n%s", ctxMem, prompt)
		}
	}

	logger.LogIO("INPUT", "PROMPT", effectivePrompt)

	args := []string{"-p", effectivePrompt}
	if opts.Model != "" {
		args = append(args, "-m", opts.Model)
	}

	cmd := exec.CommandContext(ctx, "gemini", args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	cmd.Env = os.Environ()
	if opts.Project != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOOGLE_CLOUD_PROJECT=%s", opts.Project))
	}
	if opts.Location != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOOGLE_CLOUD_LOCATION=%s", opts.Location))
	}
	cmd.Env = append(cmd.Env, "GOOGLE_GENAI_USE_VERTEXAI=true")

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Spinner animado en tiempo real con segundero
	done := make(chan struct{})
	go func() {
		spinChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		idx := 0
		start := time.Now()
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				fmt.Print("\r                                                                               \r")
				return
			case <-ticker.C:
				elapsed := int(time.Since(start).Seconds())
				fmt.Printf("\r  %s [Gemini IA: %ds] Procesando con Vertex AI (no cancelar con Ctrl+C)...",
					spinChars[idx%len(spinChars)], elapsed)
				idx++
			}
		}
	}()

	err := cmd.Run()
	close(done)
	time.Sleep(50 * time.Millisecond)

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	logger.LogIO("OUTPUT", "STDOUT", stdoutStr)
	if stderrStr != "" {
		logger.LogIO("OUTPUT", "STDERR", stderrStr)
	}

	if err != nil {
		return "", fmt.Errorf("gemini execution failed: %w (stderr: %s)", err, strings.TrimSpace(stderrStr))
	}

	fmt.Printf("  ✓ Respuesta generada por la IA con éxito.\n")
	return stdoutStr, nil
}

func (g *GeminiAgentRunner) RunInteractive(ctx context.Context, initialPrompt string, opts ports.AgentOptions) error {
	fmt.Println("  ⏳ Conectando sesión interactiva con el agente (puede demorar unos segundos)...")
	fmt.Println("  ℹ️  Una vez conectado, escribe tus respuestas directamente en la consola.")

	effectivePrompt := initialPrompt
	if opts.WorkingDir != "" {
		if ctxMem, err := domain.LoadAgentContext(opts.WorkingDir); err == nil && ctxMem != "" {
			effectivePrompt = fmt.Sprintf("<system_instruction>\n%s\n</system_instruction>\n\n%s", ctxMem, initialPrompt)
		}
	}

	args := []string{}
	if effectivePrompt != "" {
		args = append(args, "-i", effectivePrompt)
	}
	if opts.Model != "" {
		args = append(args, "-m", opts.Model)
	}

	cmd := exec.CommandContext(ctx, "gemini", args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
