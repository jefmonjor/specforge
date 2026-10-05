package gates

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jefmonjor/specforge/v6/internal/adapters/process"
	"github.com/jefmonjor/specforge/v6/internal/domain/quality"
	"github.com/jefmonjor/specforge/v6/internal/domain/stack"
	"github.com/jefmonjor/specforge/v6/internal/ports"
)

// Lint runs the stack's linter.
type Lint struct{ proc ports.CommandRunner }

// Name implements ports.Gate.
func (*Lint) Name() string { return "lint" }

// Applies implements ports.Gate.
func (*Lint) Applies(stack.Profile) bool { return true }

// Check implements ports.Gate.
func (l *Lint) Check(ctx context.Context, root string, p stack.Profile) (quality.Result, error) {
	switch p.Kind {
	case stack.Go:
		return l.golang(ctx, root)
	case stack.Node:
		return l.node(ctx, root)
	case stack.Python:
		return l.python(ctx, root)
	case stack.Maven:
		return l.maven(ctx, root)
	default:
		return skipped(l.Name(), "no linter configured for "+string(p.Kind)+" (add Checkstyle or PMD to the build)"), nil
	}
}

// golangci-lint exits 1 when it finds issues and 2 or more when it cannot
// run (bad config, toolchain mismatch). Only exit 1 is a lint failure; any
// other problem falls back to go vet so the gate still means something.
func (l *Lint) golang(ctx context.Context, root string) (quality.Result, error) {
	note := ""
	if process.Available("golangci-lint") {
		res, err := l.proc.Run(ctx, ports.Command{Name: "golangci-lint", Args: []string{"run"}, Dir: root})
		if err == nil {
			switch res.ExitCode {
			case 0:
				return quality.Result{Gate: l.Name(), Status: quality.Passed, Summary: "golangci-lint: no issues"}, nil
			case 1:
				return quality.Result{Gate: l.Name(), Status: quality.Failed, Summary: "golangci-lint found issues", Details: res.Combined()}, nil
			}
			note = " (golangci-lint could not run: exit " + fmtInt(res.ExitCode) + ")"
		} else if ctx.Err() != nil {
			return quality.Result{}, ctx.Err()
		}
	}
	res, err := l.proc.Run(ctx, ports.Command{Name: "go", Args: []string{"vet", "./..."}, Dir: root})
	if err != nil {
		return toolError(ctx, l.Name(), err)
	}
	if !res.Success() {
		return quality.Result{Gate: l.Name(), Status: quality.Failed, Summary: "go vet found issues" + note, Details: res.Combined()}, nil
	}
	return quality.Result{Gate: l.Name(), Status: quality.Passed, Summary: "go vet: no issues" + note}, nil
}

func (l *Lint) node(ctx context.Context, root string) (quality.Result, error) {
	if !hasScript(root, "lint") {
		return skipped(l.Name(), `package.json has no "lint" script`), nil
	}
	res, err := l.proc.Run(ctx, ports.Command{Name: "npm", Args: []string{"run", "lint", "--silent"}, Dir: root})
	if err != nil {
		return toolError(ctx, l.Name(), err)
	}
	if !res.Success() {
		return quality.Result{Gate: l.Name(), Status: quality.Failed, Summary: "npm run lint failed", Details: res.Combined()}, nil
	}
	return quality.Result{Gate: l.Name(), Status: quality.Passed, Summary: "npm run lint: no issues"}, nil
}

// ruff exits 1 with violations and 2 when it cannot run.
func (l *Lint) python(ctx context.Context, root string) (quality.Result, error) {
	ruff := "ruff"
	if bin, ok := process.VenvBin(root, "ruff"); ok {
		ruff = bin
	}
	res, err := l.proc.Run(ctx, ports.Command{Name: ruff, Args: []string{"check", "."}, Dir: root})
	if err != nil {
		return toolError(ctx, l.Name(), err)
	}
	switch res.ExitCode {
	case 0:
		return quality.Result{Gate: l.Name(), Status: quality.Passed, Summary: "ruff: no issues"}, nil
	case 1:
		return quality.Result{Gate: l.Name(), Status: quality.Failed, Summary: "ruff found issues", Details: res.Combined()}, nil
	default:
		return skipped(l.Name(), "ruff could not run: "+res.Combined()), nil
	}
}

// maven runs PMD when the build declares it (the SpecForge Java scaffold
// does): pmd:check fails the build on any violation of its rule set.
func (l *Lint) maven(ctx context.Context, root string) (quality.Result, error) {
	pom, err := os.ReadFile(filepath.Join(root, "pom.xml"))
	if err != nil || !strings.Contains(string(pom), "maven-pmd-plugin") {
		return skipped(l.Name(), "no linter configured for maven (add maven-pmd-plugin to the build)"), nil
	}
	mvn := "mvn"
	if _, err := os.Stat(filepath.Join(root, "mvnw")); err == nil {
		mvn = filepath.Join(root, "mvnw")
	}
	res, err := l.proc.Run(ctx, ports.Command{Name: mvn, Args: []string{"-B", "test-compile", "pmd:check"}, Dir: root})
	if err != nil {
		return toolError(ctx, l.Name(), err)
	}
	if res.Success() {
		return quality.Result{Gate: l.Name(), Status: quality.Passed, Summary: "PMD: no violations"}, nil
	}
	var found []string
	for _, line := range strings.Split(res.Combined(), "\n") {
		if i := strings.Index(line, "PMD Failure: "); i >= 0 {
			found = append(found, strings.TrimSpace(line[i+len("PMD Failure: "):]))
		}
	}
	if len(found) == 0 {
		// The build failed before PMD judged anything (compilation, plugin).
		return skipped(l.Name(), "PMD could not run: "+firstLine(res.Combined())), nil
	}
	return quality.Result{Gate: l.Name(), Status: quality.Failed,
		Summary: fmt.Sprintf("PMD found %d violation(s)", len(found)), Details: strings.Join(found, "\n")}, nil
}

func hasScript(root, name string) bool {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return false
	}
	_, ok := pkg.Scripts[name]
	return ok
}
