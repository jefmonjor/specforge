// Package gates implements the REFACTOR quality gates. Every gate reads its
// tool's machine-readable report and compares it with a numeric threshold;
// a tool that is missing or crashes yields a Skipped result, never a pass.
//
// Tools are never downloaded on the fly: npx runs with --no-install, so a
// gate works offline and in CI exactly as on the developer's machine.
package gates

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"specforge/internal/adapters/process"
	"specforge/internal/domain/quality"
	"specforge/internal/domain/stack"
	"specforge/internal/ports"
)

// ForProfile returns the gates that apply to p, in execution order.
func ForProfile(p stack.Profile, proc ports.CommandRunner, t quality.Thresholds) []ports.Gate {
	var out []ports.Gate
	for _, g := range all(proc, t) {
		if g.Applies(p) {
			out = append(out, g)
		}
	}
	return out
}

func skipped(gate, why string) quality.Result {
	return quality.Result{Gate: gate, Status: quality.Skipped, Summary: why}
}

// outcome turns a process error into a Skipped result when the tool could
// not run. It returns a non-nil error only for cancellation.
func toolError(ctx context.Context, gate string, err error) (quality.Result, error) {
	if ctx.Err() != nil {
		return quality.Result{}, ctx.Err()
	}
	if errors.Is(err, ports.ErrToolNotFound) {
		return skipped(gate, "tool not installed: "+err.Error()), nil
	}
	return skipped(gate, "tool could not run: "+err.Error()), nil
}

func hasFile(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}

// npxLocal runs a Node tool that must be installed locally.
func npxLocal(ctx context.Context, proc ports.CommandRunner, root string, tool string, args ...string) (ports.CommandResult, error) {
	res, err := proc.Run(ctx, ports.Command{Name: "npx", Args: append([]string{"--no-install", tool}, args...), Dir: root})
	if err != nil {
		return res, err
	}
	low := strings.ToLower(res.Combined())
	if !res.Success() && (strings.Contains(low, "missing packages") || strings.Contains(low, "could not determine executable")) {
		return res, fmt.Errorf("%w: %s (add it to devDependencies)", ports.ErrToolNotFound, tool)
	}
	return res, nil
}

// globalOrLocal prefers a tool installed on PATH and falls back to the
// project's node_modules through npx.
func globalOrLocal(ctx context.Context, proc ports.CommandRunner, root, tool string, args ...string) (ports.CommandResult, error) {
	if process.Available(tool) {
		return proc.Run(ctx, ports.Command{Name: tool, Args: args, Dir: root})
	}
	if !hasFile(root, "package.json") {
		return ports.CommandResult{}, fmt.Errorf("%w: %s (npm install -g %s)", ports.ErrToolNotFound, tool, tool)
	}
	return npxLocal(ctx, proc, root, tool, args...)
}
