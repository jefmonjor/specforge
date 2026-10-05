package testrun

import (
	"context"
	"strings"

	"github.com/jefmonjor/specforge/v6/internal/adapters/process"
	"github.com/jefmonjor/specforge/v6/internal/domain/stack"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

var _ ports.Readier = (*Runner)(nil)

// Ready implements ports.Readier: whether the stack's test runner is
// installed, preferring the project's wrapper or virtual environment as
// Run does.
func (r *Runner) Ready(_ context.Context, root string, p stack.Profile) ports.Readiness {
	switch p.Runner {
	case stack.RunnerGo:
		return onPath("go", "install Go: https://go.dev/dl")
	case stack.RunnerMaven:
		return wrapperOrPath(root, "mvnw", "mvn", "install Maven (https://maven.apache.org) or add the wrapper: mvn wrapper:wrapper")
	case stack.RunnerGradle:
		return wrapperOrPath(root, "gradlew", "gradle", "install Gradle (https://gradle.org) or add the wrapper: gradle wrapper")
	case stack.RunnerVitest:
		return local(root, "vitest", "npm install --save-dev vitest")
	case stack.RunnerJest:
		return local(root, "jest", "npm install --save-dev jest")
	case stack.RunnerNPM:
		return onPath("npm", "install Node.js: https://nodejs.org")
	case stack.RunnerPytest:
		if c := pythonCandidates(root); len(c) > 0 {
			return ports.Readiness{Ready: true, Detail: strings.Join(c[0], " ")}
		}
		return ports.Readiness{Detail: "neither pytest nor python found", Hint: "python3 -m venv .venv && .venv/bin/pip install pytest"}
	default:
		return ports.Readiness{Detail: "no test runner for " + p.Name(), Hint: "pass --stack or set stack: in specforge.yaml"}
	}
}

func onPath(tool, hint string) ports.Readiness {
	if path, err := process.Resolve(tool); err == nil {
		return ports.Readiness{Ready: true, Detail: path}
	}
	return ports.Readiness{Detail: tool + " not found on PATH", Hint: hint}
}

func wrapperOrPath(root, wrapper, tool, hint string) ports.Readiness {
	if bin := wrapperOr(root, wrapper, tool); bin != tool {
		return ports.Readiness{Ready: true, Detail: bin}
	}
	return onPath(tool, hint)
}

func local(root, tool, hint string) ports.Readiness {
	if p, ok := process.NodeBin(root, tool); ok {
		return ports.Readiness{Ready: true, Detail: p}
	}
	return ports.Readiness{Detail: tool + " is not installed in the project", Hint: hint + " (then npm install)"}
}
