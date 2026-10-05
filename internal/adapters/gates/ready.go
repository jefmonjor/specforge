package gates

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"specforge/internal/adapters/process"
	"specforge/internal/domain/legacy"
	"specforge/internal/domain/quality"
	"specforge/internal/domain/stack"
	"specforge/internal/ports"
)

// gate is what every gate here implements: the check and its readiness.
type gate interface {
	ports.Gate
	ports.Readier
}

func all(proc ports.CommandRunner, t quality.Thresholds) []gate {
	return []gate{
		// The migration gate is added by the caller, with its target.
		&Lint{proc: proc},
		&Duplication{proc: proc, max: t.MaxDuplicationPercent},
		&DeadCode{proc: proc},
		&Mutation{proc: proc, min: t.MinMutationScore},
	}
}

// ProbesFor returns the gates of a profile as probes for `doctor`, the
// migration gate of a rewrite included.
func ProbesFor(p stack.Profile, proc ports.CommandRunner, t quality.Thresholds, target legacy.Target) []ports.Probe {
	var out []ports.Probe
	for _, g := range append(all(proc, t), &Migration{Target: target}) {
		if g.Applies(p) {
			out = append(out, g)
		}
	}
	return out
}

func ready(detail string) ports.Readiness { return ports.Readiness{Ready: true, Detail: detail} }

func missing(detail, hint string) ports.Readiness {
	return ports.Readiness{Detail: detail, Hint: hint}
}

// Ready implements ports.Readier.
func (l *Lint) Ready(_ context.Context, root string, p stack.Profile) ports.Readiness {
	switch p.Kind {
	case stack.Go:
		if process.Available("golangci-lint") {
			return ready("golangci-lint")
		}
		if process.Available("go") {
			return ports.Readiness{Ready: true, Detail: "go vet (golangci-lint not found)", Hint: "install golangci-lint for the full lint: https://golangci-lint.run/welcome/install/"}
		}
		return missing("neither golangci-lint nor go found", "install Go: https://go.dev/dl")
	case stack.Node:
		if hasScript(root, "lint") {
			return ready(`npm run lint`)
		}
		return missing(`package.json has no "lint" script`, `add "lint": "eslint ." to the scripts of package.json`)
	case stack.Python:
		if bin, ok := process.VenvBin(root, "ruff"); ok {
			return ready(bin)
		}
		if process.Available("ruff") {
			return ready("ruff")
		}
		return missing("ruff not found", "install it in the project: .venv/bin/pip install ruff")
	case stack.Maven:
		if pom, err := os.ReadFile(filepath.Join(root, "pom.xml")); err == nil && strings.Contains(string(pom), "maven-pmd-plugin") {
			return ready("PMD (maven-pmd-plugin)")
		}
		return missing("no linter in the build", "add maven-pmd-plugin to pom.xml (the Java scaffold has it)")
	default:
		return missing("no linter for "+string(p.Kind), "add Checkstyle or PMD to the build")
	}
}

// Ready implements ports.Readier.
func (*Duplication) Ready(_ context.Context, root string, _ stack.Profile) ports.Readiness {
	if process.Available("jscpd") {
		return ready("jscpd")
	}
	if bin, ok := process.NodeBin(root, "jscpd"); ok {
		return ready(bin)
	}
	return missing("jscpd not found", "npm install -g jscpd (or add it to devDependencies)")
}

// Ready implements ports.Readier.
func (*DeadCode) Ready(_ context.Context, root string, _ stack.Profile) ports.Readiness {
	if bin, ok := process.NodeBin(root, "knip"); ok {
		return ready(bin)
	}
	return missing("knip not installed in the project", "npm install --save-dev knip")
}

// Ready implements ports.Readier.
func (*Mutation) Ready(_ context.Context, root string, _ stack.Profile) ports.Readiness {
	configured := false
	for _, c := range strykerConfigs {
		configured = configured || hasFile(root, c)
	}
	if !configured {
		return missing("Stryker is not configured (no stryker.conf.*)", "npm init stryker")
	}
	if bin, ok := process.NodeBin(root, "stryker"); ok {
		return ready(bin)
	}
	return missing("Stryker is configured but not installed", "npm install --save-dev @stryker-mutator/core")
}

// Ready implements ports.Readier: the migration gate needs no tool.
func (*Migration) Ready(context.Context, string, stack.Profile) ports.Readiness {
	return ready("built in")
}
